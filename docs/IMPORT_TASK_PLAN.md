# Plan: Import compressed task into workspace

- Goal: After importing a task archive into the editor storage, register it into the extension’s global state so it appears in recent task history for the chosen workspace.

## Inputs
- `--import <zip>`: path to task archive (single or multi) [already supported].
- `--editor` / `--code-channel`: target editor channel (Code, Insiders, Cursor, Windsurf, Trae, Custom).
- `--plugin-id`: extension ID (defaults to config).
- `--workspace <path>`: explicit workspace folder to associate with the imported task (new flag to add). Use current working folder if not specified.
- Optional: `--data-dir` when `--editor=Custom`.

## High-level Steps
1) Resolve storage root
- Use `internal/tasks.ResolveStorageRoot(cfg)` to determine `<globalStorage>/<plugin-id>`.
- Just exit if `<globalStorage>/<plugin-id>` does not exist.

2) Import archive contents
- Use `zipper.ImportAny(zip, destRoot)` [already implemented].
- Result: task directory extracted under `<destRoot>/tasks/<task-id>` (or `<destRoot>/<task-id>` fallback).

3) Locate global state DB
- Editor global state DB (SQLite):
  - macOS: `~/Library/Application Support/<Editor>/User/globalStorage/state.vscdb`.
  - Linux: `~/.config/<Editor>/User/globalStorage/state.vscdb`.
  - Windows: `%APPDATA%/<Editor>/User/globalStorage/state.vscdb`.
- We already derive `<Editor>` via `DisplayEditorName`/`mapEditorChannel`.

4) Backup + open DB (safe writes)
- Create a timestamped copy of `state.vscdb` next to the original before modification.
- Open SQLite in read-write; begin transaction.

5) Read extension settings row
- Table: `ItemTable` with `key` and `value` columns (JSON string in `value`).
- Row key: `<plugin-id>`.
- Parse JSON into a map-like struct.

6) Compute new TaskHistory entry
- Fields (see docs/IMPORT_TASK.md):
  - `id`: task UID (directory name).
  - `number`: last history `number` + 1 (scan existing `settings.taskHistory`).
  - `ts`: task create timestamp in ms (from manifest `createdAt` or earliest mtime of task dir).
  - `task`: first user prompt or summary (`internal/tasks.readSummary`).
  - `tokensIn`, `tokensOut`, `cacheWrites`, `cacheReads`, `totalCost`:
    - Prefer a `stats.json` if present; else derive from the last AI request entry in `ui_messages.json` (parse JSON payload we already parse in `LoadHistory`).
    - Fallback to zeros when unavailable.
  - `size`: on-disk bytes of the task directory (recursive sum).
  - `workspace`: `--workspace` path (mandatory for this workflow).

7) Write back settings
- Update `settings.taskHistory` array by appending the new entry.
- Serialize JSON and update the row for `<plugin-id>`.
- Commit transaction.

8) Validation and output
- Re-open and verify the row contains the appended entry.
- Print success with task id, number, and workspace; on failure, print a clear error and reference the backup DB path.

## Edge Cases
- Missing or corrupt `state.vscdb`: Just fail with message.
- No existing `<plugin-id>` settings row: Just fail with message.
- Colliding task directory: we already suffix `-copy-<timestamp>` on import.
- Unreadable `ui_messages.json`: proceed with defaults and log a warning when `--debug`.

## CLI/Code Changes to Add (follow-up PR)
- Add `--workspace` string flag to CLI; pass through config/context for the import registration step.
- Implement `internal/tasks/statevscdb` helper:
  - Small wrapper to read/write the JSON blob for `<plugin-id>` and update `settings.taskHistory`.
  - Utilities: `DetectEditorStateDB(cfg)`, `Backup(path)`, `OpenRW(path)`, `GetSettings(plugin)`, `UpsertSettings(plugin, value)`.
- Extend `--import` flow:
  - After `zipper.ImportAny`, if `--workspace` provided, perform steps 3–7.
  - Otherwise, skip DB mutation and only extract files (current behavior).

## Testing
- Unit tests (Go `testing`):
  - Fake temp SQLite DB with a minimal `ItemTable`; verify upsert and history append.
  - Parse `ui_messages.json` samples to extract stats; verify fallback logic.
- Manual test:
  - Use `--editor Code` and a throwaway profile (custom `--data-dir`) to avoid touching real VS Code data.
  - Import a known archive; verify history entry appears in settings and downstream UI.

## Rollback Strategy
- Rely on pre-write backup of `state.vscdb`.
- Provide a `--no-backup` flag only if needed (default to backup enabled).

## Notes
- All operation will not try to create non-existing files/folders and not corrupt existing database. Just fail with clear error messages.
- All file operations must validate paths; avoid following symlinks outside expected roots.
- Be mindful of concurrent editor access to `state.vscdb`; prefer quick transactions and retries.
