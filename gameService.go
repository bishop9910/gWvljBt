// gameService.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gWvljBt/data"

	"github.com/wailsapp/wails/v3/pkg/application"
	"gorm.io/gorm"
)

// GameService 负责游戏本体的增删改查、启动、目录操作与全库统计。
type GameService struct {
	store *data.Store
	media *data.MediaStore
}

func NewGameService(store *data.Store, media *data.MediaStore) *GameService {
	return &GameService{store: store, media: media}
}

// ---------------------------------------------------------------------------
// 入参 / 出参
// ---------------------------------------------------------------------------

// GameInput 是新增/编辑游戏时前端提交的表单。
//
// 单独定义而不是直接用 data.Game，是为了让 ID、CreatedAt 这些字段
// 不可能被前端覆盖。
type GameInput struct {
	Title         string  `json:"title"`
	OriginalTitle string  `json:"originalTitle"`
	Alias         string  `json:"alias"`
	Poster        string  `json:"poster"`
	Banner        string  `json:"banner"`
	Developer     string  `json:"developer"`
	Publisher     string  `json:"publisher"`
	ReleaseDate   string  `json:"releaseDate"`
	Description   string  `json:"description"`
	Tags          string  `json:"tags"`
	Genres        string  `json:"genres"`
	Rating        float64 `json:"rating"`
	PlayStatus    string  `json:"playStatus"`
	Playtime      int     `json:"playtime"`
	Favorite      bool    `json:"favorite"`
	FolderPath    string  `json:"folderPath"`
	ExePath       string  `json:"exePath"`
	SavePath      string  `json:"savePath"`
	VNDBID        string  `json:"vndbId"`
	BangumiID     int     `json:"bangumiId"`
	Source        string  `json:"source"`
	Notes         string  `json:"notes"`
}

// GameFilter 是库页的筛选条件。
type GameFilter struct {
	Query     string `json:"query"`
	Status    string `json:"status"`
	Developer string `json:"developer"`
	Tag       string `json:"tag"`
	Sort      string `json:"sort"`
	Favorite  bool   `json:"favorite"`
}

// CountItem 是"某维度 + 数量"，统计页和筛选下拉都用它。
type CountItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TimelinePoint 是统计页按年度的柱状图数据。
type TimelinePoint struct {
	Year    int `json:"year"`
	Total   int `json:"total"`
	Cleared int `json:"cleared"`
}

// Stats 是全库统计。
type Stats struct {
	Total         int             `json:"total"`
	Favorite      int             `json:"favorite"`
	Rated         int             `json:"rated"`
	AvgRating     float64         `json:"avgRating"`
	TotalPlaytime int             `json:"totalPlaytime"` // 分钟
	ClearedCount  int             `json:"clearedCount"`
	ByStatus      []CountItem     `json:"byStatus"`
	ByYear        []CountItem     `json:"byYear"`
	ByDeveloper   []CountItem     `json:"byDeveloper"`
	ByPatchKind   []CountItem     `json:"byPatchKind"`
	TopTags       []CountItem     `json:"topTags"`
	Timeline      []TimelinePoint `json:"timeline"`
	MissingFolder int             `json:"missingFolder"`
	NoPoster      int             `json:"noPoster"`
	Recent        []data.Game     `json:"recent"`
}

// ---------------------------------------------------------------------------
// 查询
// ---------------------------------------------------------------------------

// List 按筛选条件返回游戏列表，并顺带把每条游戏的补丁记录填进去。
//
// 补丁一次性查出来再按 GameID 归组，避免 N+1。补丁的文件清单不在这里加载，
// 卡片上只需要"有几个补丁、有没有汉化"。
func (s *GameService) List(f GameFilter) ([]data.Game, error) {
	db := s.store.DB.Model(&data.Game{})

	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + q + "%"
		db = db.Where(
			"title LIKE ? OR original_title LIKE ? OR alias LIKE ? OR developer LIKE ? OR tags LIKE ? OR notes LIKE ?",
			like, like, like, like, like, like,
		)
	}
	if f.Status != "" && f.Status != "all" {
		db = db.Where("play_status = ?", f.Status)
	}
	if f.Developer != "" {
		db = db.Where("developer = ?", f.Developer)
	}
	if f.Tag != "" {
		db = db.Where("tags LIKE ?", "%"+f.Tag+"%")
	}
	if f.Favorite {
		db = db.Where("favorite = ?", true)
	}

	switch f.Sort {
	case "title":
		db = db.Order("title COLLATE NOCASE ASC")
	case "year":
		db = db.Order("year DESC, title ASC")
	case "rating":
		db = db.Order("rating DESC, title ASC")
	case "playtime":
		db = db.Order("playtime DESC, title ASC")
	case "created":
		db = db.Order("created_at DESC")
	default:
		db = db.Order("updated_at DESC")
	}

	var games []data.Game
	if err := db.Find(&games).Error; err != nil {
		return nil, fmt.Errorf("查询游戏失败: %w", err)
	}
	if len(games) == 0 {
		return []data.Game{}, nil
	}

	if err := s.attachPatches(games); err != nil {
		return nil, err
	}
	return games, nil
}

// attachPatches 一次查出所有补丁并按游戏归组，填到 games[i].Patches。
func (s *GameService) attachPatches(games []data.Game) error {
	ids := make([]uint, 0, len(games))
	for _, g := range games {
		ids = append(ids, g.ID)
	}

	var patches []data.Patch
	if err := s.store.DB.Where("game_id IN ?", ids).Order("created_at ASC").Find(&patches).Error; err != nil {
		return fmt.Errorf("查询补丁失败: %w", err)
	}

	byGame := make(map[uint][]data.Patch, len(games))
	for _, p := range patches {
		byGame[p.GameID] = append(byGame[p.GameID], p)
	}
	for i := range games {
		if list, ok := byGame[games[i].ID]; ok {
			games[i].Patches = list
		} else {
			games[i].Patches = []data.Patch{}
		}
	}
	return nil
}

// Get 读取单条游戏，含补丁与补丁文件清单。
func (s *GameService) Get(id uint) (data.Game, error) {
	var g data.Game
	if err := s.store.DB.First(&g, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return g, fmt.Errorf("游戏不存在: %d", id)
		}
		return g, fmt.Errorf("读取游戏失败: %w", err)
	}

	var patches []data.Patch
	if err := s.store.DB.Where("game_id = ?", id).Order("created_at ASC").Find(&patches).Error; err != nil {
		return g, fmt.Errorf("读取补丁失败: %w", err)
	}
	for i := range patches {
		var files []data.PatchFile
		if err := s.store.DB.Where("patch_id = ?", patches[i].ID).Order("rel_path ASC").Find(&files).Error; err != nil {
			return g, fmt.Errorf("读取补丁文件失败: %w", err)
		}
		if files == nil {
			files = []data.PatchFile{}
		}
		patches[i].Files = files
	}
	if patches == nil {
		patches = []data.Patch{}
	}
	g.Patches = patches
	return g, nil
}

// ---------------------------------------------------------------------------
// 写入
// ---------------------------------------------------------------------------

// Create 新增游戏。海报/横幅若是网络链接或本地路径，会被复制进应用媒体目录。
func (s *GameService) Create(in GameInput) (data.Game, error) {
	var g data.Game
	if err := applyInput(&g, in, s.media); err != nil {
		return g, err
	}
	if strings.TrimSpace(g.Title) == "" {
		return g, fmt.Errorf("标题不能为空")
	}
	if err := s.store.DB.Create(&g).Error; err != nil {
		return g, fmt.Errorf("保存游戏失败: %w", err)
	}
	s.emitChanged(g.ID)
	// 返回时补丁为空切片，前端不用判 nil。
	g.Patches = []data.Patch{}
	return g, nil
}

// Update 覆盖式更新。只改表单提交的字段，不改 ID / 时间戳。
func (s *GameService) Update(id uint, in GameInput) (data.Game, error) {
	var g data.Game
	if err := s.store.DB.First(&g, id).Error; err != nil {
		return g, fmt.Errorf("游戏不存在: %d", id)
	}

	created := g.CreatedAt
	if err := applyInput(&g, in, s.media); err != nil {
		return g, err
	}
	if strings.TrimSpace(g.Title) == "" {
		return g, fmt.Errorf("标题不能为空")
	}
	g.ID = id
	g.CreatedAt = created

	if err := s.store.DB.Save(&g).Error; err != nil {
		return g, fmt.Errorf("保存游戏失败: %w", err)
	}
	s.emitChanged(id)

	return s.Get(id)
}

// Delete 删除游戏，并连带清掉它的补丁记录与目录快照。
//
// 只删数据库记录，绝不碰用户的游戏文件 —— 这是所有删除操作的原则。
func (s *GameService) Delete(id uint) error {
	err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		var patchIDs []uint
		if err := tx.Model(&data.Patch{}).Where("game_id = ?", id).Pluck("id", &patchIDs).Error; err != nil {
			return err
		}
		if len(patchIDs) > 0 {
			if err := tx.Where("patch_id IN ?", patchIDs).Delete(&data.PatchFile{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("game_id = ?", id).Delete(&data.Patch{}).Error; err != nil {
			return err
		}
		if err := tx.Where("game_id = ?", id).Delete(&data.Snapshot{}).Error; err != nil {
			return err
		}
		return tx.Delete(&data.Game{}, id).Error
	})
	if err != nil {
		return fmt.Errorf("删除游戏失败: %w", err)
	}
	s.emitChanged(id)
	return nil
}

// SetFavorite 收藏/取消收藏，库页的快捷操作。
func (s *GameService) SetFavorite(id uint, value bool) error {
	if err := s.store.DB.Model(&data.Game{}).Where("id = ?", id).
		Update("favorite", value).Error; err != nil {
		return fmt.Errorf("更新收藏状态失败: %w", err)
	}
	s.emitChanged(id)
	return nil
}

// SetStatus 改游玩状态。
func (s *GameService) SetStatus(id uint, status string) error {
	if !validStatus(status) {
		return fmt.Errorf("未知游玩状态: %s", status)
	}
	updates := map[string]any{"play_status": status}
	if status == data.StatusPlaying {
		updates["last_played_at"] = time.Now()
	}
	if err := s.store.DB.Model(&data.Game{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新状态失败: %w", err)
	}
	s.emitChanged(id)
	return nil
}

// SetRating 改个人评分（0-10）。
func (s *GameService) SetRating(id uint, rating float64) error {
	if rating < 0 {
		rating = 0
	}
	if rating > 10 {
		rating = 10
	}
	if err := s.store.DB.Model(&data.Game{}).Where("id = ?", id).
		Update("rating", rating).Error; err != nil {
		return fmt.Errorf("更新评分失败: %w", err)
	}
	s.emitChanged(id)
	return nil
}

// AddPlaytime 累加游玩时长（分钟），详情页的"记一笔"用。
func (s *GameService) AddPlaytime(id uint, minutes int) error {
	if minutes <= 0 {
		return fmt.Errorf("时长必须大于 0")
	}
	if err := s.store.DB.Model(&data.Game{}).Where("id = ?", id).
		Updates(map[string]any{
			"playtime":       gorm.Expr("playtime + ?", minutes),
			"last_played_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("更新时长失败: %w", err)
	}
	s.emitChanged(id)
	return nil
}

// applyInput 把表单搬到实体上，并处理图片落地、年份推导这些衍生逻辑。
func applyInput(g *data.Game, in GameInput, media *data.MediaStore) error {
	g.Title = strings.TrimSpace(in.Title)
	g.OriginalTitle = strings.TrimSpace(in.OriginalTitle)
	g.Alias = strings.TrimSpace(in.Alias)
	g.Developer = strings.TrimSpace(in.Developer)
	g.Publisher = strings.TrimSpace(in.Publisher)
	g.ReleaseDate = strings.TrimSpace(in.ReleaseDate)
	g.Description = strings.TrimSpace(in.Description)
	g.Tags = normalizeList(in.Tags)
	g.Genres = normalizeList(in.Genres)
	g.Rating = in.Rating
	g.Favorite = in.Favorite
	g.FolderPath = strings.TrimSpace(in.FolderPath)
	g.ExePath = strings.TrimSpace(in.ExePath)
	g.SavePath = strings.TrimSpace(in.SavePath)
	g.VNDBID = strings.TrimSpace(in.VNDBID)
	g.BangumiID = in.BangumiID
	g.Source = strings.TrimSpace(in.Source)
	if g.Source == "" {
		g.Source = "manual"
	}
	g.Notes = in.Notes

	status := strings.TrimSpace(in.PlayStatus)
	if status == "" {
		status = data.StatusUnplayed
	}
	if !validStatus(status) {
		return fmt.Errorf("未知游玩状态: %s", status)
	}
	g.PlayStatus = status
	if in.Playtime < 0 {
		g.Playtime = 0
	} else {
		g.Playtime = in.Playtime
	}

	g.Year = parseYear(g.ReleaseDate)

	// 图片在这里落地：前端可能提交 VNDB 链接、本地路径，或已经是 /media/ 路径。
	if media != nil {
		poster, err := media.Ingest("posters", in.Poster)
		if err != nil {
			return fmt.Errorf("处理封面失败: %w", err)
		}
		g.Poster = poster

		banner, err := media.Ingest("banners", in.Banner)
		if err != nil {
			return fmt.Errorf("处理横幅失败: %w", err)
		}
		g.Banner = banner
	} else {
		g.Poster = strings.TrimSpace(in.Poster)
		g.Banner = strings.TrimSpace(in.Banner)
	}
	return nil
}

func validStatus(s string) bool {
	for _, v := range data.AllStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// parseYear 从 "2015-06-26" / "2015" 里抽年份，抽不到返回 0。
func parseYear(date string) int {
	date = strings.TrimSpace(date)
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil || y < 1900 || y > 2200 {
		return 0
	}
	return y
}

// normalizeList 把各种奇怪分隔符统一成 ", " 分隔。
func normalizeList(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == '/' || r == '、' || r == ';'
	})
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return strings.Join(out, ", ")
}

// ---------------------------------------------------------------------------
// 启动与文件操作
// ---------------------------------------------------------------------------

// Launch 启动游戏本体。
//
// 优先用用户指定的 ExePath；没指定就在目录里找可执行文件，只有一个就用它，
// 有多个就报错让用户自己选 —— 猜错了启动别的东西更烦。
func (s *GameService) Launch(id uint) error {
	var g data.Game
	if err := s.store.DB.First(&g, id).Error; err != nil {
		return fmt.Errorf("游戏不存在: %d", id)
	}
	if g.FolderPath == "" {
		return fmt.Errorf("还没设置游戏目录")
	}
	if _, err := os.Stat(g.FolderPath); err != nil {
		return fmt.Errorf("游戏目录不存在: %s", g.FolderPath)
	}

	exe := strings.TrimSpace(g.ExePath)
	if exe == "" {
		found, err := findExecutable(g.FolderPath)
		if err != nil {
			return err
		}
		exe = found
	}
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("启动程序不存在: %s", exe)
	}

	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}
	// 不 Wait：游戏是长驻进程，等它退出会把这里卡住。
	go func() { _ = cmd.Wait() }()

	updates := map[string]any{"last_played_at": time.Now()}
	if g.PlayStatus == data.StatusUnplayed {
		updates["play_status"] = data.StatusPlaying
	}
	_ = s.store.DB.Model(&data.Game{}).Where("id = ?", id).Updates(updates).Error
	s.emitChanged(id)
	return nil
}

// findExecutable 在游戏目录里找一个可执行文件。
func findExecutable(folder string) (string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return "", fmt.Errorf("读取目录失败: %w", err)
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".exe") {
			continue
		}
		// 卸载/配置类程序不是游戏本体，跳过。
		if strings.Contains(name, "unins") || strings.Contains(name, "setup") ||
			strings.Contains(name, "config") || strings.Contains(name, "update") {
			continue
		}
		found = append(found, filepath.Join(folder, e.Name()))
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("目录里没有找到 .exe，请手动指定启动程序")
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("目录里有 %d 个可执行文件，请在编辑页手动指定启动程序", len(found))
	}
}

// OpenFolder 打开游戏安装目录。
func (s *GameService) OpenFolder(id uint) error {
	var g data.Game
	if err := s.store.DB.First(&g, id).Error; err != nil {
		return fmt.Errorf("游戏不存在: %d", id)
	}
	if g.FolderPath == "" {
		return fmt.Errorf("还没设置游戏目录")
	}
	return openInExplorer(g.FolderPath)
}

// OpenPath 打开任意路径（目录或文件）。
func (s *GameService) OpenPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("路径为空")
	}
	return openInExplorer(path)
}

// OpenURL 用系统浏览器打开链接（2dfan 补丁页、VNDB 页等）。
func (s *GameService) OpenURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("链接为空")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	if app := application.Get(); app != nil {
		return app.Browser.OpenURL(url)
	}
	return fmt.Errorf("应用未就绪")
}

// ---------------------------------------------------------------------------
// 文件对话框
// ---------------------------------------------------------------------------

// PickFolder 选择游戏安装目录，取消时返回空字符串。
func (s *GameService) PickFolder() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("应用未就绪")
	}
	path, err := app.Dialog.OpenFile().
		SetTitle("选择游戏安装目录").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(true).
		PromptForSingleSelection()
	if err != nil {
		return "", nil // 取消在 Wails 里也报错，当成取消处理
	}
	return path, nil
}

// PickExecutable 选择启动程序，默认从游戏目录开始找。
func (s *GameService) PickExecutable(startDir string) (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("应用未就绪")
	}
	dlg := app.Dialog.OpenFile().
		SetTitle("选择启动程序").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		AddFilter("可执行文件", "*.exe")
	if strings.TrimSpace(startDir) != "" {
		dlg = dlg.SetDirectory(startDir)
	}
	path, err := dlg.PromptForSingleSelection()
	if err != nil {
		return "", nil
	}
	return path, nil
}

// PickImage 选择一张本地图片（封面/横幅），返回绝对路径。
func (s *GameService) PickImage() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("应用未就绪")
	}
	path, err := app.Dialog.OpenFile().
		SetTitle("选择图片").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		AddFilter("图片", "*.jpg;*.jpeg;*.png;*.webp;*.avif;*.bmp;*.gif").
		PromptForSingleSelection()
	if err != nil {
		return "", nil
	}
	return path, nil
}

// FolderStats 概览一个目录的文件数/体积，编辑页选完目录立刻能看到。
func (s *GameService) FolderStats(path string) data.FolderStats {
	return data.StatsFolder(path)
}

// ---------------------------------------------------------------------------
// 聚合数据
// ---------------------------------------------------------------------------

// Developers 返回所有开发者及其作品数，库页的开发者筛选下拉用。
func (s *GameService) Developers() ([]CountItem, error) {
	return s.groupCount("developer")
}

// Tags 返回出现频次最高的标签。
func (s *GameService) Tags() ([]CountItem, error) {
	var rows []struct {
		Tags string
	}
	if err := s.store.DB.Model(&data.Game{}).Select("tags").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取标签失败: %w", err)
	}
	counter := make(map[string]int)
	for _, r := range rows {
		for _, t := range strings.Split(r.Tags, ",") {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			counter[t]++
		}
	}
	return sortedCounts(counter), nil
}

// groupCount 按某一列聚合计数（列名由内部调用传入，不接受外部输入）。
func (s *GameService) groupCount(column string) ([]CountItem, error) {
	var rows []CountItem
	err := s.store.DB.Model(&data.Game{}).
		Select(column + " AS name, COUNT(*) AS count").
		Where(column + " <> ''").
		Group(column).
		Order("count DESC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("统计失败: %w", err)
	}
	if rows == nil {
		rows = []CountItem{}
	}
	return rows, nil
}

func sortedCounts(m map[string]int) []CountItem {
	out := make([]CountItem, 0, len(m))
	for k, v := range m {
		out = append(out, CountItem{Name: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

// Stats 汇总全库数据，供统计页使用。
func (s *GameService) Stats() (Stats, error) {
	var st Stats

	var games []data.Game
	if err := s.store.DB.Find(&games).Error; err != nil {
		return st, fmt.Errorf("读取游戏失败: %w", err)
	}
	st.Total = len(games)

	statusCount := make(map[string]int)
	yearCount := make(map[int]int)
	devCount := make(map[string]int)
	tagCount := make(map[string]int)
	ratingSum := 0.0

	for _, g := range games {
		if g.Favorite {
			st.Favorite++
		}
		if g.Rating > 0 {
			st.Rated++
			ratingSum += g.Rating
		}
		st.TotalPlaytime += g.Playtime
		statusCount[g.PlayStatus]++
		if g.PlayStatus == data.StatusCleared {
			st.ClearedCount++
		}
		if g.Year > 0 {
			yearCount[g.Year]++
		}
		if g.Developer != "" {
			devCount[g.Developer]++
		}
		for _, t := range strings.Split(g.Tags, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tagCount[t]++
			}
		}
		if g.Poster == "" {
			st.NoPoster++
		}
		if g.FolderPath != "" {
			if _, err := os.Stat(g.FolderPath); err != nil {
				st.MissingFolder++
			}
		}
	}
	if st.Rated > 0 {
		st.AvgRating = ratingSum / float64(st.Rated)
	}

	// 状态按固定顺序输出，未游玩排最后，保证图表颜色稳定。
	order := []string{data.StatusPlaying, data.StatusCleared, data.StatusOnHold, data.StatusDropped, data.StatusUnplayed}
	for _, k := range order {
		st.ByStatus = append(st.ByStatus, CountItem{Name: k, Count: statusCount[k]})
	}

	for y, c := range yearCount {
		st.ByYear = append(st.ByYear, CountItem{Name: strconv.Itoa(y), Count: c})
	}
	sort.Slice(st.ByYear, func(i, j int) bool { return st.ByYear[i].Name > st.ByYear[j].Name })
	if len(st.ByYear) > 20 {
		st.ByYear = st.ByYear[:20]
	}

	for k, c := range devCount {
		st.ByDeveloper = append(st.ByDeveloper, CountItem{Name: k, Count: c})
	}
	sort.Slice(st.ByDeveloper, func(i, j int) bool {
		if st.ByDeveloper[i].Count != st.ByDeveloper[j].Count {
			return st.ByDeveloper[i].Count > st.ByDeveloper[j].Count
		}
		return st.ByDeveloper[i].Name < st.ByDeveloper[j].Name
	})
	if len(st.ByDeveloper) > 12 {
		st.ByDeveloper = st.ByDeveloper[:12]
	}

	st.TopTags = sortedCounts(tagCount)
	if len(st.TopTags) > 24 {
		st.TopTags = st.TopTags[:24]
	}

	// 补丁类型分布 —— 一眼看出库里有几部还没汉化。
	var patchRows []CountItem
	if err := s.store.DB.Model(&data.Patch{}).
		Select("kind AS name, COUNT(*) AS count").
		Group("kind").Order("count DESC").Find(&patchRows).Error; err != nil {
		return st, fmt.Errorf("统计补丁失败: %w", err)
	}
	if patchRows == nil {
		patchRows = []CountItem{}
	}
	st.ByPatchKind = patchRows

	// 年度时间线：取有作品的年份，按年升序，给柱状图用。
	minYear, maxYear := 0, 0
	for y := range yearCount {
		if minYear == 0 || y < minYear {
			minYear = y
		}
		if y > maxYear {
			maxYear = y
		}
	}
	if minYear > 0 {
		clearedByYear := make(map[int]int)
		for _, g := range games {
			if g.PlayStatus == data.StatusCleared && g.Year > 0 {
				clearedByYear[g.Year]++
			}
		}
		for y := minYear; y <= maxYear; y++ {
			st.Timeline = append(st.Timeline, TimelinePoint{
				Year:    y,
				Total:   yearCount[y],
				Cleared: clearedByYear[y],
			})
		}
	}

	var recent []data.Game
	if err := s.store.DB.Order("updated_at DESC").Limit(6).Find(&recent).Error; err == nil {
		if recent == nil {
			recent = []data.Game{}
		}
		st.Recent = recent
	}
	if st.ByStatus == nil {
		st.ByStatus = []CountItem{}
	}
	if st.ByYear == nil {
		st.ByYear = []CountItem{}
	}
	if st.ByDeveloper == nil {
		st.ByDeveloper = []CountItem{}
	}
	if st.TopTags == nil {
		st.TopTags = []CountItem{}
	}
	if st.Timeline == nil {
		st.Timeline = []TimelinePoint{}
	}
	return st, nil
}

// MissingGames 返回目录已不存在的游戏，库页可以给个"失效"提示。
func (s *GameService) MissingGames() ([]uint, error) {
	var games []data.Game
	if err := s.store.DB.Select("id", "folder_path").Find(&games).Error; err != nil {
		return nil, fmt.Errorf("读取游戏失败: %w", err)
	}
	var ids []uint
	for _, g := range games {
		if g.FolderPath == "" {
			continue
		}
		if _, err := os.Stat(g.FolderPath); err != nil {
			ids = append(ids, g.ID)
		}
	}
	if ids == nil {
		ids = []uint{}
	}
	return ids, nil
}

// emitChanged 广播游戏变更事件，前端据此刷新列表/详情。
func (s *GameService) emitChanged(id uint) {
	if app := application.Get(); app != nil {
		app.Event.Emit("game:changed", int64(id))
	}
}
