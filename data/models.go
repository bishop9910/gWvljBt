package data

import "time"

// ---------------------------------------------------------------------------
// 枚举
// ---------------------------------------------------------------------------

// 游玩状态。存库用这些稳定的英文标识，界面上的中文由前端映射，
// 这样以后想改文案不需要动数据库。
const (
	StatusUnplayed = "unplayed" // 未游玩
	StatusPlaying  = "playing"  // 在玩
	StatusCleared  = "cleared"  // 已通关
	StatusOnHold   = "onhold"   // 搁置
	StatusDropped  = "dropped"  // 弃坑
)

// AllStatuses 是合法状态值，服务层用它做校验。
var AllStatuses = []string{StatusUnplayed, StatusPlaying, StatusCleared, StatusOnHold, StatusDropped}

// 补丁类型。汉化和解锁（R18 解锁）是最常用的两个，单列出来让界面能
// 一眼看出"这游戏打没打汉化、开没开解锁"。
const (
	PatchTranslation = "translation" // 汉化补丁
	PatchUnlock      = "unlock"      // R18 解锁补丁
	PatchFix         = "fix"         // 官方/民间修正
	PatchMod         = "mod"         // MOD / 扩展
	PatchBypass      = "bypass"      // 免安装 / 破解 / 绕过
	PatchOther       = "other"       // 其它
)

// AllPatchKinds 是合法补丁类型。
var AllPatchKinds = []string{
	PatchTranslation, PatchUnlock, PatchFix, PatchMod, PatchBypass, PatchOther,
}

// ---------------------------------------------------------------------------
// 实体
// ---------------------------------------------------------------------------

// Game 是库里的一部 galgame / 视觉小说。
//
// 字段刻意做得比"标题 + 封面"多一些：开发者、发售年、标签、时长、状态
// 这些是筛选和统计的基础，缺了后面全库统计就无从做起。
type Game struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Title 是用户看到的主标题（通常是中文译名），
	// OriginalTitle 留原名（日文），Alias 放其它译名，逗号分隔。
	Title         string `gorm:"size:512;index" json:"title"`
	OriginalTitle string `gorm:"size:512" json:"originalTitle"`
	Alias         string `gorm:"type:text" json:"alias"`

	// Poster / Banner 存的是应用内可访问的路径（/media/xxx），
	// 也允许用户直接填外链 http(s)，读图时前端原样使用。
	Poster string `gorm:"size:1024" json:"poster"`
	Banner string `gorm:"size:1024" json:"banner"`

	Developer   string `gorm:"size:256;index" json:"developer"`
	Publisher   string `gorm:"size:256" json:"publisher"`
	ReleaseDate string `gorm:"size:32;index" json:"releaseDate"` // YYYY-MM-DD 或 YYYY
	Year        int    `gorm:"index" json:"year"`                // 从 ReleaseDate 抽出来，方便按年份过滤

	Description string `gorm:"type:text" json:"description"`
	Tags        string `gorm:"type:text" json:"tags"`   // 逗号分隔
	Genres      string `gorm:"type:text" json:"genres"` // 逗号分隔

	Rating     float64 `json:"rating"`     // 个人评分 0-10
	PlayStatus string  `gorm:"size:32;index" json:"playStatus"`
	Playtime   int     `json:"playtime"` // 累计游玩分钟数
	Favorite   bool    `gorm:"index" json:"favorite"`

	FolderPath string `gorm:"size:1024" json:"folderPath"`
	ExePath    string `gorm:"size:1024" json:"exePath"`
	SavePath   string `gorm:"size:1024" json:"savePath"`

	// 外部数据源关联。VNDBID 形如 "v17449"；BangumiID 是数字 subject id。
	VNDBID    string `gorm:"size:32;index" json:"vndbId"`
	BangumiID int    `gorm:"index" json:"bangumiId"`
	Source    string `gorm:"size:32" json:"source"` // vndb | bangumi | manual

	Notes        string     `gorm:"type:text" json:"notes"`
	LastPlayedAt *time.Time `json:"lastPlayedAt"`

	// Patches 不对应数据库列，由服务层按需填充，避免每次列表查询都 N+1。
	Patches []Patch `gorm:"-" json:"patches"`
}

// Patch 记录"这个游戏上装过的一个补丁"，以及它究竟改了哪些文件。
//
// 关键点：Files 里存的是相对于游戏目录的相对路径。删补丁时按这份清单
// 删文件，所以清单必须是安装后真实的文件集合，而不是猜出来的。
type Patch struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	GameID uint `gorm:"index;not null" json:"gameId"`

	Name      string     `gorm:"size:256" json:"name"`
	Kind      string     `gorm:"size:32;index" json:"kind"`
	Version   string     `gorm:"size:64" json:"version"`
	Source    string     `gorm:"size:256" json:"source"`    // 来源备注，例如 "2dfan 汉化组"
	SourceURL string     `gorm:"size:1024" json:"sourceUrl"` // 下载页链接，仅做记录
	Note      string     `gorm:"type:text" json:"note"`
	Installed bool       `json:"installed"`
	InstalledAt *time.Time `json:"installedAt"`

	// Files 不对应列，读取时单独查。
	Files []PatchFile `gorm:"-" json:"files"`
}

// PatchFile 是补丁实际覆盖到的单个文件。
type PatchFile struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	PatchID uint   `gorm:"index;not null" json:"patchId"`
	RelPath string `gorm:"size:1024" json:"relPath"` // 相对游戏目录，统一 / 分隔
	Size    int64  `json:"size"`
	Op      string `gorm:"size:16" json:"op"` // added | modified
	IsDir   bool   `json:"isDir"`
}

// Snapshot 是游戏目录某一时刻的文件清单，用来做"装完补丁后哪些文件变了"的差异。
//
// 只存 size + mtime：对判断补丁是否覆盖某个文件足够，而且比对整份 sha256
// 快几个数量级（galgame 目录动辄上万个文件）。
type Snapshot struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	GameID uint   `gorm:"uniqueIndex:idx_snap_path;not null" json:"gameId"`
	RelPath string `gorm:"uniqueIndex:idx_snap_path;size:1024" json:"relPath"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"` // UnixNano
	IsDir   bool   `json:"isDir"`
}
