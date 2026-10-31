// metaService.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gWvljBt/config"
)

// MetaService 对接外部视觉小说数据库（VNDB / Bangumi），做"搜索 -> 一键填充"。
//
// 只做查询和字段归一化：图片下载交给 MediaStore，落库交给 GameService，
// 这样以后加第三个数据源（比如 ErogameScape）只要再写一个 adapter。
type MetaService struct {
	cfg    *config.Config
	client *http.Client

	// 两个公开 API 都有速率限制，串行化 + 最小间隔避免被封。
	mu   sync.Mutex
	last time.Time
}

func NewMetaService(cfg *config.Config) *MetaService {
	return &MetaService{
		cfg:    cfg,
		client: &http.Client{Timeout: 25 * time.Second},
		last:   time.Now().Add(-time.Minute),
	}
}

// ---------------------------------------------------------------------------
// 出参
// ---------------------------------------------------------------------------

// SearchHit 是搜索结果里的一条，字段做了跨源归一化。
type SearchHit struct {
	Source        string   `json:"source"`
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	TitleCN       string   `json:"titleCn"`
	OriginalTitle string   `json:"originalTitle"`
	Poster        string   `json:"poster"`
	Released      string   `json:"released"`
	Developer     string   `json:"developer"`
	Score         float64  `json:"score"`
	RatingCount   int      `json:"ratingCount"`
	Description   string   `json:"description"`
	Tags          []string `json:"tags"`
	NSFW          bool     `json:"nsfw"`
}

// MetaDraft 是准备写进数据库的一整套元数据。
type MetaDraft struct {
	Source        string  `json:"source"`
	VNDBID        string  `json:"vndbId"`
	BangumiID     int     `json:"bangumiId"`
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
	LengthMinutes int     `json:"lengthMinutes"`
	NSFW          bool    `json:"nsfw"`
	SourceURL     string  `json:"sourceUrl"`
}

// ---------------------------------------------------------------------------
// 对外方法
// ---------------------------------------------------------------------------

// Search 按数据源搜索视觉小说。
func (s *MetaService) Search(source, query string) ([]SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("请输入关键词")
	}
	switch normalizeSource(source) {
	case "bangumi":
		return s.searchBangumi(query)
	default:
		return s.searchVNDB(query)
	}
}

// Detail 拉取某个条目的完整信息，用于导入表单预填。
func (s *MetaService) Detail(source, id string) (MetaDraft, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return MetaDraft{}, fmt.Errorf("缺少条目 ID")
	}
	switch normalizeSource(source) {
	case "bangumi":
		return s.detailBangumi(id)
	default:
		return s.detailVNDB(id)
	}
}

// Ping 探一下数据源是否可达，设置页的"测试连接"用。
func (s *MetaService) Ping(source string) (string, error) {
	switch normalizeSource(source) {
	case "bangumi":
		s.throttle(1200 * time.Millisecond)
		req, err := http.NewRequest(http.MethodGet, "https://api.bgm.tv/v0/subjects/8", nil)
		if err != nil {
			return "", err
		}
		s.bangumiHeaders(req)
		var out bgmSubject
		if err := s.doJSON(req, &out); err != nil {
			return "", err
		}
		return fmt.Sprintf("Bangumi 连接正常（示例条目：%s）", out.Name), nil
	default:
		s.throttle(500 * time.Millisecond)
		_, err := s.vndbQuery([]any{"id", "=", "v17"}, 1)
		if err != nil {
			return "", err
		}
		return "VNDB 连接正常", nil
	}
}

func normalizeSource(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "bangumi", "bgm":
		return "bangumi"
	default:
		return "vndb"
	}
}

// throttle 保证两次请求之间有最小间隔。
func (s *MetaService) throttle(min time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d := time.Since(s.last); d < min {
		time.Sleep(min - d)
	}
	s.last = time.Now()
}

// doJSON 发请求并把 JSON 响应解到 out，非 2xx 统一转成可读错误。
func (s *MetaService) doJSON(req *http.Request, out any) error {
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return fmt.Errorf("接口返回 HTTP %d: %s", resp.StatusCode, msg)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// VNDB
// ---------------------------------------------------------------------------

// vndbFields 是请求的字段清单。一次拿全，避免详情页再查一遍。
const vndbFields = "id,title,alttitle,titles.title,titles.lang,titles.official,titles.main," +
	"image.url,image.sexual,image.violence,released,rating,votecount,length,description," +
	"developers.name,tags.name,tags.rating,tags.category"

type vndbImage struct {
	URL      string  `json:"url"`
	Sexual   float64 `json:"sexual"`
	Violence float64 `json:"violence"`
}

type vndbTitle struct {
	Lang     string `json:"lang"`
	Title    string `json:"title"`
	Official bool   `json:"official"`
	Main     bool   `json:"main"`
}

type vndbDeveloper struct {
	Name string `json:"name"`
}

type vndbTag struct {
	Name     string  `json:"name"`
	Rating   float64 `json:"rating"`
	Category string  `json:"category"`
}

type vndbVN struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Alttitle    string          `json:"alttitle"`
	Titles      []vndbTitle     `json:"titles"`
	Image       *vndbImage      `json:"image"`
	Released    string          `json:"released"`
	Rating      float64         `json:"rating"`
	VoteCount   int             `json:"votecount"`
	Length      int             `json:"length"`
	Description string          `json:"description"`
	Developers  []vndbDeveloper `json:"developers"`
	Tags        []vndbTag       `json:"tags"`
}

type vndbResponse struct {
	Results []vndbVN `json:"results"`
	More    bool     `json:"more"`
}

// vndbQuery 是 POST /kana/vn 的通用封装。
func (s *MetaService) vndbQuery(filters []any, results int) ([]vndbVN, error) {
	s.throttle(500 * time.Millisecond)

	payload := map[string]any{
		"filters": filters,
		"fields":  vndbFields,
		"results": results,
	}
	// 搜索按相关度排；按 ID 精确查时排序无意义但无害。
	if len(filters) >= 1 && filters[0] == "search" {
		payload["sort"] = "searchrank"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.vndb.org/kana/vn", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gWvljBt/1.0 (+https://github.com/bishop9910/gWvljBt)")
	if token := strings.TrimSpace(s.cfg.Get().VNDBToken); token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}

	var out vndbResponse
	if err := s.doJSON(req, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// searchVNDB 支持两种输入：普通关键词，或直接粘 v17449 这种 ID。
func (s *MetaService) searchVNDB(query string) ([]SearchHit, error) {
	var results []vndbVN
	var err error

	if isVNDBID(query) {
		results, err = s.vndbQuery([]any{"id", "=", strings.ToLower(query)}, 1)
	} else {
		// 原名（日文）搜不到时，VNDB 的 search 已经是模糊匹配，基本够用；
		// 真搜不到就退回空列表，不要报错打扰用户。
		results, err = s.vndbQuery([]any{"search", "=", query}, 12)
	}
	if err != nil {
		return nil, err
	}

	hits := make([]SearchHit, 0, len(results))
	for _, v := range results {
		hits = append(hits, vndbToHit(v))
	}
	return hits, nil
}

func (s *MetaService) detailVNDB(id string) (MetaDraft, error) {
	if !isVNDBID(id) {
		id = "v" + strings.TrimPrefix(strings.ToLower(id), "v")
	}
	results, err := s.vndbQuery([]any{"id", "=", strings.ToLower(id)}, 1)
	if err != nil {
		return MetaDraft{}, err
	}
	if len(results) == 0 {
		return MetaDraft{}, fmt.Errorf("VNDB 上找不到条目: %s", id)
	}
	return vndbToDraft(results[0]), nil
}

func isVNDBID(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if !strings.HasPrefix(s, "v") || len(s) < 2 {
		return false
	}
	_, err := strconv.Atoi(s[1:])
	return err == nil
}

// vndbCNTitle 挑选中文标题（VNDB 的 titles 里偶尔有 zh-Hans/zh-Hant）。
func vndbCNTitle(v vndbVN) string {
	for _, t := range v.Titles {
		switch strings.ToLower(t.Lang) {
		case "zh-hans", "zh-hant", "zh":
			if strings.TrimSpace(t.Title) != "" {
				return t.Title
			}
		}
	}
	return ""
}

// vndbOriginalTitle 优先取官方日文标题，退而求其次用 alttitle。
func vndbOriginalTitle(v vndbVN) string {
	for _, t := range v.Titles {
		if strings.HasPrefix(strings.ToLower(t.Lang), "ja") && t.Official && strings.TrimSpace(t.Title) != "" {
			return t.Title
		}
	}
	if strings.TrimSpace(v.Alttitle) != "" {
		return v.Alttitle
	}
	return ""
}

// vndbTopTags 取权重最高的若干标签。
func vndbTopTags(v vndbVN, limit int) []string {
	out := make([]string, 0, limit)
	for _, t := range v.Tags {
		if t.Rating < 1.5 { // 低于 1.5 的标签基本是噪音
			continue
		}
		out = append(out, t.Name)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func vndbPoster(v vndbVN) string {
	if v.Image == nil {
		return ""
	}
	return v.Image.URL
}

func vndbToHit(v vndbVN) SearchHit {
	desc := stripBBCode(v.Description)
	if len([]rune(desc)) > 220 {
		desc = string([]rune(desc)[:220]) + "…"
	}
	dev := ""
	if len(v.Developers) > 0 {
		dev = v.Developers[0].Name
	}
	nsfw := v.Image != nil && (v.Image.Sexual >= 1.5 || v.Image.Violence >= 1.5)

	return SearchHit{
		Source:        "vndb",
		ID:            v.ID,
		Title:         v.Title,
		TitleCN:       vndbCNTitle(v),
		OriginalTitle: vndbOriginalTitle(v),
		Poster:        vndbPoster(v),
		Released:      v.Released,
		Developer:     dev,
		Score:         v.Rating,
		RatingCount:   v.VoteCount,
		Description:   desc,
		Tags:          vndbTopTags(v, 8),
		NSFW:          nsfw,
	}
}

func vndbToDraft(v vndbVN) MetaDraft {
	cn := vndbCNTitle(v)
	original := vndbOriginalTitle(v)

	// 主标题优先用中文名；没有中文名就用 VNDB 的 title。
	title := v.Title
	alias := ""
	if cn != "" && cn != v.Title {
		title = cn
		if v.Title != "" && v.Title != cn {
			alias = v.Title
		}
	}

	devs := make([]string, 0, len(v.Developers))
	for _, d := range v.Developers {
		if strings.TrimSpace(d.Name) != "" {
			devs = append(devs, d.Name)
		}
	}

	nsfw := v.Image != nil && (v.Image.Sexual >= 1.5 || v.Image.Violence >= 1.5)

	return MetaDraft{
		Source:        "vndb",
		VNDBID:        v.ID,
		Title:         title,
		OriginalTitle: original,
		Alias:         alias,
		Poster:        vndbPoster(v),
		Developer:     strings.Join(devs, ", "),
		ReleaseDate:   v.Released,
		Description:   stripBBCode(v.Description),
		Tags:          strings.Join(vndbTopTags(v, 12), ", "),
		Rating:        v.Rating,
		LengthMinutes: v.Length,
		NSFW:          nsfw,
		SourceURL:     "https://vndb.org/" + v.ID,
	}
}

// ---------------------------------------------------------------------------
// Bangumi
// ---------------------------------------------------------------------------

type bgmRating struct {
	Rank  int     `json:"rank"`
	Total int     `json:"total"`
	Score float64 `json:"score"`
}

type bgmImages struct {
	Large  string `json:"large"`
	Common string `json:"common"`
	Medium string `json:"medium"`
	Small  string `json:"small"`
	Grid   string `json:"grid"`
}

type bgmTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type bgmInfoboxItem struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type bgmSubject struct {
	ID      int              `json:"id"`
	Name    string           `json:"name"`
	NameCN  string           `json:"name_cn"`
	Summary string           `json:"summary"`
	Date    string           `json:"date"`
	Images  bgmImages        `json:"images"`
	Rating  bgmRating        `json:"rating"`
	Tags    []bgmTag         `json:"tags"`
	Infobox []bgmInfoboxItem `json:"infobox"`
	NSFW    bool             `json:"nsfw"`
}

type bgmSearchResponse struct {
	Data  []bgmSubject `json:"data"`
	Total int          `json:"total"`
}

// bangumiHeaders 补齐 Bangumi 要求的头。
// 官方明确要求带可识别的 User-Agent，否则会 403。
func (s *MetaService) bangumiHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "gWvljBt/1.0 (+https://github.com/bishop9910/gWvljBt)")
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(s.cfg.Get().BangumiToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func (s *MetaService) searchBangumi(query string) ([]SearchHit, error) {
	// 直接粘 subject id 的话走详情接口，搜索结果里没有这个 ID 反而奇怪。
	if id, err := strconv.Atoi(strings.TrimSpace(query)); err == nil && id > 0 {
		draft, err := s.detailBangumi(strconv.Itoa(id))
		if err != nil {
			return nil, err
		}
		return []SearchHit{{
			Source:        "bangumi",
			ID:            strconv.Itoa(draft.BangumiID),
			Title:         draft.Title,
			TitleCN:       draft.Title,
			OriginalTitle: draft.OriginalTitle,
			Poster:        draft.Poster,
			Released:      draft.ReleaseDate,
			Developer:     draft.Developer,
			Score:         draft.Rating,
			Description:   truncateRunes(draft.Description, 220),
			Tags:          splitList(draft.Tags),
			NSFW:          draft.NSFW,
		}}, nil
	}

	s.throttle(1200 * time.Millisecond)

	payload := map[string]any{
		"keyword": query,
		"filter": map[string]any{
			"type": []int{4}, // 4 = 游戏
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.bgm.tv/v0/search/subjects?limit=12&offset=0", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	s.bangumiHeaders(req)

	var out bgmSearchResponse
	if err := s.doJSON(req, &out); err != nil {
		return nil, err
	}

	hits := make([]SearchHit, 0, len(out.Data))
	for _, sub := range out.Data {
		hits = append(hits, bgmToHit(sub))
	}
	return hits, nil
}

func (s *MetaService) detailBangumi(id string) (MetaDraft, error) {
	s.throttle(1200 * time.Millisecond)

	req, err := http.NewRequest(http.MethodGet, "https://api.bgm.tv/v0/subjects/"+id, nil)
	if err != nil {
		return MetaDraft{}, fmt.Errorf("构造请求失败: %w", err)
	}
	s.bangumiHeaders(req)

	var sub bgmSubject
	if err := s.doJSON(req, &sub); err != nil {
		return MetaDraft{}, err
	}
	return bgmToDraft(sub), nil
}

func bgmPoster(sub bgmSubject) string {
	for _, u := range []string{sub.Images.Large, sub.Images.Common, sub.Images.Medium} {
		if strings.TrimSpace(u) != "" {
			return u
		}
	}
	return ""
}

func bgmToHit(sub bgmSubject) SearchHit {
	tags := make([]string, 0, 8)
	for _, t := range sub.Tags {
		if t.Count <= 0 {
			continue
		}
		tags = append(tags, t.Name)
		if len(tags) >= 8 {
			break
		}
	}
	return SearchHit{
		Source:        "bangumi",
		ID:            strconv.Itoa(sub.ID),
		Title:         firstNonEmpty(sub.NameCN, sub.Name),
		TitleCN:       sub.NameCN,
		OriginalTitle: sub.Name,
		Poster:        bgmPoster(sub),
		Released:      sub.Date,
		Developer:     bgmInfoboxValue(sub, "开发", "开发商", "開発", "发行", "发行商"),
		Score:         sub.Rating.Score,
		RatingCount:   sub.Rating.Total,
		Description:   truncateRunes(strings.TrimSpace(sub.Summary), 220),
		Tags:          tags,
		NSFW:          sub.NSFW,
	}
}

func bgmToDraft(sub bgmSubject) MetaDraft {
	tags := make([]string, 0, 12)
	for _, t := range sub.Tags {
		if t.Count <= 0 {
			continue
		}
		tags = append(tags, t.Name)
		if len(tags) >= 12 {
			break
		}
	}

	dev := bgmInfoboxValue(sub, "开发", "开发商", "開発")
	pub := bgmInfoboxValue(sub, "发行", "发行商", "出版社")

	return MetaDraft{
		Source:        "bangumi",
		BangumiID:     sub.ID,
		Title:         firstNonEmpty(sub.NameCN, sub.Name),
		OriginalTitle: sub.Name,
		Alias:         "",
		Poster:        bgmPoster(sub),
		Developer:     dev,
		Publisher:     pub,
		ReleaseDate:   sub.Date,
		Description:   strings.TrimSpace(sub.Summary),
		Tags:          strings.Join(tags, ", "),
		Rating:        sub.Rating.Score,
		LengthMinutes: parseLengthMinutes(bgmInfoboxValue(sub, "游戏时长", "游玩时间", "时长")),
		NSFW:          sub.NSFW,
		SourceURL:     "https://bgm.tv/subject/" + strconv.Itoa(sub.ID),
	}
}

// bgmInfoboxValue 在 infobox 里按 key 找值。
//
// Bangumi 的 value 有两种形态：纯字符串，或 [{v,k}] 数组（多值带注）。
// 这里统一展平成用 " / " 连接的字符串。
func bgmInfoboxValue(sub bgmSubject, keys ...string) string {
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	for _, item := range sub.Infobox {
		if !want[strings.TrimSpace(item.Key)] {
			continue
		}
		return flattenInfoboxValue(item.Value)
	}
	return ""
}

func flattenInfoboxValue(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}

	// 可能是 [{"v":"Key","k":"日文名"}, ...] 或 ["a","b"]
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return ""
	}
	parts := make([]string, 0, len(arr))
	for _, el := range arr {
		var str string
		if err := json.Unmarshal(el, &str); err == nil {
			if v := strings.TrimSpace(str); v != "" {
				parts = append(parts, v)
			}
			continue
		}
		var obj struct {
			V string `json:"v"`
			K string `json:"k"`
		}
		if err := json.Unmarshal(el, &obj); err == nil && strings.TrimSpace(obj.V) != "" {
			parts = append(parts, strings.TrimSpace(obj.V))
		}
	}
	return strings.Join(parts, " / ")
}

// parseLengthMinutes 把 "42h" / "12小时" / "8" 这类写法转成分钟。
func parseLengthMinutes(v string) int {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return 0
	}
	num := regexp.MustCompile(`[0-9]+(\.[0-9]+)?`).FindString(v)
	if num == "" {
		return 0
	}
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	// 带 h / 小时 的按小时算，否则认为已经是分钟。
	if strings.Contains(v, "h") || strings.Contains(v, "小时") {
		return int(f * 60)
	}
	return int(f)
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// stripBBCode 把 VNDB 的 BBCode 描述转成纯文本。
//
// 除了去标签，还要丢掉 [spoiler] 里的内容 —— 剧透不该出现在列表里。
func stripBBCode(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	var b strings.Builder
	spoiler := 0
	for i := 0; i < len(s); {
		if s[i] == '[' {
			if j := strings.IndexByte(s[i:], ']'); j >= 0 {
				tag := s[i+1 : i+j]
				name := strings.ToLower(strings.TrimPrefix(tag, "/"))
				if k := strings.IndexByte(name, '='); k >= 0 {
					name = name[:k]
				}
				if name == "spoiler" {
					if strings.HasPrefix(tag, "/") {
						if spoiler > 0 {
							spoiler--
						}
					} else {
						spoiler++
					}
				}
				i += j + 1
				continue
			}
			b.WriteByte(s[i])
			i++
			continue
		}
		if spoiler == 0 {
			b.WriteByte(s[i])
		}
		i++
	}

	// 压缩连续空行，顺手去掉行尾空白。
	lines := strings.Split(b.String(), "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, l := range lines {
		l = strings.TrimRight(l, " \t\r")
		if strings.TrimSpace(l) == "" {
			blank++
			if blank > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blank = 0
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
