# RooCode Task Manager — Product Requirements

## Overview

RooCode Task Manager (CLI + TUI) is a cross‑platform terminal app built with Go and Charmbracelet Bubble Tea. It helps developers browse, inspect, export, import, and manage “tasks” created by the RooCode VS Code extension and its forks. The app provides a fast, keyboard‑centric experience with Vim key bindings and supports customization via JavaScript hook scripts to adapt to forked extensions’ storage formats.

## Goals

- List RooCode tasks with meaningful titles and created timestamps.
- Explore a task’s details in a navigable TUI with Vim keys.
- Export tasks to a zip archive for sharing; import from zip.
- Delete tasks with confirmation/alert.
- Configure the extension plugin ID via CLI flags and a config file (`~/.config/roo-code-man.json`).
- Allow custom exploration via JavaScript hook scripts with provided types for better DX.

## Non‑Goals (Initial Version)

- Full parity with every fork’s proprietary structure. Instead, provide extensible hooks and detection patterns that can be refined.
- Rich diffing or code intelligence within task contents.

## Users

- Developers using RooCode or its forks who want a quick way to browse and manage task artifacts outside VS Code.
- Teams that exchange tasks as self‑contained archives.

## Platforms

- macOS, Linux, Windows (terminal environments).

## Data Model (Conceptual)

- Task
  - id: string (derived from directory name or metadata)
  - title: string (metadata or best‑effort from contents; customizable via hooks)
  - createdAt: time (from metadata or filesystem timestamps)
  - path: string (absolute path to task directory)
  - meta: map[string]any (optional metadata loaded from known files)

## Storage Discovery

By default, tasks are assumed to live under VS Code’s `globalStorage` area for a given plugin ID. Defaults and detection rules:

- Plugin ID (default): `RooVeterinaryInc.roo-cline`
- VS Code user data roots (Editor app folder varies by fork):
  - macOS: `~/Library/Application Support/Code/User/globalStorage/<pluginId>`
  - Linux: `~/.config/Code/User/globalStorage/<pluginId>`
  - Windows: `%APPDATA%/Code/User/globalStorage/<pluginId>`
- Supported editors via `--code-channel`/`--editor` and config `codeChannel`:
  - `Code` (default), `Insiders` (`Code - Insiders`), `VSCodium`, `Cursor`, `Windsurf`, `Trae`, or custom app dir name.
  - `Custom` requires `dataDir` override pointing to the globalStorage root.
- Variants (Insiders/VSCodium) can be configured via config/flags or overriding the data directory.
- Task directory candidates:
  - `<root>/tasks/*` (directory per task)
  - Fallback: scan `<root>` for directories with recognizable metadata (e.g., `task.json`, `metadata.json`) — customizable via hooks.

## Configuration

- File: `~/.config/roo-code-man.json`
  - Example:
    {
      "pluginId": "RooVeterinaryInc.roo-cline",
      "codeChannel": "Code", // Code | Insiders | VSCodium | Cursor | Windsurf | Trae | Custom
      "dataDir": "",          // optional override path to root
      "hooksDir": "",         // optional directory with .js hook files
      "exportDir": "",        // default directory for TUI exports
      "debug": false
    }
- CLI flags override config file values.

## TUI UX Requirements

### List View

- Two-line rows with keyboard navigation.
- Default rendering:
  - Title line: single-line title (sanitized from summary when available).
  - Second line: `created time • task UID`.
- Sorting: latest-first by default; toggle asc/desc with `S` (title shows `[sort:…]`).
- Filtering: type to search by title, UID, created time, and user prompts corpus.
  - Strict tokens: `-uid=<part>` and `-d=<part-of-created-time>` pre-filter the task set before fuzzy matching.
- Selection: `Tab`/`Space` toggle, `C` clear; selection works even when filtering.
  - While filtering, one-key item shortcuts are ignored (Esc to clear filter first).
- Actions: `e` export current, `E` export selected, `o` open task folder, `x` delete (confirm), `r` refresh, `?` help.
- Navigation: `j/k`, `g/G`, PgDown/Ctrl+f/Ctrl+d, PgUp/Ctrl+b/Ctrl+u.
- Hooks may customize list item rendering via `renderTaskListItem(task)`.

### Detail View

- Markdown-rendered details with paging.
- Actions shown at the top: back, open dir, export, delete, search.
- Search `/` with highlight; `n/N` next/prev match.
- History from `ui_messages.json` with roles (🧑 user, 🤖 AI).
- Jump entries `J/K`, by role `]`/`[`, `}`/`{`.
- Integrates hook outputs as additional sections.

### Alerts/Confirmation

- Deleting prompts: “Delete task <title>? (y/N)”
- Errors are displayed inline with a minimal notification bar.

## CLI Usage

- `roo-task-man` (starts TUI using config + defaults)
- Flags (apply to TUI and batch operations):
  - `--plugin-id <id>`
  - `--code-channel <Editor>` or `--editor <Editor>` (Code, Insiders, VSCodium, Cursor, Windsurf, Trae, Custom, or app dir name)
  - `--data-dir <path>` (overrides discovery)
  - `--config <path>` (defaults to `~/.config/roo-code-man.json`)
  - `--export <task-id>:<zip-path>` (batch export then exit)
  - `--import <zip-path>` (batch import then exit)
  - `--inspect <zip-path>` (extract and open TUI on the archive)
  - `--export-dir <path>` (default directory for TUI exports; default CWD)
  - `--debug` (print debug info and hook logs)
  - `--version` (print version and exit)

## Export/Import

- Export packs a task directory into a zip. The archive includes all files under that task dir and a manifest file (`roo-task-manifest.json`) with minimal metadata (id, title, createdAt, pluginId, source path optional).
- Multi-select export packs multiple tasks into a single zip (v2 manifest) and stores files under `<id>/...`.
- Import supports both single-task and multi-task archives; it restores into the detected root (ID preserved; collision policy unchanged).

## Hooks (JavaScript)

Purpose: allow extensions/forks to change how tasks are discovered, titled, and displayed.

- Location: configurable `hooksDir` (default `~/.config/roo-code-man/hooks`). All `.js` files in the dir are loaded.
- Runtime: Goja (embedded JS engine). Hooks are built-in by default (no build tags).
- Provided types (for DX): shipped as `docs/hooks.d.ts`.

Hook points (initial):

- `decorateTaskRow(task): string` — return a title/label for list rows.
- `extendTask(task): Task` — mutate/augment metadata after discovery.
- `renderTaskDetail(task): { title: string; sections: Array<{ heading: string; body: string }>; }` — add custom sections to detail view.
- `discoverCandidates(root: string): string[]` — optionally override how task directories are discovered.

Constraints:

- Hooks must be fast and non‑blocking; heavy I/O is discouraged. Timeouts may be enforced.
- Errors in hooks should degrade gracefully with warnings but not crash the TUI.

## Deletion Policy

- Deleting a task removes its directory recursively after confirmation. No recycle bin is guaranteed; the operation is destructive unless OS‑level recovery is available. Provide a strong confirmation prompt.

## Telemetry and Privacy

- No network calls by default. Everything runs locally.
- Import/export artifacts remain local; users decide how to share them.

## Performance

- Open list view with up to thousands of tasks in under 200ms on modern hardware.
- Zip/unzip operations stream efficiently and show minimal progress feedback (future improvement: progress bar).

## Error Handling

- Clear messages for missing storage directory, permission issues, malformed archives.
- On Windows/macOS/Linux, path normalization and user home resolution must be robust.

## Testing Approach (High Level)

- Unit tests for discovery logic, config parsing, and zipping.
- Integration tests for import/export with temp dirs.
- Snapshot tests for hook outputs (optional).

## Milestones

1. Bootstrap CLI + TUI with static list and config parsing.
2. Implement discovery for default plugin ID and channels.
3. Wire delete with confirmation, export/import.
4. Add hooks runtime and types, integrate decorate/detail hooks.
5. Polish UX (filtering/help), add tests.
