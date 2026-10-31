package main

import (
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gWvljBt/config"
	"gWvljBt/data"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails 用 Go 的 embed 把前端产物打进二进制。
//
//go:embed all:frontend/dist
var assets embed.FS

// mediaPort 是"开发模式图床"监听的端口。
//
// 为什么需要它：`wails3 dev` 时窗口直接加载 Vite dev server，产物目录里的
// asset 中间件根本不参与，`/media/xxx.jpg` 这些请求会打到 Vite 上 404。
// 所以开发模式额外起一个小的静态文件服务，由 vite.config.ts 里的 proxy 转发。
// 打包后的正式版走 application AssetOptions.Middleware，不依赖这个端口。
const mediaPort = 39217

func init() {
	// 注册事件类型，让绑定生成器能产出强类型的 TS API。
	application.RegisterEvent[config.ConfigData]("config:changed")
	application.RegisterEvent[int64]("game:changed")
	application.RegisterEvent[int64]("patch:changed")
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 配置必须先初始化：数据目录（数据库、海报都放里面）由它决定。
	cfg := &config.Config{}
	if err := cfg.Init(); err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	store, err := data.Open(filepath.Join(cfg.Dir(), "app.db"))
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("关闭数据库失败: %v", err)
		}
	}()

	media, err := data.NewMediaStore(filepath.Join(cfg.Dir(), "media"))
	if err != nil {
		log.Fatalf("%v", err)
	}

	startDevMediaServer(media.Root())

	// 扫描器要复用游戏服务的建库逻辑（校验、事件、时间戳），也要元数据服务来
	// 给扫出来的目录补标题/封面，所以两个都先单独建实例。
	gameService := NewGameService(store, media)
	metaService := NewMetaService(cfg)

	app := application.New(application.Options{
		Name:        "gWvljBt",
		Description: "Galgame / 视觉小说库管理器 --by bishop9910",
		Services: []application.Service{
			application.NewService(NewConfigService(cfg, media)),
			application.NewService(gameService),
			application.NewService(NewPatchService(store)),
			application.NewService(metaService),
			application.NewService(NewScanService(gameService, metaService)),
			application.NewService(NewTouchGalService(cfg)),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: mediaMiddleware(media.Root()),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			// WebView2 的缓存 / 用户数据也放进数据目录：
			// 换机器时把数据目录整个拷走就能用，也不会和别的 Wails 应用抢同一个 user data 目录。
			WebviewUserDataPath: filepath.Join(cfg.Dir(), "webview"),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "gWvljBt",
		Width:     1320,
		Height:    840,
		MinWidth:  980,
		MinHeight: 640,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		// 和前端默认底色（墨 #14161a）对齐，避免启动瞬间白闪。
		BackgroundColour: application.NewRGB(20, 22, 26),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatalf("%v", err)
	}
}

// mediaMiddleware 拦截 /media/ 前缀，从磁盘目录读海报；其余请求交给内嵌资源。
func mediaMiddleware(root string) func(http.Handler) http.Handler {
	serve := mediaHandler(root)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/media/") {
				serve.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// startDevMediaServer 仅在开发模式（wails3 dev）下启动独立图床端口。
func startDevMediaServer(root string) {
	if os.Getenv("FRONTEND_DEVSERVER_URL") == "" {
		return
	}
	addr := fmt.Sprintf("127.0.0.1:%d", mediaPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("开发模式图床端口 %s 不可用（海报可能不显示）: %v", addr, err)
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/media/", mediaHandler(root))

	go func() {
		log.Printf("开发模式图床已启动: http://%s/media/", addr)
		if err := http.Serve(ln, mux); err != nil {
			log.Printf("开发模式图床退出: %v", err)
		}
	}()
}

// mediaHandler 安全地从 root 下取文件。
//
// 路径必须过 SafeRel：这是个直接暴露给 webview 的读文件接口，
// 一个 "../" 就能把整个磁盘读出去。
func mediaHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/media/")
		clean, err := data.SafeRel(rel)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		full := filepath.Join(root, filepath.FromSlash(clean))
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		// 海报文件名是内容哈希，永不变化，可以放心长缓存。
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeFile(w, r, full)
	})
}
