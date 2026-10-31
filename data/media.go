package data

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MediaStore 管理"下载/复制进来的图片"这一类需要落地到本地磁盘的资源。
//
// 为什么不直接存外链？VNDB / Bangumi 的图床地址会变、会限流，而且用户
// 断网时封面全白。所以导入时把海报抓下来存本地，数据库里记应用内路径。
type MediaStore struct {
	root string // <数据目录>/media
}

// NewMediaStore 准备 media 目录。
func NewMediaStore(root string) (*MediaStore, error) {
	for _, sub := range []string{"posters", "banners", "wallpapers"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o750); err != nil {
			return nil, fmt.Errorf("创建媒体目录失败: %w", err)
		}
	}
	return &MediaStore{root: root}, nil
}

// Root 返回 media 目录的绝对路径，给静态文件中间件用。
func (m *MediaStore) Root() string { return m.root }

// urlFor 把"分类 + 文件名"翻成前端能直接放进 <img src> 的路径。
func urlFor(kind, name string) string {
	return "/media/" + kind + "/" + name
}

// resolve 把 /media/... 形式的应用内路径还原成磁盘路径，并挡住路径穿越。
func (m *MediaStore) resolve(rel string) (string, bool) {
	rel = strings.TrimPrefix(strings.ReplaceAll(rel, "\\", "/"), "/media/")
	rel = strings.TrimPrefix(rel, "/")
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", false
	}
	return filepath.Join(m.root, clean), true
}

// SaveBytes 把内存里的图片写进 media，返回应用内路径。
func (m *MediaStore) SaveBytes(kind string, data []byte, ext string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("图片内容为空")
	}
	sum := sha1.Sum(data)
	name := hex.EncodeToString(sum[:]) + normalizeExt(ext)
	dst := filepath.Join(m.root, kind, name)
	if err := os.WriteFile(dst, data, 0o640); err != nil {
		return "", fmt.Errorf("写入图片失败: %w", err)
	}
	return urlFor(kind, name), nil
}

// SaveLocalFile 把用户自己选的一张本地图片复制进 media。
func (m *MediaStore) SaveLocalFile(kind, src string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("打开图片失败: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, 32<<20)) // 32MB 上限，海报不会更大
	if err != nil {
		return "", fmt.Errorf("读取图片失败: %w", err)
	}
	return m.SaveBytes(kind, data, filepath.Ext(src))
}

// SaveRemote 下载一张网络图片并存进 media。
//
// 传进来的是 VNDB 那种 https://t.vndb.org/cv/xx/xxxx.jpg 地址。
func (m *MediaStore) SaveRemote(kind, src string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, src, nil)
	if err != nil {
		return "", fmt.Errorf("图片地址无效: %w", err)
	}
	// 部分图床会拦空 UA。
	req.Header.Set("User-Agent", "gWvljBt/1.0 (+https://github.com/bishop9910/gWvljBt)")
	req.Header.Set("Accept", "image/*,*/*;q=0.8")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载图片失败: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", fmt.Errorf("读取图片失败: %w", err)
	}

	ext := filepath.Ext(strings.Split(src, "?")[0])
	if ext == "" || len(ext) > 6 {
		ext = extensionFromContentType(resp.Header.Get("Content-Type"))
	}
	return m.SaveBytes(kind, data, ext)
}

// Ingest 统一入口：http(s) 就下载，其它当本地文件复制，空值原样返回。
//
// 服务层拿到用户/元数据给的海报字段时不必关心它是链接还是路径。
func (m *MediaStore) Ingest(kind, src string) (string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", nil
	}
	// 已经是应用内路径，别再复制一遍。
	if strings.HasPrefix(src, "/media/") {
		return src, nil
	}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return m.SaveRemote(kind, src)
	}
	return m.SaveLocalFile(kind, src)
}

func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return ".jpg"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".avif", ".bmp":
		return ext
	default:
		return ".jpg"
	}
}

func extensionFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "avif"):
		return ".avif"
	default:
		return ".jpg"
	}
}
