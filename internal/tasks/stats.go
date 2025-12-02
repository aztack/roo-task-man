package tasks

import (
    "encoding/json"
    "io/fs"
    "os"
    "path/filepath"
)

// TaskStats represents aggregate metrics parsed from a task directory.
type TaskStats struct {
    TokensIn    int
    TokensOut   int
    CacheReads  int
    CacheWrites int
    TotalCost   float64
    SizeBytes   int64
}

// StatsFromTask attempts to read metrics from ui_messages.json. Falls back to zeros.
func StatsFromTask(t Task) TaskStats {
    var st TaskStats
    // compute size first
    st.SizeBytes = dirSize(t.Path)
    // parse ui_messages.json for last AI stats JSON payload
    p := filepath.Join(t.Path, "ui_messages.json")
    b, err := os.ReadFile(p)
    if err != nil { return st }
    type raw struct {
        Text string `json:"text"`
        Images any   `json:"images"`
    }
    var arr []raw
    if err := json.Unmarshal(b, &arr); err != nil { return st }
    // iterate reversed to find last AI request (where Text contains JSON)
    for i := len(arr) - 1; i >= 0; i-- {
        if arr[i].Images != nil { continue } // skip user
        var ai struct {
            APIProtocol string  `json:"apiProtocol"`
            Costs       float64 `json:"costs"`
            Request     string  `json:"request"`
            TokenIn     int     `json:"tokenIn"`
            TokenOut    int     `json:"tokenOut"`
            CacheReads  int     `json:"cacheReads"`
            CacheWrites int     `json:"cacheWrites"`
        }
        if json.Unmarshal([]byte(arr[i].Text), &ai) == nil && (ai.TokenIn > 0 || ai.TokenOut > 0 || ai.Costs > 0) {
            st.TokensIn = ai.TokenIn
            st.TokensOut = ai.TokenOut
            st.CacheReads = ai.CacheReads
            st.CacheWrites = ai.CacheWrites
            st.TotalCost = ai.Costs
            break
        }
    }
    return st
}

func dirSize(root string) int64 {
    var n int64
    _ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return nil }
        if d.IsDir() { return nil }
        if info, err := d.Info(); err == nil { n += info.Size() }
        return nil
    })
    return n
}

