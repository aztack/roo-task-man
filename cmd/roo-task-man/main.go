package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"roocode-task-man/internal/config"
	"roocode-task-man/internal/tasks"
	"roocode-task-man/internal/tui"
	"roocode-task-man/internal/version"
	"roocode-task-man/internal/zipper"
)

func main() {
    // Flags
    var (
        cfgPath   string
        pluginID  string
        codeChan  string
        editor    string
        dataDir   string
        hooksDir  string
        exportDir string
        exportArg string // format: <task-id>:<zip-path>
        importArg string // zip-path
        debug     bool
        inspectZip string
        showVersion bool
    )

    flag.StringVar(&cfgPath, "config", filepath.Join(config.UserHome(), ".config", "roo-code-man.json"), "config file path")
    flag.StringVar(&pluginID, "plugin-id", "", "VS Code extension plugin ID (overrides config)")
    flag.StringVar(&codeChan, "code-channel", "", "Editor channel/name: Code | Insiders | VSCodium | Cursor | Windsurf | Trae | Custom | <AppDir>")
    flag.StringVar(&editor, "editor", "", "Alias of --code-channel")
    flag.StringVar(&dataDir, "data-dir", "", "override VS Code globalStorage root directory")
    flag.StringVar(&hooksDir, "hooks-dir", "", "directory containing JS hook files")
    flag.StringVar(&exportDir, "export-dir", "", "default export directory for TUI exports")
    flag.StringVar(&exportArg, "export", "", "batch export: <task-id>:<zip-path>")
    flag.StringVar(&importArg, "import", "", "batch import: <zip-path>")
    flag.StringVar(&inspectZip, "inspect", "", "inspect tasks from a zip (open TUI on extracted content)")
    flag.BoolVar(&debug, "debug", false, "print debug info (paths, counts)")
    flag.BoolVar(&showVersion, "version", false, "print version and exit")
    flag.Parse()

    if showVersion {
        fmt.Println(version.String())
        return
    }

    // Load config
    cfg := config.Default()
    if err := config.Load(cfgPath, &cfg); err != nil && !os.IsNotExist(err) {
        log.Printf("warning: failed to load config: %v", err)
    }
    // Merge overrides
    if pluginID != "" {
        cfg.PluginID = pluginID
    }
    if codeChan != "" {
        cfg.CodeChannel = codeChan
    }
    if editor != "" { // alias honors last-specified
        cfg.CodeChannel = editor
    }
    if dataDir != "" {
        cfg.DataDir = dataDir
    }
    if hooksDir != "" {
        cfg.HooksDir = hooksDir
    }
    if exportDir != "" {
        cfg.ExportDir = exportDir
    }
    if debug {
        cfg.Debug = true
    }

    // Batch operations
    if exportArg != "" {
        id, zipPath, err := parseExportArg(exportArg)
        if err != nil {
            log.Fatalf("invalid --export arg: %v", err)
        }
        // Discover tasks and export the selected one
        list, err := tasks.LoadTasks(cfg)
        if err != nil {
            log.Fatalf("failed to load tasks: %v", err)
        }
        var t *tasks.Task
        for i := range list {
            if list[i].ID == id {
                t = &list[i]
                break
            }
        }
        if t == nil {
            log.Fatalf("task not found: %s", id)
        }
        if err := zipper.ExportTask(*t, zipPath); err != nil {
            log.Fatalf("export failed: %v", err)
        }
        fmt.Printf("exported %s -> %s\n", id, zipPath)
        return
    }

    if importArg != "" {
        destRoot, err := tasks.ResolveStorageRoot(cfg)
        if err != nil {
            log.Fatalf("resolve storage root: %v", err)
        }
        if err := zipper.ImportAny(importArg, destRoot); err != nil {
            log.Fatalf("import failed: %v", err)
        }
        fmt.Printf("imported %s into %s\n", importArg, destRoot)
        return
    }

    // Inspect zip mode: extract to temp and point DataDir there
    var cleanup func()
    if inspectZip != "" {
        tmp, err := os.MkdirTemp("", "roo-task-inspect-*")
        if err != nil { log.Fatalf("mktemp: %v", err) }
        if err := zipper.ImportAny(inspectZip, tmp); err != nil { log.Fatalf("inspect import failed: %v", err) }
        cleanup = func() { _ = os.RemoveAll(tmp) }
        cfg.DataDir = tmp
        cfg.CodeChannel = "Custom"
    }

    if !(term.IsTerminal(int(os.Stdin.Fd())) || term.IsTerminal(int(os.Stdout.Fd()))) {
        list, err := tasks.LoadTasks(cfg)
        if err != nil { log.Fatalf("failed to load tasks: %v", err) }
        fmt.Printf("%d tasks\n", len(list))
        for _, t := range list { fmt.Printf("%s\t%s\n", t.ID, t.Title) }
        if cleanup != nil { cleanup() }
        return
    }
    model := tui.New(cfg)
    p := tea.NewProgram(model)
    if _, err := p.Run(); err != nil {
        log.Fatalf("tui error: %v", err)
    }
    if cleanup != nil { cleanup() }
}

func parseExportArg(s string) (id, zip string, err error) {
    for i := 0; i < len(s); i++ {
        if s[i] == ':' {
            return s[:i], s[i+1:], nil
        }
    }
    return "", "", fmt.Errorf("expected <task-id>:<zip-path>")
}
