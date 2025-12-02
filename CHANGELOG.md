# Changelog

All notable changes to this project will be documented in this file.

## v0.1.1 — 2025-12-02

- CLI export without TUI
  - Added `--taskiids` (comma-separated) and `--date-range=from..to` filters.
  - Support exporting multiple tasks with `--export <zip>` + filters (union semantics).
  - Date formats: `YYYY-MM-DD` or `YYYYMMDD` (inclusive).
- Import improvements
  - `--import` now defaults `--workspace` to current working directory if not provided.
  - Registers imported tasks into editor state DBs (`state.vscdb` and `state.vscdb.backup`) with fields: `id`, `number`, `ts`, `task`, `tokensIn`, `tokensOut`, `cacheReads`, `cacheWrites`, `totalCost`, `size`, `workspace`, `mode` (set to `code`).
  - Sets `number`, `tokensIn`, `tokensOut`, and `totalCost` to `1` as requested.
- Import/export safety
  - Skips symlinks on import.
  - Skips extraction if task directory already exists
- Restore state.vscdb from backups
  - New `--restore` interactive TUI to restore `state.vscdb` and paired `state.vscdb.backup` from backups (`state.vscdb.bak-<suffix>`), with Vim/arrow navigation and `o` to open folder.
  - Works even if original DB files are missing; prompts to close the editor first.
  - Backup file name suffixes are consistent across primary and backup DBs.

  - Default export filename rules when `--taskids` is used without `--export`.
  - `[hooks]` logs now respect `--debug` (quiet by default).

## v0.1.0 — 2025-12-01

- Initial release of RooCode Task Manager (CLI + TUI)
  - Browse, export, import, and delete tasks via Bubble Tea TUI.
  - Basic batch export/import and zip inspection.
