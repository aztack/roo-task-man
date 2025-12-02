package tasks

import (
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    "roocode-task-man/internal/config"
)

// DumpMarkdown writes all tasks and their human prompts to a markdown file.
// The title and each prompt are constrained to a single line.
func DumpMarkdown(cfg config.Config, filename string) error {
    return DumpMarkdownWithProgress(cfg, filename, nil)
}

// DumpMarkdownWithProgress is like DumpMarkdown but calls progress(cur,total) as it proceeds.
func DumpMarkdownWithProgress(cfg config.Config, filename string, progress func(int, int)) error {
    list, err := LoadTasks(cfg)
    if err != nil { return err }
    f, err := os.Create(filename)
    if err != nil { return err }
    defer f.Close()
    return writeMarkdownWithProgress(cfg, list, f, progress)
}

func writeMarkdownWithProgress(cfg config.Config, list []Task, w io.Writer, progress func(int, int)) error {
    const maxTitle = 120
    const maxPrompt = 120
    total := len(list)
    for i, t := range list {
        tFull := strings.TrimSpace(t.Title)
        title, changed, truncated := CleanOneLine(tFull, maxTitle)
        if title == "" { title = t.ID }
        fmt.Fprintf(w, "# %s\n\n", title)
        fmt.Fprintf(w, "- ID: %s\n", t.ID)
        // RFC3339 in local time for readability and timezone awareness
        if !t.CreatedAt.IsZero() {
            fmt.Fprintf(w, "- Created: %s\n", t.CreatedAt.Local().Format(time.RFC3339))
        } else {
            fmt.Fprintf(w, "- Created: %s\n", time.Now().Local().Format(time.RFC3339))
        }
        if t.Path != "" { fmt.Fprintf(w, "- Path: %s\n\n", t.Path) } else { fmt.Fprintf(w, "\n") }

        // If title was changed or truncated, include full content in a details block
        if changed || truncated {
            fmt.Fprintf(w, "\n<details><summary>%s</summary>\n\n", escapeHTML(title))
            fmt.Fprintf(w, "\n```\n%s\n```\n\n", tFull)
            fmt.Fprintln(w, "</details>\n\n")
        }

        // Prompts (user role only)
        hist := LoadHistory(t)
        prompts := 0
        for _, h := range hist { if h.Role == "user" && oneLine(h.Text) != "" { prompts++ } }
        if prompts > 0 {
            fmt.Fprintln(w, "## Prompts")
            for _, h := range hist {
                if h.Role != "user" { continue }
                full := strings.TrimSpace(h.Text)
                p, pChanged, pTrunc := CleanOneLine(full, maxPrompt)
                if p == "" { continue }
                fmt.Fprintf(w, "- %s\n", p)
                if pChanged || pTrunc {
                    fmt.Fprintf(w, "\n<details><summary>%s</summary>\n\n", escapeHTML(p))
                    fmt.Fprintf(w, "\n```\n%s\n```\n\n", full)
                    fmt.Fprintln(w, "</details>\n\n")
                }
            }
        }
        // Separator
        if i != len(list)-1 { fmt.Fprintln(w, "---\n") } else { fmt.Fprintln(w) }

        if progress != nil { progress(i+1, total) }
    }
    return nil
}

func oneLine(s string) string { out, _, _ := CleanOneLine(s, 0); return out }

// Minimal HTML escaping for <summary> text
func escapeHTML(s string) string {
    r := strings.NewReplacer(
        "&", "&amp;",
        "<", "&lt;",
        ">", "&gt;",
        "\"", "&quot;",
        "'", "&#39;",
    )
    return r.Replace(s)
}
