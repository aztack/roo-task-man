package tui

import (
    "fmt"
    "os/exec"
    "runtime"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "roocode-task-man/internal/tasks"
)

type RestoreModel struct {
    entries []tasks.BackupInfo
    dir     string
    editor  string
    idx     int
    quitting bool
    selectedSuffix string
    msg string
}

func NewRestore(infos []tasks.BackupInfo, dir string, editor string) RestoreModel {
    return RestoreModel{entries: infos, dir: dir, editor: editor, idx: 0}
}

func (m RestoreModel) Init() tea.Cmd { return nil }

func (m RestoreModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "esc", "ctrl+c":
            m.quitting = true
            return m, tea.Quit
        case "up", "k":
            if m.idx > 0 { m.idx-- }
            return m, nil
        case "down", "j":
            if m.idx < len(m.entries)-1 { m.idx++ }
            return m, nil
        case "enter":
            if len(m.entries) > 0 {
                m.selectedSuffix = m.entries[m.idx].Suffix
            }
            return m, tea.Quit
        case "o":
            openDir(m.dir)
            m.msg = fmt.Sprintf("opened: %s", m.dir)
            return m, nil
        }
    }
    return m, nil
}

func (m RestoreModel) View() string {
    if m.quitting {
        return ""
    }
    styleSel := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
    header := "Restore state.vscdb from backup\n"
    header += fmt.Sprintf("Editor: %s\nDirectory: %s\n", m.editor, m.dir)
    header += "Close the editor before restoring!\n"
    header += "Use ↑/↓ or j/k to navigate, Enter to restore, o to open folder, q to quit.\n\n"
    body := ""
    for i, e := range m.entries {
        line := fmt.Sprintf("%s  %s  (%d bytes)", e.ModTime.Format("2006-01-02 15:04:05"), e.Suffix, e.Size)
        if i == m.idx {
            body += styleSel.Render("> " + line) + "\n"
        } else {
            body += "  " + line + "\n"
        }
    }
    if m.msg != "" { body += "\n" + m.msg + "\n" }
    return header + body
}

func (m RestoreModel) Selected() string { return m.selectedSuffix }

func openDir(dir string) {
    switch runtime.GOOS {
    case "darwin":
        _ = exec.Command("open", dir).Start()
    case "windows":
        _ = exec.Command("explorer", dir).Start()
    default:
        _ = exec.Command("xdg-open", dir).Start()
    }
}
