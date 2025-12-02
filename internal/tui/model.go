package tui

import (
    "fmt"
    "path/filepath"
    "time"
    "log"
    "strings"
    "os/exec"
    "runtime"
    "sort"
    
    "github.com/charmbracelet/bubbles/help"
    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/bubbles/list"
    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/textinput"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/glamour"
    tea "github.com/charmbracelet/bubbletea"

    "roocode-task-man/internal/config"
    "roocode-task-man/internal/hooks"
    "roocode-task-man/internal/tasks"
    "roocode-task-man/internal/zipper"
)

type model struct {
    cfg       config.Config
    list      list.Model
    detail    *tasks.Task
    help      help.Model
    vp        viewport.Model
    spin      spinner.Model
    input     textinput.Model
    width     int
    height    int
    statusMsg string

    confirmingDelete bool
    hooks    *hooks.HookEnv
    showHelp bool
    loading  bool
    tasks    []tasks.Task
    pendingG bool
    // detail search
    searchMode bool
    searchQuery string
    rawDetail string
    renderedDetail string
    topMsg string
    // modes and sorting
    sortAsc bool
    // history indexes
    histAll []int
    histAI []int
    histUser []int
    lastFilter string
    selected map[string]bool
    hookApplied map[string]bool // tracks which task IDs had hooks applied
    // Selection state tracker for IME/filtering robustness
    selectionTracker map[string]bool // persistent selection state by task ID
}

type item struct{ t tasks.Task; selected bool; desc string; title string }

func (i item) Title() string       { if i.selected { return selectedPrefix() + i.title }; return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string {
    return i.t.Title + " " + i.t.ID + " " + humanTime(i.t.CreatedAt) + " uid:" + i.t.ID + " -uid=" + i.t.ID + " -d=" + humanTime(i.t.CreatedAt) + " " + i.desc
}

type keymap struct{
    open key.Binding
    back key.Binding
    refresh key.Binding
    export key.Binding
    exportSel key.Binding
    toggleSel key.Binding
    toggleSelAlt key.Binding
    clearSel key.Binding
    openDir key.Binding
    sort key.Binding
    del key.Binding
    quit key.Binding
}

func newKeymap() keymap {
    return keymap{
        open: key.NewBinding(key.WithKeys("enter", "l"), key.WithHelp("enter/l", "open")),
        back: key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "back")),
        refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
        export: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export zip")),
        exportSel: key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "export selected")),
        toggleSel: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle select")),
        toggleSelAlt: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "toggle select")),
        clearSel: key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "clear selection")),
        openDir: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open task dir")),
        sort: key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sort time")),
        del: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
        quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
    }
}

var keys = newKeymap()

func New(cfg config.Config) model {
    lm := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
    lm.Title = "RooCode Tasks — " + tasks.DisplayEditorName(cfg.CodeChannel) + "  [sort:desc]"
    lm.SetShowStatusBar(false)
    lm.SetFilteringEnabled(true)
    lm.AdditionalShortHelpKeys = func() []key.Binding { return []key.Binding{keys.open, keys.refresh, keys.sort, keys.toggleSel, keys.toggleSelAlt, keys.export, keys.exportSel, keys.clearSel, keys.del, keys.quit} }
    lm.AdditionalFullHelpKeys = lm.AdditionalShortHelpKeys
    // Make help a bit more visible (but not too bright)
    hs := lm.Styles.HelpStyle
    hs = hs.Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#B0B7C3"}).Bold(true)
    lm.Styles.HelpStyle = hs
    sp := spinner.New()
    sp.Spinner = spinner.MiniDot
    ti := textinput.New()
    ti.Placeholder = "search..."
    ti.CharLimit = 200
    m := model{
        cfg: cfg, list: lm, help: help.New(), spin: sp, input: ti, loading: true,
        selected: map[string]bool{},
        hookApplied: map[string]bool{},
        selectionTracker: map[string]bool{},
    }
    return m
}

func (m model) Init() tea.Cmd {
    return tea.Batch(loadHooksCmd(m.cfg), loadTasksWithHooksCmd(m.cfg), spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        m.list.SetSize(m.width, m.height-2)
        return m, nil
    case tasksLoadedMsg:
        m.tasks = []tasks.Task(msg)
        m.loading = false
        m.rebuildListItemsPreserveSelection()
        if len(m.tasks) == 0 {
            m.statusMsg = "No tasks found"
        } else {
            m.statusMsg = fmt.Sprintf("%d tasks", len(m.tasks))
            m.list.Select(0)
        }
        return m, nil
    case hooksLoadedMsg:
        m.hooks = msg.env
        if m.hooks != nil {
            m.setTitle(true)
            // Rebuild list items to apply renderTaskListItem overrides
            if len(m.tasks) > 0 {
                m.rebuildListItemsPreserveSelection()
            }
        }
        return m, nil
    case exportProgressMsg:
        if msg.err != nil {
            m.statusMsg = "export failed: " + msg.err.Error()
        } else {
            if ap, _ := filepath.Abs(msg.zipPath); ap != "" { msg.zipPath = ap }
            if m.detail != nil {
                m.topMsg = fmt.Sprintf("Exported %d tasks to %s", msg.total, msg.zipPath)
            } else {
                m.statusMsg = fmt.Sprintf("exported %d tasks to %s", msg.total, msg.zipPath)
            }
            _ = openInExplorer(filepath.Dir(msg.zipPath))
        }
        return m, nil
    case spinner.TickMsg:
        var cmd tea.Cmd
        m.spin, cmd = m.spin.Update(msg)
        return m, cmd
    case errMsg:
        m.statusMsg = "error: " + msg.Error()
        return m, nil
    case tea.KeyMsg:
        if m.detail != nil {
            // Handle search input first
            if m.searchMode {
                switch msg.Type {
                case tea.KeyEnter:
                    m.searchQuery = m.input.Value()
                    m.searchMode = false
                    m.applyDetailSearch()
                    return m, nil
                case tea.KeyEsc, tea.KeyCtrlC:
                    m.searchMode = false
                    return m, nil
                }
                var cmd tea.Cmd
                m.input, cmd = m.input.Update(msg)
                return m, cmd
            }
            s := msg.String()
            switch s {
            case keys.back.Keys()[0], "q":
                m.detail = nil
                m.confirmingDelete = false
                m.pendingG = false
                return m, nil
            case "j", "down":
                m.vp.LineDown(1); return m, nil
            case "k", "up":
                m.vp.LineUp(1); return m, nil
            case "pgdown", "ctrl+f":
                m.vp.ViewDown(); return m, nil
            case "pgup", "ctrl+b":
                m.vp.ViewUp(); return m, nil
            case "ctrl+d":
                m.vp.HalfViewDown(); return m, nil
            case "ctrl+u":
                m.vp.HalfViewUp(); return m, nil
            case "g":
                if m.pendingG { m.vp.GotoTop(); m.pendingG = false } else { m.pendingG = true }
                return m, nil
            case "G":
                m.vp.GotoBottom(); return m, nil
            case keys.openDir.Keys()[0]:
                if m.detail != nil {
                    _ = openInExplorer(m.detail.Path)
                    m.topMsg = "Opened folder"
                }
                return m, nil
            case "J":
                m.jumpNextEntry(); return m, nil
            case "K":
                m.jumpPrevEntry(); return m, nil
            case "]":
                m.jumpNextRole("ai"); return m, nil
            case "[":
                m.jumpPrevRole("ai"); return m, nil
            case "}":
                m.jumpNextRole("user"); return m, nil
            case "{":
                m.jumpPrevRole("user"); return m, nil
            case "/":
                m.searchMode = true
                m.input.SetValue("")
                m.input.Focus()
                return m, nil
            case "n":
                m.findNext()
                return m, nil
            case "N":
                m.findPrev()
                return m, nil
            }
            // Detail-specific actions could go here
            m.pendingG = false
            return m, nil
        }
        // List view key handling
        switch msg.String() {
        case keys.quit.Keys()[0], "ctrl+c":
            return m, tea.Quit
        case keys.open.Keys()[0], "l":
            if it, ok := m.list.SelectedItem().(item); ok {
                t := it.t
                m.detail = &t
                m.renderDetailViewport()
            }
            return m, nil
        case keys.refresh.Keys()[0]:
            m.loading = true
            return m, tea.Batch(loadHooksCmd(m.cfg), loadTasksWithHooksCmd(m.cfg))
        case "S":
            m.sortAsc = !m.sortAsc
            m.sortTasks()
            m.rebuildListItemsPreserveSelection()
            m.setTitle(m.hooks != nil)
            return m, nil
        case keys.openDir.Keys()[0]:
            if it, ok := m.list.SelectedItem().(item); ok {
                _ = openInExplorer(it.t.Path)
                m.statusMsg = "Opened folder"
            }
            return m, nil
        case "pgdown", "ctrl+f", "ctrl+d":
            per := len(m.list.VisibleItems())
            if per <= 0 { per = 1 }
            idx := m.list.Index() + per
            if idx >= len(m.list.Items()) { idx = len(m.list.Items()) - 1 }
            if idx < 0 { idx = 0 }
            m.list.Select(idx)
            return m, nil
        case "pgup", "ctrl+b", "ctrl+u":
            per := len(m.list.VisibleItems())
            if per <= 0 { per = 1 }
            idx := m.list.Index() - per
            if idx < 0 { idx = 0 }
            m.list.Select(idx)
            return m, nil
        case keys.export.Keys()[0]:
            if it, ok := m.list.SelectedItem().(item); ok {
                // If there are selected items, export all selected; else export current item
                sel := m.selectedTasks()
                if len(sel) > 0 {
                    base := m.cfg.ExportDir
                    if base == "" { base = "." }
                    prefix := fmt.Sprintf("%s-%s", slug(tasks.DisplayEditorName(m.cfg.CodeChannel)), slug(m.cfg.PluginID))
                    zipPath := filepath.Join(base, fmt.Sprintf("%s-tasks-%s.zip", prefix, time.Now().Format("20060102-150405")))
                    if m.detail != nil { m.topMsg = fmt.Sprintf("Exporting %d tasks... 0%%", len(sel)) }
                    return m, exportTasksCmd(sel, zipPath)
                } else {
                    t := it.t
                    base := m.cfg.ExportDir
                    if base == "" { base = "." }
                    prefix := fmt.Sprintf("%s-%s", slug(tasks.DisplayEditorName(m.cfg.CodeChannel)), slug(m.cfg.PluginID))
                    zipPath := filepath.Join(base, fmt.Sprintf("%s-%s.zip", prefix, t.ID))
                    if m.detail != nil { m.topMsg = "Exporting task... 0%" }
                    return m, exportTasksCmd([]tasks.Task{t}, zipPath)
                }
            }
            return m, nil
        case keys.exportSel.Keys()[0]:
            sel := m.selectedTasks()
            if len(sel) == 0 { m.statusMsg = "no selected tasks"; return m, nil }
            base := m.cfg.ExportDir
            if base == "" { base = "." }
            prefix := fmt.Sprintf("%s-%s", slug(tasks.DisplayEditorName(m.cfg.CodeChannel)), slug(m.cfg.PluginID))
            zipPath := filepath.Join(base, fmt.Sprintf("%s-tasks-%s.zip", prefix, time.Now().Format("20060102-150405")))
            if m.detail != nil { m.topMsg = fmt.Sprintf("Exporting %d tasks... 0%%", len(sel)) }
            return m, exportTasksCmd(sel, zipPath)
        case keys.toggleSel.Keys()[0], keys.toggleSelAlt.Keys()[0]:
            // Toggle selection using persistent tracker for IME robustness
            if selItem, ok := m.list.SelectedItem().(item); ok {
                id := selItem.t.ID
                // Toggle in persistent tracker
                m.selectionTracker[id] = !m.selectionTracker[id]
                // Update display for current filtered view
                for i, li := range m.list.Items() {
                    it := li.(item)
                    if it.t.ID == id {
                        it.selected = m.selectionTracker[id]
                        m.selected[id] = m.selectionTracker[id]
                        m.list.SetItem(i, it)
                        break
                    }
                }
            }
            return m, nil
        case keys.clearSel.Keys()[0]:
            // Clear all selections using persistent tracker
            m.selectionTracker = map[string]bool{}
            m.selected = map[string]bool{}
            for i, li := range m.list.Items() {
                it := li.(item)
                if it.selected {
                    it.selected = false
                    m.list.SetItem(i, it)
                }
            }
            m.statusMsg = "selection cleared"
            return m, nil
        case "?":
            m.showHelp = !m.showHelp
            m.list.SetShowHelp(m.showHelp)
            return m, nil
        case keys.del.Keys()[0]:
            if !m.confirmingDelete {
                m.confirmingDelete = true
                m.statusMsg = "Delete selected task? y/N"
                return m, nil
            }
            // already confirming, ignore
            return m, nil
        case "y":
            if m.confirmingDelete {
                m.confirmingDelete = false
                if it, ok := m.list.SelectedItem().(item); ok {
                    if err := tasks.DeleteTask(it.t); err != nil {
                        m.statusMsg = "delete failed: " + err.Error()
                    } else {
                        m.statusMsg = "deleted: " + it.t.ID
                        return m, loadTasksWithHooksCmd(m.cfg)
                    }
                }
            }
            return m, nil
        case "n":
            if m.confirmingDelete {
                m.confirmingDelete = false
                m.statusMsg = "canceled"
            }
            return m, nil
        }
    }

    // Delegate other events to list
    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    // If filter string changed, rebuild items with special token pre-filtering
    if f := m.list.FilterValue(); f != m.lastFilter {
        m.lastFilter = f
        m.rebuildListItemsPreserveSelection()
    }
    return m, cmd
}

func (m model) View() string {
    if m.detail != nil {
        header := "(h) back  (o) open dir  (e/E) export  (x) delete  (/) search  (n/N) next/prev  (J/K) next/prev entry  ([/]) ai prev/next  ({/}) user prev/next  (q) close"
        if m.topMsg != "" {
            header = header + "\n" + m.topMsg
        }
        if m.searchMode {
            header = header + "\n/ " + m.input.View()
        }
        return header + "\n\n" + m.vp.View()
    }
    if m.loading {
        return fmt.Sprintf("%s Loading tasks...", m.spin.View())
    }
    return m.list.View() + footer(m.statusMsg)
}

func (m model) selectedTasks() []tasks.Task {
    out := []tasks.Task{}
    for _, li := range m.list.Items() {
        it := li.(item)
        if it.selected { out = append(out, it.t) }
    }
    return out
}

func footer(msg string) string {
    if msg == "" { return "" }
    return "\n" + msg + "\n"
}

type tasksLoadedMsg []tasks.Task
type errMsg struct{ error }

func (e errMsg) Error() string { return e.error.Error() }

func loadTasksCmd(cfg config.Config) tea.Cmd {
    return func() tea.Msg {
        list, err := tasks.LoadTasks(cfg)
        if err != nil {
            return errMsg{err}
        }
        return tasksLoadedMsg(list)
    }
}

func loadTasksWithHooksCmd(cfg config.Config) tea.Cmd {
    return func() tea.Msg {
        hooks.EnableDebug(cfg.Debug)
        env, _ := hooks.LoadDir(cfg.HooksDir)
        list, err := tasks.LoadTasksWithHooks(cfg, env)
        if err != nil { return errMsg{err} }
        if cfg.Debug {
            if root, err2 := tasks.ResolveStorageRoot(cfg); err2 == nil {
                log.Printf("storage root: %s", root)
            }
            for _, t := range list { log.Printf("task %s: %s", t.ID, t.Path) }
        }
        return tasksLoadedMsg(list)
    }
}

type hooksLoadedMsg struct{ env *hooks.HookEnv }

type exportProgressMsg struct{ current, total int; zipPath string; err error }

func loadHooksCmd(cfg config.Config) tea.Cmd {
    return func() tea.Msg {
        hooks.EnableDebug(cfg.Debug)
        env, _ := hooks.LoadDir(cfg.HooksDir)
        return hooksLoadedMsg{env}
    }
}

func exportTasksCmd(sel []tasks.Task, zipPath string) tea.Cmd {
    return func() tea.Msg {
        err := zipper.ExportTasksWithProgress(sel, zipPath, func(current, total int) {
            // Note: We can't send messages from callback, but we track final state
        })
        return exportProgressMsg{current: len(sel), total: len(sel), zipPath: zipPath, err: err}
    }
}

func (m *model) setTitle(haveHooks bool) {
    sortStr := "desc"
    if m.sortAsc { sortStr = "asc" }
    base := "RooCode Tasks — " + tasks.DisplayEditorName(m.cfg.CodeChannel) + "  [sort:" + sortStr + "]"
    if haveHooks { base += "  [hooks]" }
    m.list.Title = base
}

func (m *model) renderDetailViewport() {
    if m.detail == nil { return }
    // build markdown content then render via glamour
    content := renderDetailMarkdown(*m.detail, m.hooks, m.cfg.Debug)
    m.rawDetail = content
    // Use glamour to render markdown to ANSI suitable for terminal
    r, err := glamour.NewTermRenderer(glamour.WithAutoStyle())
    if err == nil {
        if s, err2 := r.Render(content); err2 == nil { content = s }
    }
    m.renderedDetail = content
    m.vp = viewport.New(m.width, max(3, m.height-4))
    m.vp.SetContent(content)
    m.buildHistoryIndexes()
}

func max(a, b int) int { if a>b { return a }; return b }

func humanTime(t time.Time) string {
    if t.IsZero() { return "" }
    return t.Local().Format("2006-01-02 15:04")
}

func (m *model) rebuildListItemsPreserveSelection() {
    base := m.tasks
    // Special token pre-filtering
    q := strings.TrimSpace(m.list.FilterValue())
    if strings.Contains(q, "-uid=") || strings.Contains(q, "-d") {
        uidTok, dateOp, dateTok := parseSpecialFilter(q)
        filtered := make([]tasks.Task, 0, len(base))
        for _, t := range base {
            ok := true
            if uidTok != "" && !strings.Contains(strings.ToLower(t.ID), strings.ToLower(uidTok)) { ok = false }
            if dateTok != "" && !matchesDateFilter(t.CreatedAt, dateOp, dateTok) { ok = false }
            if ok { filtered = append(filtered, t) }
        }
        base = filtered
    }

    items := make([]list.Item, 0, len(base))
    // Clear hook tracking before rebuilding
    m.hookApplied = map[string]bool{}

    for _, t := range base {
        shownTitle := sanitizeTitle(t.Title)
        // Get selection state from persistent tracker (robust to IME/filter changes)
        isSelected := m.selectionTracker[t.ID]
        // Sync to display state
        m.selected[t.ID] = isSelected

        // Hook: renderTaskListItem can override title/desc
        if m.hooks != nil {
            mv := map[string]any{"id": t.ID, "title": t.Title, "summary": t.Summary, "createdAt": t.CreatedAt.Format(time.RFC3339), "path": t.Path, "meta": t.Meta}
            if m.cfg.Debug { log.Printf("[hooks] calling renderTaskListItem for %s", t.ID) }
            if out, ok := m.hooks.CallExported("renderTaskListItem", mv); ok {
                if om, ok2 := out.(map[string]any); ok2 {
                    if s, ok3 := om["title"].(string); ok3 && s != "" { shownTitle = sanitizeTitle(s) }
                    if s, ok3 := om["desc"].(string); ok3 && s != "" {
                        if m.cfg.Debug { log.Printf("[hooks] renderTaskListItem override for %s", t.ID) }
                        m.hookApplied[t.ID] = true
                        hookBadge := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("[H] ")
                        items = append(items, item{t: t, selected: isSelected, desc: sanitizeInline(s), title: hookBadge + shownTitle})
                        continue
                    }
                    if s, ok3 := om["description"].(string); ok3 && s != "" {
                        if m.cfg.Debug { log.Printf("[hooks] renderTaskListItem override(desc) for %s", t.ID) }
                        m.hookApplied[t.ID] = true
                        hookBadge := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("[H] ")
                        items = append(items, item{t: t, selected: isSelected, desc: sanitizeInline(s), title: hookBadge + shownTitle})
                        continue
                    }
                }
            } else if m.cfg.Debug {
                log.Printf("[hooks] renderTaskListItem no result for %s", t.ID)
            }
        }
        if shownTitle == "" { shownTitle = t.ID }
        title := shownTitle
        // always show second line: created and UID
        desc := fmt.Sprintf("%s • %s", humanTime(t.CreatedAt), t.ID)
        items = append(items, item{t: t, selected: isSelected, desc: desc, title: title})
    }
    m.list.SetItems(items)
}

func sanitizeInline(s string) string {
    // replace newlines with spaces to keep a single-line description
    b := make([]rune, 0, len(s))
    for _, r := range s { if r == '\n' || r == '\r' { r = ' ' }; b = append(b, r) }
    return string(b)
}

func sanitizeTitle(s string) string {
    ss := strings.TrimSpace(s)
    if strings.HasPrefix(ss, "```") {
        ss = strings.TrimPrefix(ss, "```")
        ss = strings.TrimSuffix(ss, "```")
    }
    // collapse newlines and backticks
    ss = strings.ReplaceAll(ss, "\n", " ")
    ss = strings.ReplaceAll(ss, "\r", " ")
    return strings.TrimSpace(ss)
}

func selectedPrefix() string {
    return lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("[x] ")
}

func parseSpecialFilter(q string) (uid, dateOp, dateVal string) {
    // parse -uid= and -d[op]= tokens (case-insensitive)
    // Operators: -d=   (contains), -d>=  (>=), -d<=  (<=), -d: (month match)
    parts := strings.Fields(q)
    for _, p := range parts {
        pp := strings.ToLower(p)
        if strings.HasPrefix(pp, "-uid=") {
            uid = strings.TrimSpace(p[len("-uid="):])
        }
        if strings.HasPrefix(pp, "-d") {
            // Order matters: check longer prefixes first
            if strings.HasPrefix(pp, "-d>=") {
                dateOp = ">="
                dateVal = strings.TrimSpace(p[len("-d>="):])
            } else if strings.HasPrefix(pp, "-d<=") {
                dateOp = "<="
                dateVal = strings.TrimSpace(p[len("-d<="):])
            } else if strings.HasPrefix(pp, "-d:") {
                dateOp = "match"
                dateVal = strings.TrimSpace(p[len("-d:"):])
            } else if strings.HasPrefix(pp, "-d=") {
                dateOp = "contains"
                dateVal = strings.TrimSpace(p[len("-d="):])
            }
        }
    }
    return
}

func matchesDateFilter(createdAt time.Time, op string, filterVal string) bool {
    if op == "" || filterVal == "" {
        return true
    }

    switch op {
    case "contains":
        // Simple substring match on formatted time
        return strings.Contains(strings.ToLower(humanTime(createdAt)), strings.ToLower(filterVal))
    case "match":
        // Month/year pattern matching (e.g., "2024-12" or "2024")
        formatted := createdAt.Format("2006-01-02")
        return strings.HasPrefix(formatted, filterVal)
    case ">=":
        // Parse date and compare: -d>=2024-12-01
        target, err := time.Parse("2006-01-02", filterVal)
        if err != nil {
            // Fallback to substring matching if parse fails
            return strings.Contains(humanTime(createdAt), filterVal)
        }
        return createdAt.After(target) || createdAt.Equal(target.Truncate(24*time.Hour))
    case "<=":
        // Parse date and compare: -d<=2024-12-31
        target, err := time.Parse("2006-01-02", filterVal)
        if err != nil {
            return strings.Contains(humanTime(createdAt), filterVal)
        }
        // End of day comparison
        endOfDay := target.Add(24*time.Hour - time.Nanosecond)
        return createdAt.Before(endOfDay) || createdAt.Equal(target.Truncate(24*time.Hour))
    default:
        return true
    }
}

func slug(s string) string {
    s = strings.ToLower(strings.TrimSpace(s))
    // replace spaces and slashes with dashes
    r := make([]rune, 0, len(s))
    for _, ch := range s {
        if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
            r = append(r, ch)
        } else if ch == ' ' || ch == '/' || ch == '\\' {
            r = append(r, '-')
        } else {
            // drop other punctuation
        }
    }
    out := strings.Trim(strings.ReplaceAll(string(r), "--", "-"), "-")
    if out == "" { out = "export" }
    return out
}

func (m *model) sortTasks() {
    if m.sortAsc {
        sort.Slice(m.tasks, func(i, j int) bool { return m.tasks[i].CreatedAt.Before(m.tasks[j].CreatedAt) })
    } else {
        sort.Slice(m.tasks, func(i, j int) bool { return m.tasks[i].CreatedAt.After(m.tasks[j].CreatedAt) })
    }
}

func (m *model) buildHistoryIndexes() {
    m.histAll, m.histAI, m.histUser = nil, nil, nil
    lines := strings.Split(m.renderedDetail, "\n")
    for i, ln := range lines {
        if strings.HasPrefix(ln, "### ") {
            m.histAll = append(m.histAll, i)
            if strings.Contains(ln, "🤖") { m.histAI = append(m.histAI, i) }
            if strings.Contains(ln, "🧑") { m.histUser = append(m.histUser, i) }
        }
    }
}

func (m *model) jumpNextEntry() { jumpToNext(&m.vp, m.histAll) }
func (m *model) jumpPrevEntry() { jumpToPrev(&m.vp, m.histAll) }
func (m *model) jumpNextRole(role string) {
    if role == "ai" { jumpToNext(&m.vp, m.histAI); return }
    if role == "user" { jumpToNext(&m.vp, m.histUser); return }
}
func (m *model) jumpPrevRole(role string) {
    if role == "ai" { jumpToPrev(&m.vp, m.histAI); return }
    if role == "user" { jumpToPrev(&m.vp, m.histUser); return }
}

func jumpToNext(vp *viewport.Model, lines []int) {
    if len(lines) == 0 { return }
    cur := vp.YOffset
    for _, ln := range lines { if ln > cur { vp.SetYOffset(ln); return } }
    vp.SetYOffset(lines[0])
}
func jumpToPrev(vp *viewport.Model, lines []int) {
    if len(lines) == 0 { return }
    cur := vp.YOffset
    for i := len(lines)-1; i >= 0; i-- { if lines[i] < cur { vp.SetYOffset(lines[i]); return } }
    vp.SetYOffset(lines[len(lines)-1])
}

func openInExplorer(dir string) error {
    if dir == "" { return nil }
    switch runtime.GOOS {
    case "darwin":
        return exec.Command("open", dir).Start()
    case "linux":
        return exec.Command("xdg-open", dir).Start()
    case "windows":
        return exec.Command("explorer", dir).Start()
    default:
        return nil
    }
}

func (m *model) applyDetailSearch() {
    if m.searchQuery == "" { m.vp.SetContent(m.renderedDetail); return }
    // simple case-insensitive highlight using lipgloss
    style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
    wrap := func(s string) string { return style.Render(s) }
    rendered := highlightAll(m.renderedDetail, m.searchQuery, wrap)
    m.vp.SetContent(rendered)
    // jump to first match line
    line := findFirstLine(rendered, m.searchQuery)
    if line > 0 { m.vp.SetYOffset(line) }
}

func highlightAll(s, q string, wrap func(string) string) string {
    if q == "" { return s }
    // naive replacement, case-insensitive
    lowerS, lowerQ := strings.ToLower(s), strings.ToLower(q)
    var out strings.Builder
    i := 0
    for i < len(s) {
        idx := strings.Index(lowerS[i:], lowerQ)
        if idx < 0 { out.WriteString(s[i:]); break }
        idx = i + idx
        out.WriteString(s[i:idx])
        out.WriteString(wrap(s[idx:idx+len(q)]))
        i = idx + len(q)
    }
    return out.String()
}

func findFirstLine(s, q string) int {
    if q == "" { return 0 }
    lowerQ := strings.ToLower(q)
    lines := strings.Split(s, "\n")
    for i, ln := range lines {
        if strings.Contains(strings.ToLower(ln), lowerQ) { return i }
    }
    return 0
}

func (m *model) findAllMatchLines() []int {
    if m.searchQuery == "" { return nil }
    lowerQ := strings.ToLower(m.searchQuery)
    lines := strings.Split(m.vp.View(), "\n")
    out := []int{}
    for i, ln := range lines {
        if strings.Contains(strings.ToLower(ln), lowerQ) { out = append(out, i) }
    }
    return out
}

func (m *model) findNext() {
    lines := m.findAllMatchLines()
    if len(lines) == 0 { return }
    cur := m.vp.YOffset
    for _, ln := range lines {
        if ln > cur { m.vp.SetYOffset(ln); return }
    }
    // wrap to first
    m.vp.SetYOffset(lines[0])
}

func (m *model) findPrev() {
    lines := m.findAllMatchLines()
    if len(lines) == 0 { return }
    cur := m.vp.YOffset
    for i := len(lines)-1; i >= 0; i-- {
        if lines[i] < cur { m.vp.SetYOffset(lines[i]); return }
    }
    // wrap to last
    m.vp.SetYOffset(lines[len(lines)-1])
}
