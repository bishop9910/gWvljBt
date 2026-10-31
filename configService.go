// configService.go
package main

import (
	"gWvljBt/config"
	"gWvljBt/data"
	"os"
	"os/exec"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ConfigService 把 config.Config 暴露给前端。
// 只有表现层开关走这里，业务数据在 GameService / PatchService 里。
type ConfigService struct {
	cfg   *config.Config
	media *data.MediaStore
}

func NewConfigService(cfg *config.Config, media *data.MediaStore) *ConfigService {
	return &ConfigService{cfg: cfg, media: media}
}

// Get 读取完整配置。
func (s *ConfigService) Get() config.ConfigData {
	return s.cfg.Get()
}

// Change 按 key 改单个配置项，改完广播 config:changed。
func (s *ConfigService) Change(key string, value any) error {
	if err := s.cfg.Change(key, value); err != nil {
		return err
	}
	s.emitChanged()
	return nil
}

// SetAll 一次性写入多个配置项 —— 主题面板里"应用整套外观"用它，
// 避免逐个 Change 触发多次落盘和多次事件。
func (s *ConfigService) SetAll(next config.ConfigData) error {
	if err := s.cfg.Set(next); err != nil {
		return err
	}
	s.emitChanged()
	return nil
}

// Reset 恢复出厂设置。
func (s *ConfigService) Reset() error {
	if err := s.cfg.Set(config.DefaultConfig()); err != nil {
		return err
	}
	s.emitChanged()
	return nil
}

// DataDir 返回数据目录（数据库 + 媒体），设置页用来展示和"打开目录"。
func (s *ConfigService) DataDir() string {
	return s.cfg.Dir()
}

// OpenDataDir 在资源管理器里打开数据目录。
func (s *ConfigService) OpenDataDir() error {
	return openInExplorer(s.cfg.Dir())
}

// OpenConfigFile 打开 config.json 本体，方便高级用户手改。
func (s *ConfigService) OpenConfigFile() error {
	path := s.cfg.Path()
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return openInExplorer(path)
}

// PickWallpaper 让用户选一张本地图片当壁纸：复制进 media/wallpapers 后写进配置。
//
// 用户取消时返回空字符串（前端不需要当成错误处理）。
func (s *ConfigService) PickWallpaper() (string, error) {
	app := application.Get()
	if app == nil {
		return "", nil
	}
	path, err := app.Dialog.OpenFile().
		SetTitle("选择壁纸").
		AddFilter("图片", "*.jpg;*.jpeg;*.png;*.webp;*.avif;*.gif;*.bmp").
		PromptForSingleSelection()
	if err != nil || strings.TrimSpace(path) == "" {
		return "", nil
	}

	url, err := s.media.Ingest("wallpapers", path)
	if err != nil {
		return "", err
	}
	if err := s.cfg.Update(func(d *config.ConfigData) { d.Wallpaper = url }); err != nil {
		return "", err
	}
	s.emitChanged()
	return url, nil
}

// ClearWallpaper 停用壁纸。图片文件留在 media/wallpapers 里不删 ——
// 同一个文件可能只被这一处引用，但删文件对用户没有好处，磁盘上几十 KB 而已。
func (s *ConfigService) ClearWallpaper() error {
	if err := s.cfg.Update(func(d *config.ConfigData) { d.Wallpaper = "" }); err != nil {
		return err
	}
	s.emitChanged()
	return nil
}

// emitChanged 用全局 application.Get() 拿 app，避免构造期循环依赖。
func (s *ConfigService) emitChanged() {
	if app := application.Get(); app != nil {
		app.Event.Emit("config:changed", s.cfg.Get())
	}
}

// openInExplorer 在 Windows 资源管理器里定位到路径。
//
// 传目录时用 explorer 直接打开；传文件时用 /select 高亮选中。
// 不依赖 Wails 的 Browser，因为这里要的是"打开文件管理器"而不是浏览器。
func openInExplorer(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return exec.Command("explorer", path).Start()
	}
	return exec.Command("explorer", "/select,", path).Start()
}
