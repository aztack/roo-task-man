package hooks

import (
    "log"
    "os"
    "path/filepath"
    "strings"

    "github.com/dop251/goja"
)

type HookEnv struct{ rt *goja.Runtime }

func LoadDir(dir string) (*HookEnv, error) {
    env := &HookEnv{rt: goja.New()}
    // expose minimal FS read helper
    env.rt.Set("readText", func(call goja.FunctionCall) goja.Value {
        if len(call.Arguments) < 1 { return goja.Undefined() }
        p := call.Arguments[0].String()
        b, err := os.ReadFile(p)
        if err != nil { return goja.Null() }
        return env.rt.ToValue(string(b))
    })
    if dir == "" { return env, nil }
    entries, err := os.ReadDir(dir)
    if err != nil { return env, nil }
    for _, e := range entries {
        if e.IsDir() { continue }
        if filepath.Ext(e.Name()) != ".js" { continue }
        b, err := os.ReadFile(filepath.Join(dir, e.Name()))
        if err != nil { continue }
        code := string(b)
        // strip simple ESM exports
        code = strings.ReplaceAll(code, "export function ", "function ")
        code = strings.ReplaceAll(code, "export const ", "const ")
        code = strings.ReplaceAll(code, "export let ", "let ")
        code = strings.ReplaceAll(code, "export var ", "var ")
        if _, err := env.rt.RunString(code); err != nil {
            log.Printf("[hooks] error evaluating %s: %v", e.Name(), err)
        } else {
            log.Printf("[hooks] loaded %s", e.Name())
        }
    }
    for _, name := range []string{"renderTaskListItem","renderTaskDetail","extendTask","decorateTaskRow","discoverCandidates"} {
        v := env.rt.Get(name)
        if !goja.IsUndefined(v) && !goja.IsNull(v) {
            log.Printf("[hooks] function available: %s", name)
        }
    }
    return env, nil
}

func (h *HookEnv) Call(fn string, arg any) (goja.Value, bool) {
    if h == nil || h.rt == nil { return goja.Undefined(), false }
    v := h.rt.Get(fn)
    if goja.IsUndefined(v) || goja.IsNull(v) {
        log.Printf("[hooks] function not found: %s", fn)
        return goja.Undefined(), false
    }
    if f, ok := goja.AssertFunction(v); ok {
        rv, err := f(goja.Undefined(), h.rt.ToValue(arg))
        if err != nil {
            log.Printf("[hooks] error calling %s: %v", fn, err)
            return goja.Undefined(), false
        }
        log.Printf("[hooks] %s returned: %#v", fn, rv.Export())
        return rv, true
    }
    log.Printf("[hooks] symbol is not a function: %s", fn)
    return goja.Undefined(), false
}

func (h *HookEnv) CallString(fn string, arg any) (string, bool) {
    if rv, ok := h.Call(fn, arg); ok { return rv.String(), true }
    return "", false
}

func (h *HookEnv) CallExported(fn string, arg any) (any, bool) {
    if rv, ok := h.Call(fn, arg); ok { return rv.Export(), true }
    return nil, false
}

func (h *HookEnv) CallStringSlice(fn string, arg any) ([]string, bool) {
    if rv, ok := h.Call(fn, arg); ok {
        if arr, ok2 := rv.Export().([]any); ok2 {
            out := make([]string, 0, len(arr))
            for _, v := range arr {
                if s, ok3 := v.(string); ok3 { out = append(out, s) }
            }
            return out, true
        }
    }
    return nil, false
}
