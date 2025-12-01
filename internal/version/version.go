package version

var (
    // Version is set via ldflags at build time. Fallback to dev.
    Version = "dev"
    // Commit is the VCS revision, set via ldflags.
    Commit  = ""
    // Date is the build timestamp in RFC3339, set via ldflags.
    Date    = ""
)

func String() string {
    s := Version
    if Commit != "" { s += "+" + Commit }
    if Date != "" { s += " (" + Date + ")" }
    return s
}

