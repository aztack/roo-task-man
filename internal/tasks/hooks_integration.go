package tasks

import (
    "time"
    "log"

    "roocode-task-man/internal/config"
    "roocode-task-man/internal/hooks"
)

// LoadTasksWithHooks applies discovery and decoration hooks when available.
func LoadTasksWithHooks(cfg config.Config, env *hooks.HookEnv) ([]Task, error) {
    root, err := ResolveStorageRoot(cfg)
    if err != nil { return nil, err }

    // Discovery override
    var dirs []string
    if env != nil {
        if ss, ok := env.CallStringSlice("discoverCandidates", root); ok && len(ss) > 0 {
            log.Printf("[hooks] discoverCandidates returned %d dirs", len(ss))
            dirs = ss
        }
    }
    if len(dirs) == 0 {
        dirs = DiscoverTaskDirs(root)
    }

    list := BuildTasksFromDirs(dirs)
    // Extend/decorate
    for i := range list {
        t := &list[i]
        // extendTask: may augment fields
        if env != nil {
            m := taskToMap(*t)
            if out, ok := env.CallExported("extendTask", m); ok {
                log.Printf("[hooks] extendTask applied for %s", t.ID)
                if mm, ok2 := out.(map[string]any); ok2 {
                    *t = mapToTask(mm, *t)
                }
            }
            if s, ok := env.CallString("decorateTaskRow", m); ok && s != "" {
                log.Printf("[hooks] decorateTaskRow override for %s", t.ID)
                t.Title = s
            }
        }
    }
    return list, nil
}

func taskToMap(t Task) map[string]any {
    return map[string]any{
        "id":        t.ID,
        "title":     t.Title,
        "createdAt": t.CreatedAt.Format(time.RFC3339),
        "path":      t.Path,
        "meta":      t.Meta,
    }
}

func mapToTask(m map[string]any, base Task) Task {
    t := base
    if v, ok := m["id"].(string); ok && v != "" { t.ID = v }
    if v, ok := m["title"].(string); ok && v != "" { t.Title = v }
    if v, ok := m["summary"].(string); ok && v != "" { t.Summary = v }
    if v, ok := m["path"].(string); ok && v != "" { t.Path = v }
    if v, ok := m["meta"].(map[string]any); ok { t.Meta = v }
    if v, ok := m["createdAt"].(string); ok && v != "" {
        if tt, err := time.Parse(time.RFC3339, v); err == nil { t.CreatedAt = tt }
    }
    return t
}
