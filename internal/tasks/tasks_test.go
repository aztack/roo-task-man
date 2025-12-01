package tasks

import (
    "os"
    "path/filepath"
    "testing"
)

func TestDiscoverAndBuildTasks(t *testing.T) {
    root := t.TempDir()
    // Create structure: <root>/tasks/t1/file.txt
    tdir := filepath.Join(root, "tasks", "t1")
    if err := os.MkdirAll(tdir, 0o755); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(tdir, "file.txt"), []byte("hi"), 0o644); err != nil { t.Fatal(err) }

    dirs := DiscoverTaskDirs(root)
    if len(dirs) != 1 {
        t.Fatalf("expected 1 task dir, got %d (%v)", len(dirs), dirs)
    }
    if filepath.Base(dirs[0]) != "t1" {
        t.Fatalf("expected dir base t1, got %s", filepath.Base(dirs[0]))
    }

    list := BuildTasksFromDirs(dirs)
    if len(list) != 1 { t.Fatalf("expected 1 task, got %d", len(list)) }
    if list[0].ID != "t1" { t.Fatalf("expected id t1, got %s", list[0].ID) }
}

