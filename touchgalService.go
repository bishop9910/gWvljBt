package main

// TouchGal 的资源检索。
//
// 站点整个挂在 Cloudflare 后面，普通 HTTP 客户端拿到的只会是挑战页，所以这里
// 完全依赖用户从浏览器里复制来的 Cookie（cf_clearance 是核心，登录令牌可选）。
// 请求头也照着站点前端的习惯带上 x-requested-with 与 referer，否则一样会被挡。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gWvljBt/config"
)

const (
	touchgalUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	// 站点没写缓存，20 秒足够；真被 Cloudflare 卡住时早点报错比一直转圈好。
	touchgalTimeout = 20 * time.Second
)

// TouchGalHit 是搜索结果里的一条作品。
type TouchGalHit struct {
	ID       int     `json:"id"`
	UniqueID string  `json:"uniqueId"`
	Name     string  `json:"name"`
	Banner   string  `json:"banner"`
	Rating   float64 `json:"rating"`
	Language string  `json:"language"`
	Platform string  `json:"platform"`
	Type     string  `json:"type"`
	PageURL  string  `json:"pageUrl"`
}

// TouchGalLink 是一条网盘链接。
type TouchGalLink struct {
	Storage  string `json:"storage"`
	Size     string `json:"size"`
	URL      string `json:"url"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

// TouchGalResource 是某部作品下的一个资源（补丁/本体/MOD）。
type TouchGalResource struct {
	ID       int            `json:"id"`
	Name     string         `json:"name"`
	Section  string         `json:"section"`
	Type     string         `json:"type"`
	Language string         `json:"language"`
	Note     string         `json:"note"`
	Platform string         `json:"platform"`
	Links    []TouchGalLink `json:"links"`
}

// TouchGalService 负责和 TouchGal 通信。
type TouchGalService struct {
	cfg    *config.Config
	client *http.Client
}

func NewTouchGalService(cfg *config.Config) *TouchGalService {
	return &TouchGalService{
		cfg:    cfg,
		client: &http.Client{Timeout: touchgalTimeout},
	}
}

// Ping 用 /api/home/random 探一下 Cookie 还能不能用。
func (s *TouchGalService) Ping() (string, error) {
	var payload struct {
		UniqueID string `json:"uniqueId"`
	}
	if err := s.getJSON("api/home/random", &payload); err != nil {
		return "", err
	}
	if payload.UniqueID == "" {
		return "", fmt.Errorf("接口有响应但没返回内容，站点结构可能变了")
	}
	return "连接正常，示例作品：" + payload.UniqueID, nil
}

// Search 按标题搜作品。limit 为 0 时用 12。
func (s *TouchGalService) Search(query string, limit int) ([]TouchGalHit, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("先填一个要搜的标题")
	}
	if limit <= 0 || limit > 30 {
		limit = 12
	}

	// queryString 是一个被字符串化的数组，这是站点自己的约定。
	conds, err := json.Marshal([]map[string]string{{"type": "keyword", "name": q}})
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"queryString": string(conds),
		"limit":       limit,
		"page":        1,
		"searchOption": map[string]bool{
			"searchInIntroduction": false,
			"searchInAlias":        true,
			"searchInTag":          false,
		},
		"selectedType":     "all",
		"selectedLanguage": "all",
		"selectedPlatform": "all",
		"sortField":        "resource_update_time",
		"sortOrder":        "desc",
		"selectedYears":    []string{"all"},
		"selectedMonths":   []string{"all"},
	}

	var payload struct {
		Galgames []struct {
			ID            int      `json:"id"`
			UniqueID      string   `json:"uniqueId"`
			Name          string   `json:"name"`
			Banner        string   `json:"banner"`
			AverageRating float64  `json:"averageRating"`
			Type          []string `json:"type"`
			Language      []string `json:"language"`
			Platform      []string `json:"platform"`
		} `json:"galgames"`
		Total int `json:"total"`
	}
	if err := s.postJSON("api/search/", body, &payload); err != nil {
		return nil, err
	}

	base := s.baseURL()
	hits := make([]TouchGalHit, 0, len(payload.Galgames))
	for _, item := range payload.Galgames {
		hits = append(hits, TouchGalHit{
			ID:       item.ID,
			UniqueID: item.UniqueID,
			Name:     item.Name,
			Banner:   item.Banner,
			Rating:   math.Round(item.AverageRating*100) / 100,
			Language: strings.Join(item.Language, " / "),
			Platform: strings.Join(item.Platform, " / "),
			Type:     strings.Join(item.Type, " / "),
			PageURL:  base + item.UniqueID,
		})
	}
	return hits, nil
}

// Resources 拉某个作品的资源列表（含网盘链接）。
func (s *TouchGalService) Resources(patchID int) ([]TouchGalResource, error) {
	if patchID <= 0 {
		return nil, fmt.Errorf("作品 ID 无效")
	}

	var payload []struct {
		ID       int      `json:"id"`
		Name     string   `json:"name"`
		Section  string   `json:"section"`
		Type     []string `json:"type"`
		Language []string `json:"language"`
		Note     string   `json:"note"`
		Platform []string `json:"platform"`
		Links    []struct {
			Storage  string `json:"storage"`
			Size     string `json:"size"`
			Content  string `json:"content"`
			Code     string `json:"code"`
			Password string `json:"password"`
		} `json:"links"`
	}
	path := "api/patch/resource?patchId=" + strconv.Itoa(patchID)
	if err := s.getJSON(path, &payload); err != nil {
		return nil, err
	}

	out := make([]TouchGalResource, 0, len(payload))
	for _, item := range payload {
		res := TouchGalResource{
			ID:       item.ID,
			Name:     item.Name,
			Section:  item.Section,
			Type:     strings.Join(item.Type, " / "),
			Language: strings.Join(item.Language, " / "),
			Note:     item.Note,
			Platform: strings.Join(item.Platform, " / "),
			Links:    make([]TouchGalLink, 0, len(item.Links)),
		}
		for _, link := range item.Links {
			res.Links = append(res.Links, TouchGalLink{
				Storage:  link.Storage,
				Size:     link.Size,
				URL:      link.Content,
				Code:     link.Code,
				Password: link.Password,
			})
		}
		out = append(out, res)
	}
	return out, nil
}

func (s *TouchGalService) baseURL() string {
	raw := strings.TrimSpace(s.cfg.Get().TouchgalURL)
	if raw == "" {
		raw = "https://www.touchgal.ink/"
	}
	if !strings.HasSuffix(raw, "/") {
		raw += "/"
	}
	return raw
}

func (s *TouchGalService) getJSON(path string, out any) error {
	return s.do(path, http.MethodGet, nil, out)
}

func (s *TouchGalService) postJSON(path string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return s.do(path, http.MethodPost, raw, out)
}

func (s *TouchGalService) do(path, method string, body []byte, out any) error {
	base := s.baseURL()
	endpoint := strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", touchgalUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Referer", base)
	// 站点前端所有请求都带这个头，少了会拿到 403。
	req.Header.Set("x-requested-with", "kun-fetch")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie := s.cookieHeader(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("连不上 %s：%v", base, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 挑战页经常还是 403，但也有 200 带 HTML 的情况，所以正文也要看。
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	text := string(data)

	if resp.StatusCode == http.StatusForbidden || strings.Contains(text, "Just a moment") || strings.Contains(text, "cf-chl") {
		return fmt.Errorf("被 Cloudflare 拦下了（可能是 Cookie 过期）：打开 %s 过一次人机验证，再把浏览器里的 cookie 整段复制到「设置 → TouchGal Cookie」", base)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TouchGal 返回 %d：%s", resp.StatusCode, firstLine(text))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("返回的不是 JSON（站点可能改版或要求验证）：%s", firstLine(text))
	}
	return nil
}

// cookieHeader 拼 Cookie：站点需要的那几个偏好项 + 用户填的通行证 + 可选登录令牌。
func (s *TouchGalService) cookieHeader() string {
	cfg := s.cfg.Get()
	parts := []string{
		"kun-patch-setting-store=all",
		"kun-patch-setting-state=all",
		"kun-patch-setting-data=all",
		"kunNsfwEnable=all",
	}

	raw := strings.TrimSpace(cfg.TouchgalCookie)
	if raw != "" {
		if strings.Contains(raw, "=") {
			// 用户整段粘过来的：去掉可能的 "Cookie:" 前缀和换行。
			cleaned := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "Cookie:"))
			cleaned = strings.Join(strings.Fields(cleaned), " ")
			cleaned = strings.TrimSuffix(cleaned, ";")
			parts = append(parts, cleaned)
		} else {
			parts = append(parts, "cf_clearance="+url.QueryEscape(raw))
		}
	}

	if token := strings.TrimSpace(cfg.TouchgalToken); token != "" {
		parts = append(parts, "kun-galgame-patch-moe-token="+url.QueryEscape(token))
	}
	return strings.Join(parts, "; ")
}

func firstLine(text string) string {
	trimmed := strings.TrimSpace(text)
	if idx := strings.IndexByte(trimmed, '\n'); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	if len(trimmed) > 160 {
		trimmed = trimmed[:160] + "…"
	}
	if trimmed == "" {
		return "（空响应）"
	}
	return trimmed
}
