// scanService.go
//
// 游戏扫描器：选一个大目录 → 递归找 galgame 安装目录 → 勾选后批量入库。
// 真正的目录判定在 data/scanner.go，这里只负责对话框、去重和落库。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gWvljBt/data"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ScanImportInput 是"导入扫描结果"时前端提交的一项。
//
// 前端把扫描时已经算好的标题/主程序一起带回来，避免再跑一次目录识别；
// 抓到的元数据也原样带回：用户在列表里改过的标题优先，其余字段为空就回退。
type ScanImportInput struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Exe    string `json:"exe"`
	Engine string `json:"engine"`

	Source        string  `json:"source"`
	SourceID      string  `json:"sourceId"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"originalTitle"`
	Poster        string  `json:"poster"`
	Banner        string  `json:"banner"`
	Developer     string  `json:"developer"`
	Publisher     string  `json:"publisher"`
	ReleaseDate   string  `json:"releaseDate"`
	Description   string  `json:"description"`
	Tags          string  `json:"tags"`
	Genres        string  `json:"genres"`
	Rating        float64 `json:"rating"`
}

// ScanImportResult 是批量导入的结果。
type ScanImportResult struct {
	Created int      `json:"created"`
	Skipped []string `json:"skipped"` // 已经在库里，跳过
	Failed  []string `json:"failed"`  // 目录不存在 / 建库失败
}

// ScanService 提供扫描、元数据匹配与批量导入。
type ScanService struct {
	games *GameService
	meta  *MetaService
}

func NewScanService(games *GameService, meta *MetaService) *ScanService {
	return &ScanService{games: games, meta: meta}
}

// PickRoot 选择要扫描的根目录，取消时返回空字符串。
func (s *ScanService) PickRoot() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("应用未就绪")
	}
	path, err := app.Dialog.OpenFile().
		SetTitle("选择要扫描的目录").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(false).
		PromptForSingleSelection()
	if err != nil {
		return "", nil // Wails 取消选择也报错，当成取消处理
	}
	return path, nil
}

// Scan 扫描目录。maxDepth <= 0 时用默认的 4 层。
func (s *ScanService) Scan(root string, maxDepth int) (data.ScanResult, error) {
	if strings.TrimSpace(root) == "" {
		return data.ScanResult{}, fmt.Errorf("先选一个要扫描的目录")
	}
	known, err := s.knownFolders()
	if err != nil {
		return data.ScanResult{}, err
	}
	return data.ScanGames(strings.TrimSpace(root), maxDepth, known)
}

// Import 把选中的扫描结果批量建库。
func (s *ScanService) Import(items []ScanImportInput) (ScanImportResult, error) {
	result := ScanImportResult{Skipped: []string{}, Failed: []string{}}
	if len(items) == 0 {
		return result, fmt.Errorf("没有选中任何目录")
	}

	known, err := s.knownFolders()
	if err != nil {
		return result, err
	}

	for _, item := range items {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			result.Failed = append(result.Failed, path+"（路径无效）")
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			result.Failed = append(result.Failed, filepath.Base(abs)+"（目录不存在）")
			continue
		}

		key := data.NormalizePath(abs)
		if _, exists := known[key]; exists {
			result.Skipped = append(result.Skipped, filepath.Base(abs))
			continue
		}

		title := firstNonEmpty(item.Title, item.Name, data.CleanTitle(filepath.Base(abs)))
		if title == "" {
			title = filepath.Base(abs)
		}

		exe := strings.TrimSpace(item.Exe)
		if exe != "" {
			if info, statErr := os.Stat(exe); statErr != nil || info.IsDir() {
				exe = ""
			}
		}
		if exe == "" {
			// 扫描时没猜到主程序，或者被删了，这里再试一次。
			if found, findErr := findExecutable(abs); findErr == nil {
				exe = found
			}
		}

		source := strings.ToLower(strings.TrimSpace(item.Source))
		if source == "" {
			source = "scan"
		}
		input := GameInput{
			Title:         title,
			OriginalTitle: item.OriginalTitle,
			Poster:        item.Poster,
			Banner:        item.Banner,
			Developer:     item.Developer,
			Publisher:     item.Publisher,
			ReleaseDate:   item.ReleaseDate,
			Description:   item.Description,
			Tags:          item.Tags,
			Genres:        item.Genres,
			Rating:        item.Rating,
			FolderPath:    abs,
			ExePath:       exe,
			Source:        source,
			PlayStatus:    data.StatusUnplayed,
		}
		switch source {
		case "vndb":
			input.VNDBID = item.SourceID
		case "bangumi":
			if id, convErr := strconv.Atoi(strings.TrimSpace(item.SourceID)); convErr == nil {
				input.BangumiID = id
			}
		}

		created, err := s.games.Create(input)
		if err != nil {
			result.Failed = append(result.Failed, fmt.Sprintf("%s：%v", title, err))
			continue
		}
		known[key] = created.ID
		result.Created++
	}

	return result, nil
}

// knownFolders 取出"已经在库里的目录 → 游戏 ID"，用来判断重复。
func (s *ScanService) knownFolders() (map[string]uint, error) {
	var games []data.Game
	if err := s.games.store.DB.
		Select("id", "folder_path").
		Where("folder_path IS NOT NULL AND folder_path <> ''").
		Find(&games).Error; err != nil {
		return nil, fmt.Errorf("读取游戏库失败: %w", err)
	}
	known := make(map[string]uint, len(games))
	for _, game := range games {
		known[data.NormalizePath(game.FolderPath)] = game.ID
	}
	return known, nil
}

// minMatchScore 是"标题算匹配上"的相似度下限。
//
// 低于这个值就当作没抓到：宁可让用户手动填，也不要塞一个错的作品进去。
const minMatchScore = 0.6

// Lookup 按标题去数据源找一部作品的基础信息。
//
// 抓不到（没搜到 / 相似度不够 / 详情请求失败）时返回空结构而不报错，
// 前端据此把这一行标成"未匹配"，标题仍用目录名清洗出来的那个。
func (s *ScanService) Lookup(name string, source string) (data.ScanMeta, error) {
	query := strings.TrimSpace(name)
	if query == "" {
		return data.ScanMeta{}, fmt.Errorf("标题为空，没法搜索")
	}
	if s.meta == nil {
		return data.ScanMeta{}, fmt.Errorf("元数据服务未就绪")
	}

	src := strings.ToLower(strings.TrimSpace(source))
	if src != "bangumi" {
		src = "vndb"
	}

	hits, err := s.meta.Search(src, query)
	if err != nil {
		return data.ScanMeta{}, err
	}
	if len(hits) == 0 {
		return data.ScanMeta{}, nil
	}

	best := -1
	bestScore := 0.0
	for i, hit := range hits {
		score := hitScore(query, hit)
		if score > bestScore {
			bestScore = score
			best = i
		}
	}
	if best < 0 || bestScore < minMatchScore {
		return data.ScanMeta{}, nil
	}

	hit := hits[best]
	meta := data.ScanMeta{
		Source:        src,
		SourceID:      hit.ID,
		Title:         hit.Title,
		OriginalTitle: hit.OriginalTitle,
		Poster:        hit.Poster,
		Released:      hit.Released,
		Developer:     hit.Developer,
		Description:   hit.Description,
		Match:         round2(bestScore),
	}

	// 搜索结果只有概要，再要一次详情把 banner / 标签 / 分补上。
	if draft, detailErr := s.meta.Detail(src, hit.ID); detailErr == nil {
		meta.Title = firstNonEmpty(draft.Title, meta.Title)
		meta.OriginalTitle = firstNonEmpty(draft.OriginalTitle, meta.OriginalTitle)
		meta.Poster = firstNonEmpty(draft.Poster, meta.Poster)
		meta.Banner = draft.Banner
		meta.Released = firstNonEmpty(draft.ReleaseDate, meta.Released)
		meta.Developer = firstNonEmpty(draft.Developer, meta.Developer)
		meta.Publisher = draft.Publisher
		meta.Rating = draft.Rating
		meta.Description = firstNonEmpty(draft.Description, meta.Description)
		meta.Tags = draft.Tags
		meta.Genres = draft.Genres
		meta.SourceURL = draft.SourceURL
	}
	if meta.Tags == "" && len(hit.Tags) > 0 {
		meta.Tags = strings.Join(hit.Tags, ", ")
	}
	return meta, nil
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

// hitScore 取搜索结果里几个候选标题的最高相似度。
func hitScore(query string, hit SearchHit) float64 {
	best := 0.0
	for _, candidate := range []string{hit.Title, hit.TitleCN, hit.OriginalTitle} {
		if score := titleSimilarity(query, candidate); score > best {
			best = score
		}
	}
	return best
}

// titleSimilarity 比较两个标题：完全相等 1.0，互相包含按长度差打折，其余走编辑距离。
func titleSimilarity(a string, b string) float64 {
	na, nb := normalizeTitle(a), normalizeTitle(b)
	if na == "" || nb == "" {
		return 0
	}
	if na == nb {
		return 1
	}
	ra, rb := []rune(na), []rune(nb)
	short, long := len(ra), len(rb)
	if short > long {
		short, long = long, short
	}
	if strings.Contains(na, nb) || strings.Contains(nb, na) {
		// 光是"包含"还不够：clannad 也会包含在 clannad after story 里。
		return 0.72 + 0.2*float64(short)/float64(long)
	}
	distance := levenshtein(ra, rb)
	ratio := 1 - float64(distance)/float64(len(ra))
	if ratio < 0 {
		return 0
	}
	return ratio
}

// normalizeTitle 只留下字母、数字和 CJK，去掉括号、空格、标点这些噪音。
func normalizeTitle(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 0x3040: // 假名 / 汉字 / 谚文
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func levenshtein(a []rune, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			current[j] = minInt(previous[j]+1, minInt(current[j-1]+1, previous[j-1]+cost))
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}
