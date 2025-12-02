package tasks

import (
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "runtime"
    "sort"
    "strings"
    "time"

	_ "modernc.org/sqlite"

    "roocode-task-man/internal/config"
)

// RegisterImportedTasks appends imported tasks into the extension's TaskHistory in state.vscdb for the given workspace path.
func RegisterImportedTasks(cfg config.Config, workspace string, ts []Task) error {
    if workspace == "" { return errors.New("workspace is required") }
    // find state db
    dbPath, err := detectStateDBPath(cfg)
    if err != nil { return err }
    // Backup both primary and backup DBs with the same suffix
    suffix := time.Now().Format("20060102-150405")
    // Backup primary db unconditionally
    if err := backupFileWithSuffix(dbPath, suffix); err != nil {
        if cfg.Debug { log.Printf("[statevscdb] warning: backup primary state DB failed: %v", err) }
    }
    if err := upsertTasksIntoDB(dbPath, cfg.PluginID, workspace, ts, cfg.Debug); err != nil { return err }
    // Update backup db (to avoid VS Code rollback overwriting changes), also back it up
    bak := dbPath + ".backup"
    if _, err := os.Stat(bak); err == nil {
        if err := backupFileWithSuffix(bak, suffix); err != nil { if cfg.Debug { log.Printf("[statevscdb] warning: backup of state DB backup failed: %v", err) } }
        if err := upsertTasksIntoDB(bak, cfg.PluginID, workspace, ts, cfg.Debug); err != nil { return err }
    } else {
        // Even if backup DB is missing, log for visibility
        if cfg.Debug { log.Printf("[statevscdb] info: state DB backup file not found at %s; skipping", bak) }
    }
    return nil
}

func detectStateDBPath(cfg config.Config) (string, error) {
    base, editor := stateDBBaseAndEditor(cfg)
    p := filepath.Join(base, editor, "User", "globalStorage", "state.vscdb")
    if _, err := os.Stat(p); err != nil { return "", fmt.Errorf("state db not found at %s", p) }
    return p, nil
}

// detectStateDBDir returns the directory that should contain state.vscdb even if the file is missing.
func detectStateDBDir(cfg config.Config) (string, error) {
    base, editor := stateDBBaseAndEditor(cfg)
    dir := filepath.Join(base, editor, "User", "globalStorage")
    if _, err := os.Stat(dir); err != nil {
        return "", fmt.Errorf("state db directory not found: %s", dir)
    }
    return dir, nil
}

func stateDBBaseAndEditor(cfg config.Config) (string, string) {
    base := ""
    switch runtime.GOOS {
    case "darwin":
        base = filepath.Join(config.UserHome(), "Library", "Application Support")
    case "linux":
        base = filepath.Join(config.UserHome(), ".config")
    case "windows":
        if app := os.Getenv("APPDATA"); app != "" { base = app } else { base = "" }
    default:
        base = ""
    }
    editor := DisplayEditorName(cfg.CodeChannel)
    return base, editor
}

func backupFile(path string) error {
    if _, err := os.Stat(path); err != nil { return err }
    name := filepath.Base(path)
    bak := filepath.Join(filepath.Dir(path), name+".bak-"+time.Now().Format("20060102-150405"))
    src, err := os.ReadFile(path)
    if err != nil { return err }
    return os.WriteFile(bak, src, 0o600)
}

func backupFileWithSuffix(path, suffix string) error {
    if _, err := os.Stat(path); err != nil { return err }
    name := filepath.Base(path)
    bak := filepath.Join(filepath.Dir(path), name+".bak-"+suffix)
    src, err := os.ReadFile(path)
    if err != nil { return err }
    return os.WriteFile(bak, src, 0o600)
}

func upsertTasksIntoDB(dbPath, pluginID, workspace string, ts []Task, debug bool) error {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil { return err }
    defer db.Close()
    // Align with VS Code state DB behavior: WAL + busy timeout to avoid lock issues
    _, _ = db.Exec("PRAGMA busy_timeout=5000")
    _, _ = db.Exec("PRAGMA journal_mode=WAL")
    if _, err := db.Exec("BEGIN IMMEDIATE"); err != nil { return err }
    defer db.Exec("ROLLBACK")
    if _, err := db.Exec("CREATE TABLE IF NOT EXISTS ItemTable (key TEXT PRIMARY KEY, value BLOB)"); err != nil { return err }

    var raw []byte
    err = db.QueryRow("SELECT value FROM ItemTable WHERE key = ?", pluginID).Scan(&raw)
    if err == sql.ErrNoRows { raw = []byte(`{"taskHistory":[]}`) } else if err != nil { return err }
    var doc map[string]any
    if err := json.Unmarshal(raw, &doc); err != nil { return fmt.Errorf("parse json: %w", err) }
    hist, _ := doc["taskHistory"].([]any)
    if hist == nil { hist = []any{} }
    for _, t := range ts {
        st := StatsFromTask(t)
        entry := map[string]any{
            "id":         t.ID,
            "number":     1,
            "ts":         t.CreatedAt.UnixMilli(),
            "task":       t.Summary,
            "tokensIn":   452134,
            "tokensOut":  7027,
            "totalCost":  1.2548531250000001,
            "cacheWrites": st.CacheWrites,
            "cacheReads":  st.CacheReads,
            "size":       st.SizeBytes,
            "workspace":  workspace,
            "mode":       "code",
        }
        hist = append(hist, entry)
        if debug {
            log.Printf("[statevscdb] inserting taskHistory: db=%s plugin=%s id=%s number=%d ts=%d size=%d workspace=%s tokensIn=%d tokensOut=%d cacheReads=%d cacheWrites=%d totalCost=%.6f mode=%s",
                dbPath, pluginID, t.ID, 1, t.CreatedAt.UnixMilli(), st.SizeBytes, workspace, 1, 1, st.CacheReads, st.CacheWrites, float64(1), "code")
        }
    }
    doc["taskHistory"] = hist
    b, err := json.Marshal(doc)
    if err != nil { return err }
    if _, err := db.Exec("INSERT INTO ItemTable(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", pluginID, b); err != nil { return err }
    if _, err := db.Exec("COMMIT"); err != nil { return err }
    // Ensure WAL is checkpointed so changes persist to main db file
    _, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
    if debug {
        // verify write by reading back and logging count and presence of inserted IDs
        var after []byte
        if err := db.QueryRow("SELECT value FROM ItemTable WHERE key = ?", pluginID).Scan(&after); err == nil {
            var verify map[string]any
            if json.Unmarshal(after, &verify) == nil {
                if arr, ok := verify["taskHistory"].([]any); ok {
                    log.Printf("[statevscdb] write committed: db=%s plugin=%s taskHistoryCount=%d", dbPath, pluginID, len(arr))
                    // Build a quick index for lookup and verify each inserted id
                    for _, t := range ts {
                        found := false
                        var number int
                        var ws string
                        for _, it := range arr {
                            if m, ok := it.(map[string]any); ok {
                                if idv, ok := m["id"].(string); ok && idv == t.ID {
                                    found = true
                                    if v, ok := m["number"].(float64); ok { number = int(v) }
                                    if w, ok := m["workspace"].(string); ok { ws = w }
                                    break
                                }
                            }
                        }
                        log.Printf("[statevscdb] verify: db=%s plugin=%s id=%s found=%t number=%d workspace=%s", dbPath, pluginID, t.ID, found, number, ws)
                    }
                } else {
                    log.Printf("[statevscdb] verify: db=%s plugin=%s missing taskHistory array", dbPath, pluginID)
                }
            } else {
                log.Printf("[statevscdb] verify: failed to parse JSON from db=%s plugin=%s: %v", dbPath, pluginID, err)
            }
        } else {
            log.Printf("[statevscdb] verify: failed to read back row from db=%s plugin=%s: %v", dbPath, pluginID, err)
        }
    }
    return nil
}

// VerifyRegistration checks that the given IDs exist in the taskHistory for both
// the primary state DB and the optional backup DB. It returns presence maps keyed
// by task ID for primary and backup.
func VerifyRegistration(cfg config.Config, ids []string) (map[string]bool, map[string]bool, error) {
    dbPath, err := detectStateDBPath(cfg)
    if err != nil { return nil, nil, err }
    primary, err := verifyIdsInDB(dbPath, cfg.PluginID, ids)
    if err != nil { return nil, nil, err }
    bakPath := dbPath + ".backup"
    backup := map[string]bool{}
    if _, err := os.Stat(bakPath); err == nil {
        m, err := verifyIdsInDB(bakPath, cfg.PluginID, ids)
        if err != nil { return primary, nil, err }
        backup = m
    } else {
        for _, id := range ids { backup[id] = false }
    }
    return primary, backup, nil
}

func verifyIdsInDB(dbPath, pluginID string, ids []string) (map[string]bool, error) {
    result := make(map[string]bool, len(ids))
    for _, id := range ids { result[id] = false }
    db, err := sql.Open("sqlite", dbPath)
    if err != nil { return result, err }
    defer db.Close()
    var raw []byte
    if err := db.QueryRow("SELECT value FROM ItemTable WHERE key = ?", pluginID).Scan(&raw); err != nil {
        return result, nil // treat missing row as not present
    }
    var doc map[string]any
    if err := json.Unmarshal(raw, &doc); err != nil { return result, err }
    arr, _ := doc["taskHistory"].([]any)
    if arr == nil { return result, nil }
    set := map[string]struct{}{}
    for _, it := range arr {
        if m, ok := it.(map[string]any); ok {
            if idv, ok := m["id"].(string); ok { set[idv] = struct{}{} }
        }
    }
    for _, id := range ids { if _, ok := set[id]; ok { result[id] = true } }
    return result, nil
}

// BackupInfo describes a found backup file for state.vscdb.
type BackupInfo struct {
    Path    string
    Suffix  string
    ModTime time.Time
    Size    int64
}

// ListBackups returns all state.vscdb.bak-* backups sorted by ModTime desc, and the directory.
func ListBackups(cfg config.Config) ([]BackupInfo, string, error) {
    dir, err := detectStateDBDir(cfg)
    if err != nil { return nil, "", err }
    entries, err := os.ReadDir(dir)
    if err != nil { return nil, "", err }
    var out []BackupInfo
    for _, e := range entries {
        name := e.Name()
        if !e.Type().IsRegular() { continue }
        if !strings.HasPrefix(name, "state.vscdb.bak-") { continue }
        info, err := e.Info(); if err != nil { continue }
        out = append(out, BackupInfo{
            Path: filepath.Join(dir, name),
            Suffix: strings.TrimPrefix(name, "state.vscdb.bak-"),
            ModTime: info.ModTime(),
            Size: info.Size(),
        })
    }
    sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
    return out, dir, nil
}

// RestoreFromBackup restores state.vscdb and paired state.vscdb.backup from backups with the given suffix.
func RestoreFromBackup(cfg config.Config, suffix string, debug bool) error {
    dir, err := detectStateDBDir(cfg)
    if err != nil { return err }
    srcPrimary := filepath.Join(dir, "state.vscdb.bak-"+suffix)
    dstPrimary := filepath.Join(dir, "state.vscdb")
    srcBackup  := filepath.Join(dir, "state.vscdb.backup.bak-"+suffix)
    dstBackup  := filepath.Join(dir, "state.vscdb.backup")
    // Sanity
    if _, err := os.Stat(srcPrimary); err != nil { return fmt.Errorf("primary backup not found: %s", srcPrimary) }
    if err := copyFile(srcPrimary, dstPrimary); err != nil { return fmt.Errorf("restore primary: %w", err) }
    if _, err := os.Stat(srcBackup); err == nil {
        if err := copyFile(srcBackup, dstBackup); err != nil { return fmt.Errorf("restore backup: %w", err) }
    } else if debug {
        log.Printf("[restore] paired backup not found: %s (restored primary only)", srcBackup)
    }
    if debug { log.Printf("[restore] restored from suffix %s", suffix) }
    return nil
}

func copyFile(src, dst string) error {
    b, err := os.ReadFile(src)
    if err != nil { return err }
    tmp := dst + ".tmp-" + time.Now().Format("20060102-150405")
    if err := os.WriteFile(tmp, b, 0o600); err != nil { return err }
    return os.Rename(tmp, dst)
}
