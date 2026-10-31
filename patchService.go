// patchService.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gWvljBt/data"

	"github.com/wailsapp/wails/v3/pkg/application"
	"gorm.io/gorm"
)

// PatchService 管理补丁记录：哪些文件属于哪个补丁、补丁是否还在、
// 以及"卸载补丁"时把记录过的文件从磁盘上删掉。
//
// 应用本身不提供补丁下载 —— 用户自己去 2dfan 找，装完回来这里登记。
// 这样做的好处是法律上干净，而且能精确到文件：删补丁只删它真正覆盖的那几个。
type PatchService struct {
	store *data.Store
}

func NewPatchService(store *data.Store) *PatchService {
	return &PatchService{store: store}
}

// PatchInput 是前端提交的补丁表单。
type PatchInput struct {
	GameID    uint     `json:"gameId"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Version   string   `json:"version"`
	Source    string   `json:"source"`
	SourceURL string   `json:"sourceUrl"`
	Note      string   `json:"note"`
	Installed bool     `json:"installed"`
	RelPaths  []string `json:"relPaths"`
}

// PatchRow 是全局补丁列表里的一行，带上所属游戏信息方便跨游戏查看。
type PatchRow struct {
	Patch      data.Patch `json:"patch"`
	GameTitle  string     `json:"gameTitle"`
	GamePoster string     `json:"gamePoster"`
}

// DetectResult 是"检测目录变化"的结果。
type DetectResult struct {
	// HadBaseline=false 表示这次只是建立了基线，没有可比对的东西。
	HadBaseline bool              `json:"hadBaseline"`
	Changes     []data.FileChange `json:"changes"`
	Total       int               `json:"total"`
	Truncated   bool              `json:"truncated"`
	ScannedAt   int64             `json:"scannedAt"`
}

// UninstallResult 描述卸载补丁动了哪些文件。
type UninstallResult struct {
	Deleted int      `json:"deleted"`
	Missing int      `json:"missing"`
	Failed  []string `json:"failed"`
}

// maxChanges 限制一次返回的变化条数，避免把几万个文件塞给界面。
const maxChanges = 3000

// ---------------------------------------------------------------------------
// 查询
// ---------------------------------------------------------------------------

// List 返回某个游戏的全部补丁（含文件清单）。
func (s *PatchService) List(gameID uint) ([]data.Patch, error) {
	var patches []data.Patch
	if err := s.store.DB.Where("game_id = ?", gameID).Order("created_at ASC").Find(&patches).Error; err != nil {
		return nil, fmt.Errorf("查询补丁失败: %w", err)
	}
	if len(patches) == 0 {
		return []data.Patch{}, nil
	}

	ids := make([]uint, 0, len(patches))
	for _, p := range patches {
		ids = append(ids, p.ID)
	}
	var files []data.PatchFile
	if err := s.store.DB.Where("patch_id IN ?", ids).Order("rel_path ASC").Find(&files).Error; err != nil {
		return nil, fmt.Errorf("查询补丁文件失败: %w", err)
	}
	byPatch := make(map[uint][]data.PatchFile, len(patches))
	for _, f := range files {
		byPatch[f.PatchID] = append(byPatch[f.PatchID], f)
	}
	for i := range patches {
		if list, ok := byPatch[patches[i].ID]; ok {
			patches[i].Files = list
		} else {
			patches[i].Files = []data.PatchFile{}
		}
	}
	return patches, nil
}

// All 返回全部补丁，带上所属游戏标题，供补丁总览页使用。
func (s *PatchService) All() ([]PatchRow, error) {
	var patches []data.Patch
	if err := s.store.DB.Order("game_id ASC, created_at ASC").Find(&patches).Error; err != nil {
		return nil, fmt.Errorf("查询补丁失败: %w", err)
	}
	if len(patches) == 0 {
		return []PatchRow{}, nil
	}

	var games []data.Game
	if err := s.store.DB.Select("id", "title", "poster").Find(&games).Error; err != nil {
		return nil, fmt.Errorf("查询游戏失败: %w", err)
	}
	gameMap := make(map[uint]data.Game, len(games))
	for _, g := range games {
		gameMap[g.ID] = g
	}

	// 顺带统计每个补丁的文件数，总览页要显示"覆盖 N 个文件"。
	type countRow struct {
		PatchID uint
		Count   int
	}
	var counts []countRow
	if err := s.store.DB.Model(&data.PatchFile{}).
		Select("patch_id, COUNT(*) AS count").Group("patch_id").Find(&counts).Error; err != nil {
		return nil, fmt.Errorf("统计补丁文件失败: %w", err)
	}
	countMap := make(map[uint]int, len(counts))
	for _, c := range counts {
		countMap[c.PatchID] = c.Count
	}

	rows := make([]PatchRow, 0, len(patches))
	for _, p := range patches {
		g := gameMap[p.GameID]
		p.Files = make([]data.PatchFile, countMap[p.ID])
		rows = append(rows, PatchRow{Patch: p, GameTitle: g.Title, GamePoster: g.Poster})
	}
	return rows, nil
}

// Verify 检查补丁记录的文件是否还在磁盘上，返回已丢失的相对路径。
func (s *PatchService) Verify(patchID uint) ([]string, error) {
	p, root, err := s.loadWithGame(patchID)
	if err != nil {
		return nil, err
	}
	var files []data.PatchFile
	if err := s.store.DB.Where("patch_id = ?", p.ID).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("读取补丁文件失败: %w", err)
	}
	missing := make([]string, 0)
	for _, f := range files {
		full, err := data.SafeJoin(root, f.RelPath)
		if err != nil {
			missing = append(missing, f.RelPath)
			continue
		}
		if _, err := os.Stat(full); err != nil {
			missing = append(missing, f.RelPath)
		}
	}
	return missing, nil
}

// loadWithGame 取出补丁及所属游戏的安装目录，顺带校验目录存在。
func (s *PatchService) loadWithGame(patchID uint) (data.Patch, string, error) {
	var p data.Patch
	if err := s.store.DB.First(&p, patchID).Error; err != nil {
		return p, "", fmt.Errorf("补丁不存在: %d", patchID)
	}
	var g data.Game
	if err := s.store.DB.First(&g, p.GameID).Error; err != nil {
		return p, "", fmt.Errorf("补丁所属游戏不存在: %d", p.GameID)
	}
	if strings.TrimSpace(g.FolderPath) == "" {
		return p, "", fmt.Errorf("游戏还没设置安装目录，无法定位补丁文件")
	}
	return p, g.FolderPath, nil
}

// ---------------------------------------------------------------------------
// 写入
// ---------------------------------------------------------------------------

// Create 新建补丁记录，并立即把目录快照刷新到当前状态。
//
// 刷新快照很关键：否则下次"检测变化"还会把这批文件再报一遍，用户得手动排除。
func (s *PatchService) Create(in PatchInput) (data.Patch, error) {
	var out data.Patch
	if in.GameID == 0 {
		return out, fmt.Errorf("缺少游戏 ID")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return out, fmt.Errorf("补丁名称不能为空")
	}

	var g data.Game
	if err := s.store.DB.First(&g, in.GameID).Error; err != nil {
		return out, fmt.Errorf("游戏不存在: %d", in.GameID)
	}

	kind := in.Kind
	if !validPatchKind(kind) {
		kind = data.PatchOther
	}

	p := data.Patch{
		GameID:    in.GameID,
		Name:      in.Name,
		Kind:      kind,
		Version:   strings.TrimSpace(in.Version),
		Source:    strings.TrimSpace(in.Source),
		SourceURL: strings.TrimSpace(in.SourceURL),
		Note:      in.Note,
		Installed: in.Installed,
	}
	if p.Installed {
		now := time.Now()
		p.InstalledAt = &now
	}

	err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
		files, err := s.buildFiles(g.FolderPath, p.ID, in.RelPaths)
		if err != nil {
			return err
		}
		if len(files) > 0 {
			if err := tx.CreateInBatches(&files, 200).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return out, fmt.Errorf("保存补丁失败: %w", err)
	}

	// 快照刷新放在事务外：扫描很慢，不该占着写锁。
	if g.FolderPath != "" {
		_, _ = s.rebuildSnapshot(g)
	}

	s.emitChanged(in.GameID)
	return s.Get(p.ID)
}

// Update 更新补丁的元信息；传了 RelPaths 则整体替换文件清单。
func (s *PatchService) Update(id uint, in PatchInput) (data.Patch, error) {
	var p data.Patch
	if err := s.store.DB.First(&p, id).Error; err != nil {
		return p, fmt.Errorf("补丁不存在: %d", id)
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		return p, fmt.Errorf("补丁名称不能为空")
	}
	kind := in.Kind
	if !validPatchKind(kind) {
		kind = data.PatchOther
	}

	p.Name = name
	p.Kind = kind
	p.Version = strings.TrimSpace(in.Version)
	p.Source = strings.TrimSpace(in.Source)
	p.SourceURL = strings.TrimSpace(in.SourceURL)
	p.Note = in.Note

	if p.Installed != in.Installed {
		p.Installed = in.Installed
		if in.Installed {
			now := time.Now()
			p.InstalledAt = &now
		} else {
			p.InstalledAt = nil
		}
	}

	var g data.Game
	if err := s.store.DB.First(&g, p.GameID).Error; err != nil {
		return p, fmt.Errorf("所属游戏不存在: %d", p.GameID)
	}

	err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&p).Error; err != nil {
			return err
		}
		if in.RelPaths != nil {
			if err := tx.Where("patch_id = ?", p.ID).Delete(&data.PatchFile{}).Error; err != nil {
				return err
			}
			files, err := s.buildFiles(g.FolderPath, p.ID, in.RelPaths)
			if err != nil {
				return err
			}
			if len(files) > 0 {
				return tx.CreateInBatches(&files, 200).Error
			}
		}
		return nil
	})
	if err != nil {
		return p, fmt.Errorf("保存补丁失败: %w", err)
	}

	s.emitChanged(p.GameID)
	return s.Get(p.ID)
}

// Get 读取单条补丁（含文件清单）。
func (s *PatchService) Get(id uint) (data.Patch, error) {
	var p data.Patch
	if err := s.store.DB.First(&p, id).Error; err != nil {
		return p, fmt.Errorf("补丁不存在: %d", id)
	}
	var files []data.PatchFile
	if err := s.store.DB.Where("patch_id = ?", id).Order("rel_path ASC").Find(&files).Error; err != nil {
		return p, fmt.Errorf("读取补丁文件失败: %w", err)
	}
	if files == nil {
		files = []data.PatchFile{}
	}
	p.Files = files
	return p, nil
}

// Delete 只删除补丁记录，磁盘文件原样保留。
func (s *PatchService) Delete(id uint) error {
	var p data.Patch
	if err := s.store.DB.First(&p, id).Error; err != nil {
		return fmt.Errorf("补丁不存在: %d", id)
	}
	err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("patch_id = ?", id).Delete(&data.PatchFile{}).Error; err != nil {
			return err
		}
		return tx.Delete(&data.Patch{}, id).Error
	})
	if err != nil {
		return fmt.Errorf("删除补丁失败: %w", err)
	}
	s.emitChanged(p.GameID)
	return nil
}

// Uninstall 卸载补丁：按记录把覆盖过的文件从磁盘删掉，再删记录。
//
// deleteFiles=false 时退化成"只删记录"（例如补丁已经手动删干净了）。
// 删除只针对记录里的相对路径，并且每个路径都过 SafeJoin 挡穿越。
func (s *PatchService) Uninstall(id uint, deleteFiles bool) (UninstallResult, error) {
	var res UninstallResult
	res.Failed = []string{}

	p, root, err := s.loadWithGame(id)
	if err != nil {
		return res, err
	}

	if deleteFiles {
		var files []data.PatchFile
		if err := s.store.DB.Where("patch_id = ?", p.ID).Find(&files).Error; err != nil {
			return res, fmt.Errorf("读取补丁文件失败: %w", err)
		}

		dirs := make(map[string]bool)
		for _, f := range files {
			full, err := data.SafeJoin(root, f.RelPath)
			if err != nil {
				res.Failed = append(res.Failed, f.RelPath)
				continue
			}
			info, statErr := os.Stat(full)
			if statErr != nil {
				res.Missing++
				continue
			}
			if info.IsDir() {
				res.Missing++
				continue
			}
			if err := os.Remove(full); err != nil {
				res.Failed = append(res.Failed, f.RelPath)
				continue
			}
			res.Deleted++
			if dir := filepath.Dir(full); dir != root {
				dirs[dir] = true
			}
		}

		// 尽量把因此变空的子目录也清掉（从深到浅）。
		for dir := range dirs {
			_ = os.Remove(dir) // 非空时 os.Remove 会失败，忽略即可
		}
	}

	err = s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("patch_id = ?", p.ID).Delete(&data.PatchFile{}).Error; err != nil {
			return err
		}
		return tx.Delete(&data.Patch{}, p.ID).Error
	})
	if err != nil {
		return res, fmt.Errorf("删除补丁记录失败: %w", err)
	}

	s.emitChanged(p.GameID)
	return res, nil
}

// AddFiles 往已有补丁里追加文件。
func (s *PatchService) AddFiles(patchID uint, relPaths []string) error {
	p, root, err := s.loadWithGame(patchID)
	if err != nil {
		return err
	}
	files, err := s.buildFiles(root, p.ID, relPaths)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	// 已存在的路径先删掉再插，避免重复行。
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.RelPath)
	}
	err = s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("patch_id = ? AND rel_path IN ?", p.ID, paths).
			Delete(&data.PatchFile{}).Error; err != nil {
			return err
		}
		return tx.CreateInBatches(&files, 200).Error
	})
	if err != nil {
		return fmt.Errorf("追加补丁文件失败: %w", err)
	}
	s.emitChanged(p.GameID)
	return nil
}

// RemoveFiles 从补丁里移除若干文件记录（不删磁盘）。
func (s *PatchService) RemoveFiles(patchID uint, relPaths []string) error {
	p, _, err := s.loadWithGame(patchID)
	if err != nil {
		return err
	}
	clean := make([]string, 0, len(relPaths))
	for _, r := range relPaths {
		if v, err := data.SafeRel(r); err == nil {
			clean = append(clean, v)
		}
	}
	if len(clean) == 0 {
		return nil
	}
	if err := s.store.DB.Where("patch_id = ? AND rel_path IN ?", p.ID, clean).
		Delete(&data.PatchFile{}).Error; err != nil {
		return fmt.Errorf("移除补丁文件失败: %w", err)
	}
	s.emitChanged(p.GameID)
	return nil
}

// buildFiles 把相对路径列表变成可插入的 PatchFile，顺便补上 size/op。
func (s *PatchService) buildFiles(root string, patchID uint, relPaths []string) ([]data.PatchFile, error) {
	if len(relPaths) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(relPaths))
	out := make([]data.PatchFile, 0, len(relPaths))

	for _, raw := range relPaths {
		rel, err := data.SafeRel(raw)
		if err != nil {
			continue // 非法路径直接忽略，不要因为一条脏数据把整批拒绝
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true

		file := data.PatchFile{PatchID: patchID, RelPath: rel, Op: "added"}
		if root != "" {
			if full, err := data.SafeJoin(root, rel); err == nil {
				if info, err := os.Stat(full); err == nil {
					file.Size = info.Size()
					file.IsDir = info.IsDir()
				}
			}
		}
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}

func validPatchKind(k string) bool {
	for _, v := range data.AllPatchKinds {
		if v == k {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 目录快照
// ---------------------------------------------------------------------------

// DetectChanges 对比当前目录与上次快照，返回新增/被修改的文件。
//
// 首次调用会静默建立基线并返回 HadBaseline=false —— 这是必要的：
// 没有基线就没法知道"装补丁之前"长什么样。
func (s *PatchService) DetectChanges(gameID uint) (DetectResult, error) {
	res := DetectResult{Changes: []data.FileChange{}}
	res.ScannedAt = time.Now().UnixMilli()

	var g data.Game
	if err := s.store.DB.First(&g, gameID).Error; err != nil {
		return res, fmt.Errorf("游戏不存在: %d", gameID)
	}
	if strings.TrimSpace(g.FolderPath) == "" {
		return res, fmt.Errorf("请先设置游戏的安装目录")
	}

	current, truncated, err := data.ScanFolder(g.FolderPath)
	if err != nil {
		return res, err
	}
	res.Truncated = truncated

	var base []data.Snapshot
	if err := s.store.DB.Where("game_id = ?", gameID).Find(&base).Error; err != nil {
		return res, fmt.Errorf("读取快照失败: %w", err)
	}

	if len(base) == 0 {
		// 没有基线：建立基线，本次不返回变化。
		if _, err := s.rebuildSnapshot(g); err != nil {
			return res, err
		}
		res.HadBaseline = false
		return res, nil
	}

	res.HadBaseline = true
	changes := data.DiffSnapshots(base, current)
	res.Total = len(changes)
	if len(changes) > maxChanges {
		changes = changes[:maxChanges]
	}
	res.Changes = changes
	return res, nil
}

// HasSnapshot 判断某个游戏是否已经有目录基线。
func (s *PatchService) HasSnapshot(gameID uint) (bool, error) {
	var n int64
	if err := s.store.DB.Model(&data.Snapshot{}).Where("game_id = ?", gameID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("查询快照失败: %w", err)
	}
	return n > 0, nil
}

// RebuildSnapshot 手动重建基线，返回记录的文件数。
//
// 典型用法：用户装完补丁、登记好记录之后，点"重置基线"让下次检测重新开始。
func (s *PatchService) RebuildSnapshot(gameID uint) (int, error) {
	var g data.Game
	if err := s.store.DB.First(&g, gameID).Error; err != nil {
		return 0, fmt.Errorf("游戏不存在: %d", gameID)
	}
	if strings.TrimSpace(g.FolderPath) == "" {
		return 0, fmt.Errorf("请先设置游戏的安装目录")
	}
	return s.rebuildSnapshot(g)
}

// rebuildSnapshot 清空旧快照并按当前目录重写。
func (s *PatchService) rebuildSnapshot(g data.Game) (int, error) {
	current, _, err := data.ScanFolder(g.FolderPath)
	if err != nil {
		return 0, err
	}
	err = s.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("game_id = ?", g.ID).Delete(&data.Snapshot{}).Error; err != nil {
			return err
		}
		if len(current) == 0 {
			return nil
		}
		for i := range current {
			current[i].GameID = g.ID
		}
		return tx.CreateInBatches(&current, 500).Error
	})
	if err != nil {
		return 0, fmt.Errorf("保存快照失败: %w", err)
	}
	return len(current), nil
}

// Browse 逐层浏览游戏目录，给补丁文件选择器用。
// 已经在某个补丁里的文件会被标上 InPatch。
func (s *PatchService) Browse(gameID uint, relDir string) ([]data.DirEntry, error) {
	var g data.Game
	if err := s.store.DB.First(&g, gameID).Error; err != nil {
		return nil, fmt.Errorf("游戏不存在: %d", gameID)
	}
	if strings.TrimSpace(g.FolderPath) == "" {
		return nil, fmt.Errorf("请先设置游戏的安装目录")
	}

	entries, err := data.Browse(g.FolderPath, relDir)
	if err != nil {
		return nil, err
	}

	// 一次查出该游戏所有补丁文件路径，标注哪些已被登记。
	var paths []string
	if err := s.store.DB.Model(&data.PatchFile{}).
		Joins("JOIN patches ON patches.id = patch_files.patch_id").
		Where("patches.game_id = ?", gameID).
		Pluck("patch_files.rel_path", &paths).Error; err != nil {
		// 标注失败不影响浏览，只是少了灰显提示。
		return entries, nil
	}
	known := make(map[string]bool, len(paths))
	for _, p := range paths {
		known[p] = true
	}
	for i := range entries {
		if known[entries[i].RelPath] {
			entries[i].InPatch = true
		}
	}
	return entries, nil
}

// emitChanged 广播补丁变更事件。
func (s *PatchService) emitChanged(gameID uint) {
	if app := application.Get(); app != nil {
		app.Event.Emit("patch:changed", int64(gameID))
	}
}
