package tui

import (
    "fmt"
    "strings"
    "log"

    "roocode-task-man/internal/hooks"
    "roocode-task-man/internal/tasks"
)

// renderDetailMarkdown builds a markdown string that will be rendered for the viewport.
func renderDetailMarkdown(t tasks.Task, env *hooks.HookEnv, debug bool) string {
    b := &strings.Builder{}
    title := t.Title
    if title == "" { title = t.ID }
    fmt.Fprintf(b, "# %s\n\n", title)
    fmt.Fprintf(b, "- ID: `%s`\n", t.ID)
    fmt.Fprintf(b, "- Created: %s\n", humanTime(t.CreatedAt))
    if t.Path != "" { fmt.Fprintf(b, "- Path: `%s`\n", t.Path) }

    // Hook-provided sections
    if env != nil {
        mv := map[string]any{"id": t.ID, "title": t.Title, "createdAt": t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), "path": t.Path, "meta": t.Meta}
        if debug { log.Printf("[hooks] calling renderTaskDetail for %s", t.ID) }
        if out, ok := env.CallExported("renderTaskDetail", mv); ok {
            if m, ok2 := out.(map[string]any); ok2 {
                if debug { log.Printf("[hooks] renderTaskDetail returned sections for %s", t.ID) }
                if s, ok3 := m["title"].(string); ok3 && s != "" { fmt.Fprintf(b, "\n## %s\n\n", s) }
                if secs, ok3 := m["sections"].([]any); ok3 {
                    for _, sec := range secs {
                        if mm, ok4 := sec.(map[string]any); ok4 {
                            head, _ := mm["heading"].(string)
                            body, _ := mm["body"].(string)
                            if head != "" { fmt.Fprintf(b, "\n## %s\n\n", head) }
                            if body != "" { fmt.Fprintf(b, "%s\n\n", body) }
                        }
                    }
                }
            }
        } else if debug {
            log.Printf("[hooks] renderTaskDetail had no result for %s", t.ID)
        }
    }

    // History from ui_messages.json
    items := tasks.LoadHistory(t)
    if len(items) > 0 {
        fmt.Fprintf(b, "\n## History\n\n")
        for _, it := range items {
            // Entry heading with kind and timestamp
            label := it.Kind
            if it.Role == "user" { label = "🧑 User" }
            if it.Role == "ai" { label = "🤖 AI" }
            when := humanTime(it.At)
            if label != "" && when != "" { fmt.Fprintf(b, "### %s — %s\n\n", label, when) }
            if label != "" && when == "" { fmt.Fprintf(b, "### %s\n\n", label) }
            if label == "" && when != "" { fmt.Fprintf(b, "### %s\n\n", when) }
            if it.Text != "" { fmt.Fprintf(b, "%s\n\n", it.Text) }
        }
    }

    fmt.Fprintf(b, "\n(h) back  (q) quit  (e) export  (x) delete\n")
    return b.String()
}
