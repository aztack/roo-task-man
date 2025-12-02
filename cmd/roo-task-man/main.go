package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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
        exportArg string // batch: <task-id>:<zip-path> OR, with --taskids/--date-range: <zip-path>
        importArg string // zip-path
        debug     bool
        inspectZip string
        showVersion bool
        taskIDsStr string // comma-separated task IDs for multi export
        dateRange  string // from..to, dates: YYYY-MM-DD or YYYYMMDD (inclusive)
        workspace  string // workspace path for import registration
        restore     bool   // interactive restore of state DB from backups
    )

    flag.StringVar(&cfgPath, "config", filepath.Join(config.UserHome(), ".config", "roo-code-man.json"), "config file path")
    flag.StringVar(&pluginID, "plugin-id", "", "VS Code extension plugin ID (overrides config)")
    flag.StringVar(&codeChan, "code-channel", "", "Editor channel/name: Code | Insiders | VSCodium | Cursor | Windsurf | Trae | Custom | <AppDir>")
    flag.StringVar(&editor, "editor", "", "Alias of --code-channel")
    flag.StringVar(&dataDir, "data-dir", "", "override VS Code globalStorage root directory")
    flag.StringVar(&hooksDir, "hooks-dir", "", "directory containing JS hook files")
    flag.StringVar(&exportDir, "export-dir", "", "default export directory for TUI exports")
    flag.StringVar(&exportArg, "export", "", "export: <task-id>:<zip-path> or, with --taskids/--date-range, <zip-path>")
    flag.StringVar(&importArg, "import", "", "batch import: <zip-path>")
    flag.StringVar(&inspectZip, "inspect", "", "inspect tasks from a zip (open TUI on extracted content)")
    flag.StringVar(&taskIDsStr, "taskids", "", "comma-separated task UIDs to export into a single archive")
    flag.StringVar(&dateRange, "date-range", "", "date range for export: from..to; dates YYYY-MM-DD or YYYYMMDD (inclusive)")
    flag.StringVar(&workspace, "workspace", "", "workspace path to associate on --import (updates state.vscdb)")
    flag.BoolVar(&restore, "restore", false, "restore state DB from backups (interactive)")
    flag.BoolVar(&debug, "debug", false, "print debug info (paths, counts)")
    flag.BoolVar(&showVersion, "version", false, "print version and exit")
    flag.Parse()

    if showVersion {
        fmt.Printf("Version: %s\n", version.String())
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

    // Restore mode
    if restore {
        // Warn loudly to close the editor first
        fmt.Printf("[restore] Please fully close %s before restoring. Press Enter to continue...\n", tasks.DisplayEditorName(cfg.CodeChannel))
        fmt.Scanln()
        infos, dir, err := tasks.ListBackups(cfg)
        if err != nil { log.Fatalf("list backups: %v", err) }
        if len(infos) == 0 { fmt.Println("no backups found"); return }
        rm := tui.NewRestore(infos, dir, tasks.DisplayEditorName(cfg.CodeChannel))
        p := tea.NewProgram(rm)
        res, err := p.Run()
        if err != nil { log.Fatalf("restore TUI error: %v", err) }
        sel := res.(tui.RestoreModel).Selected()
        if sel == "" { fmt.Println("restore canceled"); return }
        if err := tasks.RestoreFromBackup(cfg, sel, cfg.Debug); err != nil {
            log.Fatalf("restore failed: %v", err)
        }
        fmt.Printf("restored state DBs from suffix %s\n", sel)
        return
    }

    // Batch operations
    if exportArg != "" || taskIDsStr != "" || dateRange != "" {
        // Two modes:
        // 1) Legacy single export: --export <task-id>:<zip-path>
        // 2) Multi export: --export <zip-path> with one/both filters: --taskids, --date-range
        if taskIDsStr == "" && dateRange == "" {
            // Legacy single
            id, zipPath, err := parseExportArg(exportArg)
            if err != nil { log.Fatalf("invalid --export arg: %v", err) }
            list, err := tasks.LoadTasks(cfg)
            if err != nil { log.Fatalf("failed to load tasks: %v", err) }
            var t *tasks.Task
            for i := range list { if list[i].ID == id { t = &list[i]; break } }
            if t == nil { log.Fatalf("task not found: %s", id) }
            if err := zipper.ExportTask(*t, zipPath); err != nil { log.Fatalf("export failed: %v", err) }
            fmt.Printf("exported %s -> %s\n", id, zipPath)
            return
        }
        // Multi export path
        zipPath := exportArg
        if exportArg == "" {
            if taskIDsStr != "" {
                // derive default filename in CWD: <editor>-<plugin-id>-<A>_<B>_<C>.zip
                idsOrder := splitCSV(taskIDsStr)
                zipPath = defaultExportName(tasks.DisplayEditorName(cfg.CodeChannel), cfg.PluginID, idsOrder)
                if cfg.Debug { fmt.Printf("[export] no --export provided; using %s\n", zipPath) }
            } else {
                log.Fatal("--export <zip-path> is required when using --date-range only")
            }
        } else {
            if hasColon(exportArg) { log.Fatalf("when using --taskids/--date-range, --export must be <zip-path> (not <id>:<zip>)") }
        }

        // Load tasks
        list, err := tasks.LoadTasks(cfg)
        if err != nil { log.Fatalf("failed to load tasks: %v", err) }

        // Build ID filter set
        idSet := map[string]struct{}{}
        if taskIDsStr != "" {
            for _, id := range splitCSV(taskIDsStr) { idSet[id] = struct{}{} }
        }
        // Parse date range
        var from, to *time.Time
        if dateRange != "" {
            f, t, err := parseDateRange(dateRange)
            if err != nil { log.Fatalf("invalid --date-range: %v", err) }
            from, to = f, t
        }
        // Select tasks (union semantics across filters)
        selected := make([]tasks.Task, 0)
        for _, t := range list {
            matchesID := false
            if len(idSet) == 0 { matchesID = false } else { _, matchesID = idSet[t.ID] }
            matchesDate := true
            if from != nil && t.CreatedAt.Before(*from) { matchesDate = false }
            if to != nil && t.CreatedAt.After(*to) { matchesDate = false }

            include := false
            if len(idSet) > 0 && matchesID { include = true }
            if (from != nil || to != nil) && matchesDate { include = true }
            if include { selected = append(selected, t) }
        }
        if len(selected) == 0 { log.Fatal("no tasks matched filters for export") }
        if err := zipper.ExportTasks(selected, zipPath); err != nil { log.Fatalf("export failed: %v", err) }
        fmt.Printf("exported %d tasks -> %s\n", len(selected), zipPath)
        return
    }

    if importArg != "" {
        destRoot, err := tasks.ResolveStorageRoot(cfg)
        if err != nil { log.Fatalf("resolve storage root: %v", err) }
        ids, err := zipper.InspectIDs(importArg)
        if err != nil { log.Fatalf("read manifest: %v", err) }
        if cfg.Debug {
            fmt.Printf("[import] manifest IDs: %v\n", ids)
            fmt.Printf("[import] destination root: %s\n", destRoot)
        }
        // enable zipper debug if requested
        zipper.EnableDebug(cfg.Debug)
        if err := zipper.ImportAny(importArg, destRoot); err != nil { log.Fatalf("import failed: %v", err) }
        fmt.Printf("imported %s into %s\n", importArg, destRoot)
        // Determine workspace for registration: default to current working directory if not provided
        if workspace == "" {
            if wd, err := os.Getwd(); err == nil {
                workspace = wd
                if cfg.Debug { fmt.Printf("[import] workspace not provided; defaulting to CWD: %s\n", workspace) }
            } else {
                if cfg.Debug { fmt.Printf("[import] could not resolve CWD for workspace; skipping registration: %v\n", err) }
                return
            }
        }
        // Register into global state DB
        list, err := tasks.LoadTasks(cfg)
        if err != nil { log.Fatalf("post-import load tasks: %v", err) }
        byID := map[string]tasks.Task{}
        for _, t := range list { byID[t.ID] = t }
        var selected []tasks.Task
        for _, id := range ids { if t, ok := byID[id]; ok { selected = append(selected, t) } }
        if len(selected) == 0 { log.Printf("warning: no imported tasks found for registration") }
        if err := tasks.RegisterImportedTasks(cfg, workspace, selected); err != nil {
            log.Fatalf("register in global state failed: %v", err)
        }
        // Integrity check: verify inserted IDs present in primary and backup DBs
        primary, backup, err := tasks.VerifyRegistration(cfg, ids)
        if err != nil {
            log.Printf("integrity check failed: %v", err)
        }
        // summarize
        pOK, bOK := 0, 0
        missingP, missingB := []string{}, []string{}
        for _, id := range ids {
            if primary != nil && primary[id] { pOK++ } else { missingP = append(missingP, id) }
            if backup != nil && backup[id] { bOK++ } else { missingB = append(missingB, id) }
        }
        fmt.Printf("registered %d tasks into global state for workspace %s\n", len(selected), workspace)
        if cfg.Debug {
            fmt.Printf("integrity: primary %d/%d ok\n", pOK, len(ids))
            fmt.Printf("integrity: backup  %d/%d ok\n", bOK, len(ids))
            if len(missingP) > 0 { fmt.Printf("[integrity] primary missing: %v\n", missingP) }
            if len(missingB) > 0 { fmt.Printf("[integrity] backup missing: %v\n", missingB) }
        }
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

func hasColon(s string) bool {
    for i := 0; i < len(s); i++ { if s[i] == ':' { return true } }
    return false
}

func splitCSV(s string) []string {
    if s == "" { return nil }
    parts := strings.Split(s, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        p = strings.TrimSpace(p)
        if p != "" { out = append(out, p) }
    }
    return out
}

// defaultExportName returns a default zip filename in CWD using the editor name, pluginID,
// and the first segment of each task ID (split by '-'), joined with underscores.
func defaultExportName(editorName, pluginID string, ids []string) string {
    parts := make([]string, 0, len(ids))
    useFirstSegment := len(ids) > 1
    for _, id := range ids {
        seg := id
        if useFirstSegment {
            if i := strings.Index(id, "-"); i > 0 { seg = id[:i] }
        }
        parts = append(parts, seg)
    }
    ed := strings.ToLower(strings.ReplaceAll(editorName, " ", ""))
    name := fmt.Sprintf("%s-%s-%s.zip", ed, pluginID, strings.Join(parts, "_"))
    // path in current working directory
    return name
}

// parseDateRange parses "from..to" with dates in YYYY-MM-DD or YYYYMMDD.
// Returns inclusive time bounds in local time at 00:00:00 and 23:59:59.999...
func parseDateRange(s string) (*time.Time, *time.Time, error) {
    dots := strings.Index(s, "..")
    if dots <= 0 || dots+2 > len(s) { return nil, nil, fmt.Errorf("expected from..to") }
    left := strings.TrimSpace(s[:dots])
    right := strings.TrimSpace(s[dots+2:])
    if left == "" || right == "" { return nil, nil, fmt.Errorf("both from and to are required") }
    parse := func(in string) (time.Time, error) {
        // Try YYYY-MM-DD
        if len(in) == 10 && in[4] == '-' && in[7] == '-' {
            t, err := time.ParseInLocation("2006-01-02", in, time.Local)
            if err == nil { return t, nil }
        }
        // Try YYYYMMDD
        if len(in) == 8 {
            t, err := time.ParseInLocation("20060102", in, time.Local)
            if err == nil { return t, nil }
        }
        return time.Time{}, fmt.Errorf("invalid date: %q", in)
    }
    f, err := parse(left); if err != nil { return nil, nil, err }
    t, err := parse(right); if err != nil { return nil, nil, err }
    // Normalize to day start/end
    f = time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, f.Location())
    t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Millisecond*999000), t.Location())
    return &f, &t, nil
}
