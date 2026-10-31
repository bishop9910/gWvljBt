package data

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxScanFiles 是单次目录扫描的文件上限。
//
// galgame 目录一般几千个文件，但有人会把整个磁盘根目录当安装目录填进来。
// 与其卡死，不如扫到上限就停，并在结果里标记 truncated 让界面提示用户。
const MaxScanFiles = 60000

// FileChange 描述补丁安装后目录里一个文件的变化。
type FileChange struct {
	RelPath string `json:"relPath"`
	Op      string `json:"op"` // added | modified | deleted
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

// DirEntry 是补丁文件选择器里的一行。
type DirEntry struct {
	RelPath string `json:"relPath"`
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
	// InPatch 表示这个文件已经被某条补丁记录收进去了，界面上标灰。
	InPatch bool `json:"inPatch"`
}

// FolderStats 是游戏目录的概览，编辑页选择目录后立刻能看一眼。
type FolderStats struct {
	Exists   bool   `json:"exists"`
	Files    int    `json:"files"`
	Size     int64  `json:"size"`
	Truncated bool  `json:"truncated"`
}

// SafeRel 把用户/前端给的相对路径规范化，并拒绝跳出 root 的输入。
//
// 补丁删除会真的删磁盘文件，所以这一步必须严格：任何 ".." 都直接拒绝。
func SafeRel(rel string) (string, error) {
	rel = strings.ReplaceAll(strings.TrimSpace(rel), "\\", "/")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		return "", fmt.Errorf("路径为空")
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法路径: %s", rel)
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("非法绝对路径: %s", rel)
	}
	return filepath.ToSlash(clean), nil
}

// SafeJoin 在 root 下安全拼出 rel 对应的绝对路径。
func SafeJoin(root, rel string) (string, error) {
	clean, err := SafeRel(rel)
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, filepath.FromSlash(clean))
	// 双重保险：Join 之后再确认结果仍在 root 里面。
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if absFull != absRoot && !strings.HasPrefix(absFull, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("路径越界: %s", rel)
	}
	return absFull, nil
}

// ScanFolder 递归列出 root 下所有常规文件，生成快照条目。
//
// 只收文件不收目录：目录本身对"补丁覆盖了哪些文件"没有意义，空目录的清理
// 交给删除补丁时的后处理。
func ScanFolder(root string) ([]Snapshot, bool, error) {
	if root == "" {
		return nil, false, fmt.Errorf("游戏目录为空")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, false, fmt.Errorf("无法访问目录: %w", err)
	}
	if !info.IsDir() {
		return nil, false, fmt.Errorf("不是目录: %s", root)
	}

	var out []Snapshot
	truncated := false

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// 单个文件没权限读不该让整次扫描失败，跳过继续。
			if d != nil && d.IsDir() && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			// 跳过明显的噪音目录，省时间也避免把缓存文件当成补丁。
			switch strings.ToLower(d.Name()) {
			case ".git", "node_modules", "__pycache__":
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // 符号链接、设备文件等一律忽略
		}
		if len(out) >= MaxScanFiles {
			truncated = true
			return fs.SkipAll
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, Snapshot{
			RelPath: filepath.ToSlash(rel),
			Size:    fi.Size(),
			ModTime: fi.ModTime().UnixNano(),
			IsDir:   false,
		})
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("扫描目录失败: %w", err)
	}
	return out, truncated, nil
}

// DiffSnapshots 比较新旧两份快照，产出"新增 + 被修改"的文件列表。
//
// 注意这里故意不报"被删除"：装补丁通常是覆盖/新增文件，误报删除会让用户把
// 自己删掉的文件也写进补丁清单，卸载补丁时就删不到东西。
func DiffSnapshots(base []Snapshot, current []Snapshot) []FileChange {
	old := make(map[string]Snapshot, len(base))
	for _, s := range base {
		old[s.RelPath] = s
	}

	var changes []FileChange
	for _, c := range current {
		prev, ok := old[c.RelPath]
		if !ok {
			changes = append(changes, FileChange{RelPath: c.RelPath, Op: "added", Size: c.Size, ModTime: c.ModTime})
			continue
		}
		// 大小变了必然变；大小没变但 mtime 也变了，仍然可能是同尺寸替换，
		// 一并算作 modified 让用户自己勾。
		if prev.Size != c.Size || prev.ModTime != c.ModTime {
			changes = append(changes, FileChange{RelPath: c.RelPath, Op: "modified", Size: c.Size, ModTime: c.ModTime})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].RelPath < changes[j].RelPath })
	return changes
}

// Browse 列出一层目录内容，给补丁文件选择器做逐层浏览。
func Browse(root, relDir string) ([]DirEntry, error) {
	if root == "" {
		return nil, fmt.Errorf("游戏目录为空")
	}
	relDir = strings.TrimSpace(strings.ReplaceAll(relDir, "\\", "/"))
	relDir = strings.Trim(relDir, "/")

	dir := root
	if relDir != "" {
		full, err := SafeJoin(root, relDir)
		if err != nil {
			return nil, err
		}
		dir = full
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && !e.Type().IsRegular() {
			continue
		}
		rel := e.Name()
		if relDir != "" {
			rel = relDir + "/" + e.Name()
		}
		entry := DirEntry{RelPath: rel, Name: e.Name(), IsDir: e.IsDir()}
		if fi, err := e.Info(); err == nil {
			entry.Size = fi.Size()
			entry.ModTime = fi.ModTime().UnixNano()
		}
		out = append(out, entry)
	}
	// 目录在前，文件在后，各自按名字排。
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// StatsFolder 快速统计目录里的文件数与总体积。
func StatsFolder(root string) FolderStats {
	var st FolderStats
	if root == "" {
		return st
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return st
	}
	st.Exists = true

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if st.Files >= MaxScanFiles {
			st.Truncated = true
			return fs.SkipAll
		}
		st.Files++
		if fi, err := d.Info(); err == nil {
			st.Size += fi.Size()
		}
		return nil
	})
	return st
}
