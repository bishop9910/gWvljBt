// scanner.go
//
// 目录扫描：从一个大目录（比如 D:\Games）里找出"哪些子目录是一个 galgame 的安装目录"。
//
// 判定思路来自这类库管理器（mikavn-library / moeshelf / galroon 等）的通用做法：
// 不看目录名，看**文件指纹** —— 引擎的封包后缀（.xp3/.ypf/.cpz/.pfs/.npa/.int…）
// 和脚本后缀（.tjs/.ks/.nsc/.rpy…）几乎只出现在视觉小说里，比"有没有 exe"可靠得多。
// 每个目录算一个分数，过线就当成游戏，并且**不再往下钻**（游戏目录里不会套游戏）。
package data

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ScanCandidate 是扫描到的一个疑似游戏目录。
type ScanCandidate struct {
	Path       string   `json:"path"`
	Name       string   `json:"name"`       // 从目录名清洗出来的标题
	Exe        string   `json:"exe"`        // 猜出来的主程序，可能为空
	Engine     string   `json:"engine"`     // 猜出来的引擎，可能为空
	Evidence   []string `json:"evidence"`   // 判定依据，给用户看的
	Files      int      `json:"files"`      //
	Size       int64    `json:"size"`       // 字节
	Partial    bool     `json:"partial"`    // 统计是否因为文件太多被截断
	Confidence int      `json:"confidence"` // 0-99
	MatchedID  uint     `json:"matchedId"`  // 已经在库里的话，这里是游戏 ID

	// Meta 是从 VNDB / Bangumi 抓回来的基础信息，可能为空。
	// 前端可以在导入前就地改标题，剩下的字段原样带回后端。
	Meta *ScanMeta `json:"meta"`
}

// ScanMeta 是扫描结果附带的元数据快照（抓不到就整体为空）。
type ScanMeta struct {
	Source        string  `json:"source"` // vndb / bangumi
	SourceID      string  `json:"sourceId"`
	SourceURL     string  `json:"sourceUrl"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"originalTitle"`
	Poster        string  `json:"poster"`
	Banner        string  `json:"banner"`
	Released      string  `json:"released"`
	Developer     string  `json:"developer"`
	Publisher     string  `json:"publisher"`
	Rating        float64 `json:"rating"`
	Description   string  `json:"description"`
	Tags          string  `json:"tags"`
	Genres        string  `json:"genres"`
	Match         float64 `json:"match"` // 标题相似度 0-1，给界面显示用
}

// ScanResult 是一次扫描的结果。
type ScanResult struct {
	Root       string          `json:"root"`
	Dirs       int             `json:"dirs"`
	Candidates []ScanCandidate `json:"candidates"`
	Truncated  bool            `json:"truncated"`
	ElapsedMs  int64           `json:"elapsedMs"`
}

const (
	maxScanDirs       = 20000 // 最多看这么多个目录，防止扫到 C:\ 上卡死
	maxScanCandidates = 500   // 一次最多返回这么多结果
	maxSizeFiles      = 12000 // 单个游戏统计体积时最多数这么多文件
	acceptScore       = 56    // 判定分数下限
	markSearchDepth   = 2     // 引擎指纹 / 素材目录最多看到第二层子目录
	nestedExeDepth    = 2     // 兜底方案：主程序允许藏在两层子目录里
	nestedExePenalty  = 10    // 主程序不在根目录时扣分
)

// extMark 是"后缀 → 引擎"的指纹。
type extMark struct {
	Ext    string
	Engine string
	Weight int
}

// 顺序即优先级：同一个目录里命中多个引擎时，取权重最高的那个作为显示用的引擎。
var extMarks = []extMark{
	{".xp3", "KiriKiri", 44},
	{".tjs", "KiriKiri", 20},
	{".ks", "KiriKiri", 16},
	{".ypf", "YU-RIS", 44},
	{".ybn", "YU-RIS", 14},
	{".cpz", "CMVS", 44},
	{".pfs", "BGI", 42},
	{".npa", "Nitroplus", 42},
	{".int", "CatSystem2", 40},
	{".g00", "RealLive", 40},
	{".g01", "RealLive", 40},
	{".rpa", "Ren'Py", 44},
	{".rpy", "Ren'Py", 24},
	{".nsa", "NScripter", 40},
	{".ns2", "NScripter", 38},
	{".nsc", "NScripter", 24},
	{".arc", "NScripter", 22},
	{".pak", "Majiro", 38},
	{".dat", "", 0}, // .dat 太常见，不算证据
	{".bin", "", 0},
}

// 单独的文件名指纹。
var fileMarks = []struct {
	Name   string
	Engine string
	Weight int
}{
	{"data.win", "GameMaker", 40},
	{"unityplayer.dll", "Unity", 30},
	{"startup.tjs", "KiriKiri", 36},
	{"nscript.dat", "NScripter", 40},
}

// mediaDirNames 是"游戏里有、普通软件里少见"的结构证据。
var mediaDirNames = map[string]bool{
	"voice": true, "bgm": true, "se": true, "movie": true, "system": true,
	"scenario": true, "script": true, "image": true, "graph": true, "sound": true,
	"savedata": true, "save": true, "data": true, "xp3": true,
}

// contentDirNames 是"游戏内部的资源/脚本目录"，它们永远不能算作一个游戏本体。
//
// 这些名字是踩出来的坑：Key 系游戏的安装目录里一定有 file/ 和 g00/，
// 里面塞的全是 .g00/.ogg，如果只看后缀而不看有没有主程序，就会扫出
// "Summer Pockets file" 这种假条目，而真正的游戏根目录反而落选。
var contentDirNames = map[string]bool{
	"file": true, "g00": true, "g01": true, "g02": true, "g03": true,
	"data": true, "voice": true, "bgm": true, "se": true, "movie": true,
	"image": true, "graph": true, "sound": true, "system": true,
	"scenario": true, "script": true, "save": true, "savedata": true,
	"xp3": true, "pack": true, "bg": true, "music": true,
	"omake": true, "extra": true, "patch": true, "update": true,
	"voice2": true, "sound2": true, "movie2": true,
}

// 扫到大目录时直接跳过的东西。
var skipDirNames = map[string]bool{
	"node_modules": true, ".git": true, ".svn": true, ".hg": true, ".idea": true,
	".vscode": true, "__pycache__": true, "$recycle.bin": true,
	"system volume information": true, "windows": true, "windows.old": true,
	"program files": true, "program files (x86)": true, "programdata": true,
	"appdata": true, "recovery": true, "msocache": true, "perflogs": true,
	"temp": true, "tmp": true, "cache": true, ".cache": true, "lost+found": true,
}

// 卸载/安装/配置类程序不是游戏本体。
var exeSkipWords = []string{
	"unins", "setup", "install", "vcredist", "dxsetup", "dxwebsetup", "directx",
	"config", "setting", "crash", "report", "update", "patch", "launcher",
	"helper", "helper_", "activation", "register",
}

// ScanGames 扫描 root 下面最多 maxDepth 层的目录，返回疑似游戏目录。
//
// known 是"已经在库里的目录 → 游戏 ID"，用来在结果里标注"已入库"。
// 传 nil 就是全新扫描。
func ScanGames(root string, maxDepth int, known map[string]uint) (ScanResult, error) {
	started := time.Now()
	result := ScanResult{Root: root, Candidates: []ScanCandidate{}}

	abs, err := filepath.Abs(root)
	if err != nil {
		return result, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return result, err
	}
	if !info.IsDir() {
		return result, errNotDir(abs)
	}
	result.Root = abs

	if maxDepth <= 0 {
		maxDepth = 4
	}
	if maxDepth > 8 {
		maxDepth = 8
	}

	type node struct {
		dir   string
		depth int
	}

	queue := []node{{abs, 0}}
	for len(queue) > 0 {
		if result.Dirs >= maxScanDirs || len(result.Candidates) >= maxScanCandidates {
			result.Truncated = true
			break
		}
		current := queue[0]
		queue = queue[1:]

		entries, err := os.ReadDir(current.dir)
		if err != nil {
			continue // 权限不够之类的，跳过这个目录
		}
		result.Dirs++

		if candidate, ok := evaluateDir(current.dir, known); ok {
			// 命中就不再往下钻：游戏目录内部不会再有游戏。
			result.Candidates = append(result.Candidates, candidate)
			continue
		}

		if current.depth >= maxDepth {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if skipDirNames[name] || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
				continue
			}
			queue = append(queue, node{filepath.Join(current.dir, entry.Name()), current.depth + 1})
		}
	}

	sort.Slice(result.Candidates, func(i, j int) bool {
		return strings.ToLower(result.Candidates[i].Name) < strings.ToLower(result.Candidates[j].Name)
	})

	result.ElapsedMs = time.Since(started).Milliseconds()
	return result, nil
}

type scanErr string

func (e scanErr) Error() string { return string(e) }

func errNotDir(path string) error { return scanErr("不是目录: " + path) }

// shallowInfo 是"浅扫一遍"得到的信息：只看两层子目录，便宜。
type shallowInfo struct {
	exe         string // 根目录里的主程序（最大且不是安装器）
	exeSize     int64
	nestedExe   string // 藏在子目录里的主程序，兜底用
	nestedSize  int64
	engine      string // 命中的引擎
	engineMax   int    // 该引擎的最高权重（不是累加，累加会被几十个 .g00 撑爆）
	engineCount int    // 命中多少个特征文件
	engineNear  bool   // 根目录这一层就命中了
	mediaDirs   int
	files       int
	hasIni      bool
}

// scanShallow 用一次深度受限的遍历，把"判断是不是游戏"需要的证据收齐。
func scanShallow(dir string) shallowInfo {
	var info shallowInfo
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		depth := strings.Count(rel, string(os.PathSeparator)) + 1

		if entry.IsDir() {
			if mediaDirNames[strings.ToLower(entry.Name())] {
				info.mediaDirs++
			}
			if depth >= markSearchDepth {
				return fs.SkipDir
			}
			return nil
		}

		name := entry.Name()
		lower := strings.ToLower(name)
		info.files++

		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".exe" && !isInstallerExe(lower) {
			size := int64(0)
			if fi, infoErr := entry.Info(); infoErr == nil {
				size = fi.Size()
			}
			if depth == 1 {
				if size > info.exeSize || info.exe == "" {
					info.exe, info.exeSize = path, size
				}
			} else if depth <= nestedExeDepth && (size > info.nestedSize || info.nestedExe == "") {
				info.nestedExe, info.nestedSize = path, size
			}
			return nil
		}
		if depth == 1 && ext == ".ini" {
			info.hasIni = true
		}

		weight, engine := markOf(lower)
		if engine == "" || weight == 0 {
			return nil
		}
		info.engineCount++
		if weight > info.engineMax {
			info.engineMax = weight
			info.engine = engine
		}
		if depth == 1 {
			info.engineNear = true
		}
		return nil
	})
	return info
}

// markOf 把文件名映射成"引擎 + 权重"，命中不了就返回空。
func markOf(lowerName string) (int, string) {
	ext := strings.ToLower(filepath.Ext(lowerName))
	for _, mark := range extMarks {
		if mark.Ext == ext && mark.Engine != "" {
			return mark.Weight, mark.Engine
		}
	}
	for _, mark := range fileMarks {
		if mark.Name == lowerName && mark.Engine != "" {
			return mark.Weight, mark.Engine
		}
	}
	return 0, ""
}

// evaluateDir 判断一个目录是不是游戏安装目录。
//
// 硬性条件：**根目录里必须有主程序**（或主程序藏在子目录、但自己这层已经有引擎指纹）。
// 光有 .g00/.xp3 这类资源文件不算 —— Key 系游戏的 file/ 和 g00/ 就是反例。
func evaluateDir(dir string, known map[string]uint) (ScanCandidate, bool) {
	if contentDirNames[strings.ToLower(filepath.Base(dir))] {
		return ScanCandidate{}, false
	}

	info := scanShallow(dir)
	exe := info.exe
	penalty := 0
	if exe == "" {
		if info.nestedExe == "" || !info.engineNear {
			return ScanCandidate{}, false
		}
		exe = info.nestedExe
		penalty = nestedExePenalty
	}

	candidate := ScanCandidate{
		Path:     dir,
		Name:     CleanTitle(filepath.Base(dir)),
		Exe:      exe,
		Evidence: []string{"主程序 " + filepath.Base(exe)},
	}
	score := 34

	if info.engine != "" {
		score += info.engineMax
		candidate.Engine = info.engine
		candidate.Evidence = append(candidate.Evidence, info.engine+" 文件指纹")
		if info.engineCount > 1 {
			candidate.Evidence = append(candidate.Evidence, "命中 "+strconv.Itoa(info.engineCount)+" 个特征文件")
		}
	}
	if info.mediaDirs > 0 {
		score += minInt(info.mediaDirs, 3) * 5
		candidate.Evidence = append(candidate.Evidence, "素材目录 "+strconv.Itoa(info.mediaDirs)+" 个")
	}
	if info.files >= 40 {
		score += 8
	}
	if info.hasIni {
		score += 4
	}
	score -= penalty

	// 引擎指纹只是加分项：光有资源文件、没有主程序，不认为是一个游戏。
	if info.engine == "" && info.mediaDirs < 2 {
		return ScanCandidate{}, false
	}
	if score < acceptScore {
		return ScanCandidate{}, false
	}

	candidate.Confidence = minInt(97, score)
	candidate.Files, candidate.Size, candidate.Partial = measureDir(dir)
	if known != nil {
		candidate.MatchedID = known[NormalizePath(dir)]
	}
	return candidate, true
}

func isInstallerExe(lowerName string) bool {
	for _, word := range exeSkipWords {
		if strings.Contains(lowerName, word) {
			return true
		}
	}
	return false
}

// measureDir 统计目录里的文件数与总字节数，超过上限就停下并标记 partial。
func measureDir(dir string) (files int, size int64, partial bool) {
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		files++
		if files > maxSizeFiles {
			partial = true
			return filepath.SkipAll
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			size += info.Size()
		}
		return nil
	})
	return files, size, partial
}

// NormalizePath 把路径统一成"小写 + 干净分隔符"，用于判断两个路径是不是同一个目录。
func NormalizePath(path string) string {
	if path == "" {
		return ""
	}
	return strings.ToLower(filepath.Clean(path))
}

var (
	leadingBracketRe = regexp.MustCompile(`^\s*[\[【(（][^\]】)）]{0,60}[\]】)）]\s*`)
	trailingGroupRe  = regexp.MustCompile(`\s*[\[【(（][^\]】)）]{0,60}[\]】)）]\s*$`)
	versionSuffixRe  = regexp.MustCompile(`(?i)[\s_\-]*(v|ver|version)?\.?\d+(\.\d+){1,3}[a-z]?\s*$`)
	noiseWords       = []string{
		"汉化版", "汉化补丁", "汉化", "中文版", "中文", "完整版", "免安装版", "免安装",
		"破解版", "dl版", "下载版", "绿色版", "高压版", "合集", "全cg", "无修",
	}
)

// CleanTitle 从目录名里猜标题：去掉 [社团] 前缀、（汉化）后缀、版本号这些噪音。
func CleanTitle(raw string) string {
	title := strings.TrimSpace(raw)
	if title == "" {
		return title
	}
	// 反复剥掉开头的 [xxx] / 【xxx】 / (xxx)。
	for i := 0; i < 3; i++ {
		next := leadingBracketRe.ReplaceAllString(title, "")
		if next == title {
			break
		}
		title = next
	}
	// 反复剥掉结尾带噪音的括号组，比如 "游戏名 (汉化版)"。
	for i := 0; i < 3; i++ {
		trimmed := trailingGroupRe.ReplaceAllString(title, "")
		if trimmed == title || strings.TrimSpace(trimmed) == "" {
			break
		}
		title = trimmed
	}

	lower := strings.ToLower(title)
	for _, word := range noiseWords {
		if idx := strings.Index(lower, word); idx >= 0 {
			title = title[:idx] + title[idx+len(word):]
			lower = strings.ToLower(title)
		}
	}
	title = versionSuffixRe.ReplaceAllString(title, "")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.Join(strings.Fields(title), " ")
	title = strings.Trim(title, " -_·・[]【】()（）")
	if title == "" {
		return strings.TrimSpace(raw)
	}
	return title
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
