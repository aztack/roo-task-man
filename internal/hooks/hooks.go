package hooks

// Default stub implementation (no JS runtime) to ensure builds succeed
// without the optional js_hooks tag.

type HookEnv struct{}

func LoadDir(dir string) (*HookEnv, error) { return &HookEnv{}, nil }

func (h *HookEnv) CallString(fn string, arg any) (string, bool) { return "", false }

func (h *HookEnv) CallExported(fn string, arg any) (any, bool) { return nil, false }

func (h *HookEnv) CallStringSlice(fn string, arg any) ([]string, bool) { return nil, false }
