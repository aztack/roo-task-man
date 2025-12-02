package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
    PluginID   string `json:"pluginId"`
    CodeChannel string `json:"codeChannel"` // Code | Insiders | VSCodium | Custom
    DataDir    string `json:"dataDir"`      // optional override to globalStorage root
    HooksDir   string `json:"hooksDir"`
    ExportDir  string `json:"exportDir"`    // default export destination directory
    Debug      bool   `json:"debug"`
}

func Default() Config {
    return Config{
        PluginID:    "RooVeterinaryInc.roo-cline",
        CodeChannel: "Code",
        DataDir:     "",
        HooksDir:    filepath.Join(UserHome(), ".config", "roo-code-man", "hooks"),
        // CWD by default; app will fallback to "." when empty
        ExportDir:   "",
        Debug:       false,
    }
}

func Load(path string, out *Config) error {
    b, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    var c Config
    if err := json.Unmarshal(b, &c); err != nil {
        return err
    }
    if c.PluginID == "" {
        c.PluginID = out.PluginID
    }
    if c.CodeChannel == "" {
        c.CodeChannel = out.CodeChannel
    }
    if c.HooksDir == "" {
        c.HooksDir = out.HooksDir
    }
    *out = c
    return nil
}

func Save(path string, c Config) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    b, err := json.MarshalIndent(c, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, b, 0o644)
}

func UserHome() string {
    if h, err := os.UserHomeDir(); err == nil {
        return h
    }
    if runtime.GOOS == "windows" {
        if h := os.Getenv("USERPROFILE"); h != "" {
            return h
        }
    }
    return "."
}

func EnsureDir(path string) error {
    if path == "" {
        return errors.New("empty path")
    }
    return os.MkdirAll(path, 0o755)
}

// DownloadsDir returns the user's default Downloads folder.
func DownloadsDir() string {
    home := UserHome()
    switch runtime.GOOS {
    case "windows":
        if p := os.Getenv("USERPROFILE"); p != "" {
            return filepath.Join(p, "Downloads")
        }
        return filepath.Join(home, "Downloads")
    default:
        return filepath.Join(home, "Downloads")
    }
}
