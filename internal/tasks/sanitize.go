package tasks

import (
    "encoding/json"
    "strings"
    "unicode/utf8"
)

// CleanOneLine converts input to a single line, removing fenced code blocks (``` … ```)
// and dropping the content entirely if it is a pure JSON object. If the result exceeds
// maxLen runes, it is truncated. It returns the cleaned text, and booleans indicating
// whether content was changed/removed and whether it was truncated.
func CleanOneLine(s string, maxLen int) (string, bool, bool) {
    orig := s
    changed := false

    // Remove fenced code blocks anywhere in the text.
    for {
        i := strings.Index(s, "```")
        if i < 0 { break }
        j := strings.Index(s[i+3:], "```")
        if j < 0 { // opening without closing → drop the rest
            s = s[:i]
            changed = true
            break
        }
        // Remove from i to end of closing fence
        s = s[:i] + s[i+3+j+3:]
        changed = true
    }

    // If remaining trimmed content is pure JSON object, drop it (leave empty one-liner)
    ss := strings.TrimSpace(s)
    if looksLikeJSON(ss) {
        var js any
        if json.Unmarshal([]byte(ss), &js) == nil {
            s = ""
            changed = true
        }
    }

    // Collapse to one line
    s = strings.ReplaceAll(s, "\r", " ")
    s = strings.ReplaceAll(s, "\n", " ")
    s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))

    truncated := false
    if maxLen > 0 {
        sRunes := []rune(s)
        if len(sRunes) > maxLen {
            s = string(sRunes[:maxLen]) + "…"
            truncated = true
        }
    }

    if s != orig { changed = true }
    return s, changed, truncated
}

func looksLikeJSON(s string) bool {
    if s == "" { return false }
    // quick checks: must start with '{' and end with '}' and be valid UTF-8
    if !utf8.ValidString(s) { return false }
    s = strings.TrimSpace(s)
    if len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}' {
        return true
    }
    return false
}

