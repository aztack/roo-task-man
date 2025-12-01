package zipper

import (
    "archive/zip"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "time"

    "roocode-task-man/internal/tasks"
)

// ProgressCallback is called during export with current progress (current, total)
type ProgressCallback func(current, total int)

type Manifest struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    CreatedAt time.Time `json:"createdAt"`
    PluginID  string    `json:"pluginId"`
}

type ManifestMulti struct {
    Version int        `json:"version"`
    Tasks   []Manifest `json:"tasks"`
}

func ExportTask(t tasks.Task, zipPath string) error {
    if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
        return err
    }
    f, err := os.Create(zipPath)
    if err != nil {
        return err
    }
    defer f.Close()

    zw := zip.NewWriter(f)
    defer zw.Close()

    // Write manifest
    manifest := Manifest{ID: t.ID, Title: t.Title, CreatedAt: t.CreatedAt}
    if err := writeJSON(zw, "roo-task-manifest.json", manifest); err != nil {
        return err
    }
    // Walk files
    base := filepath.Dir(t.Path)
    err = filepath.Walk(t.Path, func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        if info.IsDir() { return nil }
        rel, _ := filepath.Rel(base, path)
        return addFile(zw, path, rel)
    })
    return err
}

// ExportTasks writes multiple tasks into a single zip with a v2 manifest.
// Progress callback is optional and called with (current, total) file count.
func ExportTasks(ts []tasks.Task, zipPath string) error {
    return ExportTasksWithProgress(ts, zipPath, nil)
}

// ExportTasksWithProgress writes multiple tasks into a single zip with progress reporting.
func ExportTasksWithProgress(ts []tasks.Task, zipPath string, progress ProgressCallback) error {
    if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil { return err }
    f, err := os.Create(zipPath)
    if err != nil { return err }
    defer f.Close()
    zw := zip.NewWriter(f)
    defer zw.Close()

    mm := ManifestMulti{Version: 2}
    for _, t := range ts {
        mm.Tasks = append(mm.Tasks, Manifest{ID: t.ID, Title: t.Title, CreatedAt: t.CreatedAt})
    }
    if err := writeJSON(zw, "roo-task-manifest.json", mm); err != nil { return err }

    // Count total files first for progress reporting
    totalFiles := 0
    for _, t := range ts {
        _ = filepath.Walk(t.Path, func(path string, info os.FileInfo, err error) error {
            if err != nil { return nil }
            if !info.IsDir() { totalFiles++ }
            return nil
        })
    }

    currentFiles := 0
    // Include files for each task under <id>/...
    for _, t := range ts {
        err := filepath.Walk(t.Path, func(path string, info os.FileInfo, err error) error {
            if err != nil { return err }
            if info.IsDir() { return nil }
            currentFiles++
            if progress != nil { progress(currentFiles, totalFiles) }
            rel, _ := filepath.Rel(t.Path, path)
            return addFile(zw, path, filepath.Join(t.ID, rel))
        })
        if err != nil { return err }
    }
    if progress != nil { progress(totalFiles, totalFiles) }
    return nil
}

// ImportAny imports either a single-task or multi-task archive.
func ImportAny(zipPath, destRoot string) error {
    r, err := zip.OpenReader(zipPath)
    if err != nil { return err }
    defer r.Close()

    // Try multi manifest first
    var multi ManifestMulti
    var single Manifest
    var hasManifest bool
    for _, f := range r.File {
        if strings.EqualFold(filepath.Base(f.Name), "roo-task-manifest.json") {
            hasManifest = true
            rc, err := f.Open(); if err != nil { return err }
            b, err := io.ReadAll(rc); rc.Close(); if err != nil { return err }
            if err := json.Unmarshal(b, &multi); err == nil && multi.Version >= 2 && len(multi.Tasks) > 0 { break }
            // Fallback to single
            if err := json.Unmarshal(b, &single); err != nil { return fmt.Errorf("invalid manifest in %s", zipPath) }
            break
        }
    }
    if !hasManifest { return fmt.Errorf("manifest missing in %s", zipPath) }

    if len(multi.Tasks) > 0 {
        // Build set of IDs
        ids := map[string]struct{}{}
        for _, m := range multi.Tasks { ids[m.ID] = struct{}{} }
        // Extract files
        for _, f := range r.File {
            if f.FileInfo().IsDir() { continue }
            if strings.EqualFold(filepath.Base(f.Name), "roo-task-manifest.json") { continue }
            rel := strings.TrimLeft(f.Name, "/\\")
            segs := strings.Split(rel, "/")
            if len(segs) == 0 { continue }
            id := segs[0]
            if _, ok := ids[id]; !ok { continue }
            // Destination path: prefer destRoot/tasks/<id>/...
            base := filepath.Join(destRoot, "tasks", id)
            if _, err := os.Stat(filepath.Dir(base)); err != nil {
                base = filepath.Join(destRoot, id)
            }
            outPath := filepath.Join(base, filepath.Join(segs[1:]...))
            if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil { return err }
            if err := extractFile(f, outPath); err != nil { return err }
        }
        return nil
    }

    // Single task fallback
    return ImportTask(zipPath, destRoot)
}

func ImportTask(zipPath, destRoot string) error {
    r, err := zip.OpenReader(zipPath)
    if err != nil {
        return err
    }
    defer r.Close()

    var manifest Manifest
    // First pass: read manifest
    for _, f := range r.File {
        if strings.EqualFold(filepath.Base(f.Name), "roo-task-manifest.json") {
            rc, err := f.Open()
            if err != nil { return err }
            b, err := io.ReadAll(rc)
            rc.Close()
            if err != nil { return err }
            if err := json.Unmarshal(b, &manifest); err != nil {
                return err
            }
            break
        }
    }
    if manifest.ID == "" {
        return fmt.Errorf("manifest missing or invalid in %s", zipPath)
    }

    // Destination path: prefer <destRoot>/tasks/<id> if exists; else <destRoot>/<id>
    dst := filepath.Join(destRoot, "tasks", manifest.ID)
    if _, err := os.Stat(filepath.Dir(dst)); err != nil {
        dst = filepath.Join(destRoot, manifest.ID)
    }
    if _, err := os.Stat(dst); err == nil {
        // Collision policy: create -copy with timestamp suffix
        dst = dst + "-copy-" + time.Now().Format("20060102-150405")
    }
    if err := os.MkdirAll(dst, 0o755); err != nil { return err }

    // Extract all non-manifest files under dst
    for _, f := range r.File {
        if strings.EqualFold(filepath.Base(f.Name), "roo-task-manifest.json") {
            continue
        }
        if f.FileInfo().IsDir() { continue }
        // Recreate the relative path under the task directory name
        // We expect paths like <id>/sub/dir/file; if not, place under dst preserving subpath after id
        rel := f.Name
        // Strip leading slashes
        rel = strings.TrimLeft(rel, "/\\")
        // If the first segment equals manifest.ID, drop it
        segs := strings.Split(rel, "/")
        if len(segs) > 0 && segs[0] == manifest.ID {
            rel = strings.Join(segs[1:], "/")
        }
        outPath := filepath.Join(dst, rel)
        if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil { return err }
        if err := extractFile(f, outPath); err != nil { return err }
    }
    return nil
}

func writeJSON(zw *zip.Writer, name string, v any) error {
    w, err := zw.Create(name)
    if err != nil { return err }
    b, err := json.MarshalIndent(v, "", "  ")
    if err != nil { return err }
    _, err = w.Write(b)
    return err
}

func addFile(zw *zip.Writer, diskPath, zipRel string) error {
    f, err := os.Open(diskPath)
    if err != nil { return err }
    defer f.Close()
    w, err := zw.Create(zipRel)
    if err != nil { return err }
    _, err = io.Copy(w, f)
    return err
}

func extractFile(f *zip.File, out string) error {
    rc, err := f.Open()
    if err != nil { return err }
    defer rc.Close()
    of, err := os.Create(out)
    if err != nil { return err }
    defer of.Close()
    _, err = io.Copy(of, rc)
    return err
}
