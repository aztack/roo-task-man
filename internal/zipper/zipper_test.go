package zipper

import (
    "archive/zip"
    "os"
    "path/filepath"
    "testing"
    "time"

    "roocode-task-man/internal/tasks"
)

func TestExportImport(t *testing.T) {
    // Make a fake task
    root := t.TempDir()
    tdir := filepath.Join(root, "t123")
    if err := os.MkdirAll(filepath.Join(tdir, "sub"), 0o755); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(tdir, "sub", "file.txt"), []byte("hello"), 0o644); err != nil { t.Fatal(err) }
    tk := tasks.Task{ID: "t123", Title: "T123", CreatedAt: time.Now(), Path: tdir}

    // Export
    zipPath := filepath.Join(root, "out.zip")
    if err := ExportTask(tk, zipPath); err != nil { t.Fatalf("export: %v", err) }
    if _, err := os.Stat(zipPath); err != nil { t.Fatalf("zip not found: %v", err) }

    // Sanity check zip has manifest
    zr, err := zip.OpenReader(zipPath)
    if err != nil { t.Fatal(err) }
    hasManifest := false
    for _, f := range zr.File { if filepath.Base(f.Name) == "roo-task-manifest.json" { hasManifest = true; break } }
    zr.Close()
    if !hasManifest { t.Fatal("manifest missing in zip") }

    // Import
    destRoot := filepath.Join(root, "dest")
    if err := os.MkdirAll(destRoot, 0o755); err != nil { t.Fatal(err) }
    if err := ImportTask(zipPath, destRoot); err != nil { t.Fatalf("import: %v", err) }
    // Expect destRoot/tasks/t123 or destRoot/t123
    out1 := filepath.Join(destRoot, "tasks", "t123", "sub", "file.txt")
    out2 := filepath.Join(destRoot, "t123", "sub", "file.txt")
    if _, err1 := os.Stat(out1); err1 != nil {
        if _, err2 := os.Stat(out2); err2 != nil {
            t.Fatalf("imported file not found in expected locations: %v | %v", err1, err2)
        }
    }
}

func TestExportImportMulti(t *testing.T) {
    root := t.TempDir()
    // Two fake tasks
    mk := func(id string) tasks.Task {
        tdir := filepath.Join(root, id)
        if err := os.MkdirAll(filepath.Join(tdir, "sub"), 0o755); err != nil { t.Fatal(err) }
        if err := os.WriteFile(filepath.Join(tdir, "sub", id+".txt"), []byte(id), 0o644); err != nil { t.Fatal(err) }
        return tasks.Task{ID: id, Title: id, CreatedAt: time.Now(), Path: tdir}
    }
    t1 := mk("t1")
    t2 := mk("t2")
    zipPath := filepath.Join(root, "out-multi.zip")
    if err := ExportTasks([]tasks.Task{t1, t2}, zipPath); err != nil { t.Fatalf("export multi: %v", err) }
    dest := filepath.Join(root, "dest")
    if err := os.MkdirAll(dest, 0o755); err != nil { t.Fatal(err) }
    if err := ImportAny(zipPath, dest); err != nil { t.Fatalf("import any: %v", err) }
    // Validate presence
    _, e1 := os.Stat(filepath.Join(dest, "tasks", "t1", "sub", "t1.txt"))
    _, e2 := os.Stat(filepath.Join(dest, "tasks", "t2", "sub", "t2.txt"))
    if e1 != nil && e2 != nil {
        // Fallback structure without /tasks
        if _, ee1 := os.Stat(filepath.Join(dest, "t1", "sub", "t1.txt")); ee1 != nil ||
            func() error { _, err := os.Stat(filepath.Join(dest, "t2", "sub", "t2.txt")); return err }() != nil {
            t.Fatalf("imported files not found: %v, %v", e1, e2)
        }
    }
}
