package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"roocode-task-man/internal/config"
)

type Task struct {
    ID        string
    Title     string
    Summary   string
    CreatedAt time.Time
    Path      string
    Meta      map[string]any
}

type HistoryItem struct {
    At   time.Time
    Kind string
    Text string
    Role string // user or ai
}

// ResolveStorageRoot returns the directory where the plugin's globalStorage resides.
func ResolveStorageRoot(cfg config.Config) (string, error) {
    if cfg.DataDir != "" {
        return cfg.DataDir, nil
    }
    base := ""
    switch runtime.GOOS {
    case "darwin":
        // macOS
        base = filepath.Join(config.UserHome(), "Library", "Application Support")
    case "linux":
        base = filepath.Join(config.UserHome(), ".config")
    case "windows":
        appdata := os.Getenv("APPDATA")
        if appdata == "" {
            return "", errors.New("APPDATA not set")
        }
        base = appdata
    default:
        return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
    }

    channel := cfg.CodeChannel
    if channel == "" { channel = "Code" }
    codeDir, isCustom := mapEditorChannel(channel)
    if isCustom {
        if cfg.DataDir == "" {
            return "", errors.New("custom editor requires dataDir override")
        }
        return cfg.DataDir, nil
    }

    // Global storage root
    userDir := filepath.Join(base, codeDir, "User", "globalStorage", cfg.PluginID)
    return userDir, nil
}

// LoadTasks discovers available tasks under the plugin storage.
func LoadTasks(cfg config.Config) ([]Task, error) {
    root, err := ResolveStorageRoot(cfg)
    if err != nil { return nil, err }
    dirs := DiscoverTaskDirs(root)
    tasks := BuildTasksFromDirs(dirs)
    return tasks, nil
}

// DiscoverTaskDirs returns likely task directories.
func DiscoverTaskDirs(root string) []string {
    candidates := []string{
        filepath.Join(root, "tasks"),
        root,
    }
    var taskDirs []string
    for _, c := range candidates {
        entries, err := os.ReadDir(c)
        if err != nil { continue }
        for _, e := range entries {
            if !e.IsDir() { continue }
            p := filepath.Join(c, e.Name())
            // Heuristic: directory with at least one file.
            hasFiles := false
            _ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
                if err != nil { return nil }
                if !d.IsDir() { hasFiles = true; return fs.SkipDir }
                return nil
            })
            if hasFiles { taskDirs = append(taskDirs, p) }
        }
        if len(taskDirs) > 0 { break }
    }
    return taskDirs
}

// BuildTasksFromDirs builds Task objects from directory paths.
func BuildTasksFromDirs(taskDirs []string) []Task {
    tasks := make([]Task, 0, len(taskDirs))
    for _, d := range taskDirs {
        id := filepath.Base(d)
        summary := readSummary(d)
        title := summary
        if title == "" { title = id }
        created := dirCreatedAt(d)
        tasks = append(tasks, Task{
            ID:        id,
            Title:     title,
            Summary:   summary,
            CreatedAt: created,
            Path:      d,
            Meta:      map[string]any{},
        })
    }
    sort.Slice(tasks, func(i, j int) bool { return tasks[i].CreatedAt.After(tasks[j].CreatedAt) })
    return tasks
}

func readSummary(dir string) string {
    // Attempt to read <dir>/ui_messages.json and extract first object's "text" field
    p := filepath.Join(dir, "ui_messages.json")
    b, err := os.ReadFile(p)
    if err != nil { return "" }
    // Minimal JSON scan to avoid importing encoding/json if we can? We already use it in zipper, so use it.
    type msg struct{ Text string `json:"text"` }
    var arr []msg
    if err := json.Unmarshal(b, &arr); err != nil || len(arr) == 0 { return "" }
    return arr[0].Text
}

// LoadHistory parses ui_messages.json within the task directory and returns a list of history items.
func LoadHistory(t Task) []HistoryItem {
    p := filepath.Join(t.Path, "ui_messages.json")
    b, err := os.ReadFile(p)
    if err != nil { return nil }
    // support array of objects with fields: ts, type, say, text
    type raw struct {
        Ts   int64   `json:"ts"`
        Type string  `json:"type"`
        Say  string  `json:"say"`
        Text string  `json:"text"`
        Images any   `json:"images"`
    }
    var arr []raw
    if err := json.Unmarshal(b, &arr); err != nil { return nil }
    out := make([]HistoryItem, 0, len(arr))
    for _, r := range arr {
        if r.Ts > 0 { /* ok */ }
        // If user message: has images field (array even if empty)
        if r.Images != nil {
            it := HistoryItem{Role: "user", Kind: "User", Text: r.Text}
            if r.Ts > 0 { it.At = time.UnixMilli(r.Ts) }
            out = append(out, it)
            continue
        }
        // Try to parse AI request JSON in r.Text
        var ai struct {
            APIProtocol string  `json:"apiProtocol"`
            Costs       float64 `json:"costs"`
            Request     string  `json:"request"`
            Mode        string  `json:"mode"`
            TokenIn     int     `json:"tokenIn"`
            TokenOut    int     `json:"tokenOut"`
            CacheReads  int     `json:"cacheReads"`
            CacheWrites int     `json:"cacheWrites"`
        }
        var it HistoryItem
        if json.Unmarshal([]byte(r.Text), &ai) == nil && ai.Request != "" {
            it.Role = "ai"
            it.Kind = "AI Request"
            // Build markdown block with request and stats
            sb := strings.Builder{}
            if ai.Request != "" { sb.WriteString(ai.Request); sb.WriteString("\n\n") }
            sb.WriteString("**Stats**\\n")
            fmt.Fprintf(&sb, "- Protocol: %s\\n", ai.APIProtocol)
            fmt.Fprintf(&sb, "- Cost: $%.4f\\n", ai.Costs)
            fmt.Fprintf(&sb, "- Tokens: in %d / out %d\\n", ai.TokenIn, ai.TokenOut)
            fmt.Fprintf(&sb, "- Mode: %s\\n", ai.Mode)
            fmt.Fprintf(&sb, "- Cache: reads %d / writes %d\\n", ai.CacheReads, ai.CacheWrites)
            it.Text = sb.String()
        } else {
            // Fallback: include as-is if text present
            it.Role = "other"
            it.Kind = r.Say
            if it.Kind == "" { it.Kind = r.Type }
            it.Text = r.Text
        }
        if r.Ts > 0 { it.At = time.UnixMilli(r.Ts) }
        out = append(out, it)
    }
    return out
}

func dirCreatedAt(path string) time.Time {
    // Best effort: earliest mtime within the dir; fallback to dir stat mtime
    var t time.Time
    _ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
        if err != nil { return nil }
        if d.IsDir() { return nil }
        if info, err := d.Info(); err == nil {
            mt := info.ModTime()
            if t.IsZero() || mt.Before(t) { t = mt }
        }
        return nil
    })
    if t.IsZero() {
        if info, err := os.Stat(path); err == nil {
            return info.ModTime()
        }
    }
    return t
}

// mapEditorChannel normalizes known VS Code forks to their application data folder name.
// Returns (codeDir, isCustom). For unknown strings, returns the input as-is with isCustom=false.
func mapEditorChannel(channel string) (string, bool) {
    s := strings.TrimSpace(channel)
    if s == "" { return "Code", false }
    norm := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(s, "_", "-"), " ", "-"))
    switch norm {
    case "code", "stable":
        return "Code", false
    case "insiders", "code-insiders", "code---insiders":
        return "Code - Insiders", false
    case "vscodium", "codium":
        return "VSCodium", false
    case "cursor":
        return "Cursor", false
    case "windsurf":
        return "Windsurf", false
    case "trae":
        return "Trae", false
    case "custom":
        return "", true
    default:
        // Assume caller provided exact app dir name
        return s, false
    }
}

// DisplayEditorName returns a friendly editor/app folder name used for display and path resolution.
func DisplayEditorName(channel string) string {
    if channel == "" { return "Code" }
    if name, custom := mapEditorChannel(channel); !custom && name != "" {
        return name
    }
    return channel
}

// DeleteTask removes the task directory recursively.
func DeleteTask(t Task) error {
    return os.RemoveAll(t.Path)
}
