package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// ConfigData 是 config.json 的全部内容。
//
// 约定：只有"表现层开关"放这里。游戏、补丁这些业务数据一律进 SQLite ——
// 配置文件是给人手改的，塞进几千条记录会变得没法维护。
type ConfigData struct {
	// Preset 是整套外观预设：sumi | yoru | bengara | washi | sakura | matcha。
	// 每套预设自带底色、纸面、线色与主色，明暗也由它决定（没有单独的明暗开关）。
	Preset string `json:"preset"`
	// Accent 覆盖预设主色（#RRGGBB），留空表示跟随预设。
	Accent string `json:"accent"`
	// Wallpaper 是壁纸图片的应用内路径（/media/wallpapers/xxx.jpg），留空表示不用壁纸。
	Wallpaper string `json:"wallpaper"`
	// WallpaperBlur 壁纸模糊半径（px），0-40。
	WallpaperBlur int `json:"wallpaperBlur"`
	// WallpaperVeil 压在壁纸上的底色浓度（%），0-95。数值越大字越清楚。
	WallpaperVeil int `json:"wallpaperVeil"`
	// Radius 卡片/面板圆角（px），0-24。
	Radius int `json:"radius"`
	// Motion 控制过渡动效，低配机器可以关掉。
	Motion bool `json:"motion"`
	// Density 是列表密度：cozy | compact。
	Density string `json:"density"`
	// PosterAspect 是海报比例：2/3 | 3/4 | 1/1。
	PosterAspect string `json:"posterAspect"`
	// SortMode 是库页默认排序：updated | title | year | rating | playtime。
	SortMode string `json:"sortMode"`
	// DefaultSource 是元数据检索默认源：vndb | bangumi。
	DefaultSource string `json:"defaultSource"`
	// VNDBToken / BangumiToken 可选，填了会带上认证头。
	VNDBToken    string `json:"vndbToken"`
	BangumiToken string `json:"bangumiToken"`
	// TwoDFanURL 是"去下补丁"的跳转地址，用户可以改成自己的镜像站。
	TwoDFanURL string `json:"twoDFanUrl"`
	// TouchgalURL 是 TouchGal 站点根地址，换镜像站时改这里。
	TouchgalURL string `json:"touchgalUrl"`
	// TouchgalCookie 是整段 Cookie（至少要有 cf_clearance），用来过 Cloudflare。
	TouchgalCookie string `json:"touchgalCookie"`
	// TouchgalToken 是登录令牌（kun-galgame-patch-moe-token），不登录留空即可。
	TouchgalToken string `json:"touchgalToken"`
}

// Presets 是出厂提供的整套外观。前端 src/lib/theme.ts 里有一份同构的定义，
// 两边改动要同步 —— 这里只做合法性校验，配色本身在前端。
var presets = []string{"sumi", "yoru", "bengara", "washi", "sakura", "matcha"}

// DefaultConfig 返回出厂设置。新增字段时记得同步这里，否则老用户升级后
// 拿到的是零值。
func DefaultConfig() ConfigData {
	return ConfigData{
		Preset:         "sumi",
		Accent:         "",
		Wallpaper:      "",
		WallpaperBlur:  8,
		WallpaperVeil:  72,
		Radius:         6,
		Motion:         true,
		Density:        "cozy",
		PosterAspect:   "2/3",
		SortMode:       "updated",
		DefaultSource:  "vndb",
		VNDBToken:      "",
		BangumiToken:   "",
		TwoDFanURL:     "https://www.2dfan.com/",
		TouchgalURL:    "https://www.touchgal.ink/",
		TouchgalCookie: "",
		TouchgalToken:  "",
	}
}

// Config 持有配置的内存副本与落盘路径。
type Config struct {
	data ConfigData
	dir  string
	path string
}

// Init 加载配置；文件不存在时写入默认配置。
//
// 关键细节：先塞默认值再 Unmarshal，这样老版本 config.json 里缺失的字段
// 会自动拿到默认值，而不是变成零值。
func (c *Config) Init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// GWVLJBT_HOME 可以把整个数据目录挪走（便携使用、自动化测试都方便）。
	dir := filepath.Join(home, "gWvljBt")
	if override := os.Getenv("GWVLJBT_HOME"); override != "" {
		dir = override
	}
	path := filepath.Join(dir, "config.json")

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	c.dir = dir
	c.path = path

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.data = DefaultConfig()
			return c.Save()
		}
		return err
	}

	c.data = DefaultConfig()
	if err := json.Unmarshal(b, &c.data); err != nil {
		return fmt.Errorf("解析配置文件失败 %s: %w", path, err)
	}
	// 校验一遍，避免用户手改出非法值把界面搞坏。
	c.data = sanitize(c.data)
	return nil
}

// Dir 返回应用数据目录（<用户目录>/gWvljBt），数据库和媒体都存在这里。
func (c *Config) Dir() string {
	if c.dir != "" {
		return c.dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "gWvljBt")
}

// Path 返回 config.json 的完整路径。
func (c *Config) Path() string { return c.path }

// Save 序列化后写盘（先序列化成功再写，失败不会破坏原文件）。
func (c *Config) Save() error {
	b, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, b, 0o600)
}

// Get 返回当前配置的副本。
func (c *Config) Get() ConfigData { return c.data }

// Set 整体替换配置（用于导入/恢复默认），校验后落盘。
func (c *Config) Set(next ConfigData) error {
	backup := c.data
	c.data = sanitize(next)
	if err := c.Save(); err != nil {
		c.data = backup
		return err
	}
	return nil
}

// Update 用 Go 侧改配置，改完自动落盘；失败回滚内存。
func (c *Config) Update(fn func(*ConfigData)) error {
	backup := c.data
	fn(&c.data)
	c.data = sanitize(c.data)
	if err := c.Save(); err != nil {
		c.data = backup
		return err
	}
	return nil
}

// Change 按 json tag 匹配字段改值并落盘，供前端按 key 调用。
// 前端传来的 JSON 数字是 float64，转整数时只接受整值。
func (c *Config) Change(key string, newVal any) error {
	v := reflect.ValueOf(&c.data).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name != key {
			continue
		}

		fv := v.Field(i)
		nv := reflect.ValueOf(newVal)
		if !nv.IsValid() {
			return fmt.Errorf("%s 不能为 nil", key)
		}

		if nv.Kind() == reflect.Float64 && fv.Kind() != reflect.Float64 {
			if nv.Float() != float64(int64(nv.Float())) {
				return fmt.Errorf("%s 需要整数，收到 %v", key, newVal)
			}
			nv = reflect.ValueOf(int64(nv.Float()))
		}

		if !nv.Type().ConvertibleTo(fv.Type()) {
			return fmt.Errorf("%s 需要 %s 类型，收到 %T", key, fv.Type(), newVal)
		}
		before := fv.Interface()

		// 先写进内存、整体 sanitize + 落盘；失败则退回原值。
		fv.Set(nv.Convert(fv.Type()))
		c.data = sanitize(c.data)
		if err := c.Save(); err != nil {
			fv.Set(reflect.ValueOf(before))
			return err
		}
		return nil
	}
	return fmt.Errorf("未知配置项: %s", key)
}

// sanitize 把越界/非法的值夹回合法范围，而不是报错拒绝。
//
// 前端滑块和手改配置都可能给出奇怪的值，静默纠正比弹错误体验好。
func sanitize(d ConfigData) ConfigData {
	valid := false
	for _, p := range presets {
		if d.Preset == p {
			valid = true
			break
		}
	}
	if !valid {
		d.Preset = "sumi"
	}

	d.Accent = normalizeHex(d.Accent)

	// 壁纸只接受我们自己的媒体路径：config.json 是纯文本，
	// 手写一个 file:///C:/... 进来也没用，前端 <img> 拿不到。
	if !strings.HasPrefix(d.Wallpaper, "/media/wallpapers/") {
		d.Wallpaper = ""
	}

	d.WallpaperBlur = clampInt(d.WallpaperBlur, 0, 40)
	d.WallpaperVeil = clampInt(d.WallpaperVeil, 0, 95)
	d.Radius = clampInt(d.Radius, 0, 24)

	switch d.Density {
	case "cozy", "compact":
	default:
		d.Density = "cozy"
	}
	switch d.PosterAspect {
	case "2/3", "3/4", "1/1", "4/5":
	default:
		d.PosterAspect = "2/3"
	}
	switch d.SortMode {
	case "updated", "created", "title", "year", "rating", "playtime":
	default:
		d.SortMode = "updated"
	}
	switch d.DefaultSource {
	case "vndb", "bangumi":
	default:
		d.DefaultSource = "vndb"
	}
	if strings.TrimSpace(d.TwoDFanURL) == "" {
		d.TwoDFanURL = "https://www.2dfan.com/"
	}
	d.TouchgalURL = normalizeBaseURL(d.TouchgalURL, "https://www.touchgal.ink/")
	d.TouchgalCookie = strings.TrimSpace(d.TouchgalCookie)
	d.TouchgalToken = strings.TrimSpace(d.TouchgalToken)
	return d
}

// normalizeBaseURL 只补协议和结尾的斜杠，不做 URL 校验 —— 用户可能填的是
// 自建镜像站，多拦一道只会让人摸不着头脑。
func normalizeBaseURL(value, fallback string) string {
	s := strings.TrimSpace(value)
	if s == "" {
		return fallback
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	if !strings.HasSuffix(s, "/") {
		s += "/"
	}
	return s
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// normalizeHex 只接受 #RRGGBB / #RGB，其它一律当"没填"。
func normalizeHex(value string) string {
	s := strings.TrimSpace(value)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	if len(s) != 4 && len(s) != 7 {
		return ""
	}
	for _, r := range s[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return ""
		}
	}
	return strings.ToLower(s)
}
