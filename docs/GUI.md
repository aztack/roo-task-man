# RooCode Task Manager — macOS SwiftUI GUI Brief

Goal
- Build a native macOS SwiftUI app that provides the same core capabilities as the CLI TUI:
  - Discover tasks for the configured VS Code extension (default `RooVeterinaryInc.roo-cline`) or forks/editors.
  - Browse task list with sorting and filtering.
  - Inspect task details, history, and attachments.
  - Export/import tasks (zip), delete tasks, open task folders.
  - Optional JS hooks (future parity) to extend rendering and discovery.

Key Requirements
- macOS 13+ target. Swift 5.9+. SwiftUI architecture (MVVM).
- No Vim keybindings; use standard macOS interactions (lists, search fields, menus, toolbars).
- Smooth, responsive UI. Task list scales to thousands of items.

App Structure
- AppState: Holds configuration (editor/channel, pluginId, dataDir, exportDir, hooksDir), task store, and settings.
- TaskStore: Discovers and loads tasks from globalStorage, builds models (id, title, summary, createdAt, path, meta).
- Views:
  - Sidebar + main content layout.
  - TaskListView: searchable/sortable list with 2-line rows (title + created/UID). Shows counts and quick actions (Export, Delete, Open Folder).
  - TaskDetailView: header with title/UID/time/path, sections (History, Extra Info), and actions (Export, Delete, Open Folder).
  - Import/Export Sheets: choose zip path(s) and confirm operations. Show progress bars.
  - PreferencesView: set plugin ID, editor/channel (Code/Insiders/VSCodium/Cursor/Windsurf/Trae/Custom), dataDir override, exportDir.

Data Model
- TaskModel: id, title, summary, createdAt, path, meta [String: Any].
- HistoryItemModel: timestamp, role (user/ai/other), kind, text (markdown), attachments.
- Manifest for export (v2): array of tasks with minimal metadata.

Discovery
- Same logic as CLI: resolve globalStorage per editor/channel and pluginId, search `<root>/tasks/*` fallback root dirs.
- Provide override via Preferences (dataDir) for custom installs.

Rendering
- List row: single-line title (truncate middle) + secondary line `created • UID`.
- Detail:
  - Render markdown content; consider using a native markdown renderer (Text with AttributedString markdown or third-party if needed).
  - History: user entries vs AI request entries (parse as JSON to show stats + request).
  - Provide search within detail with highlights.

User Flows
- Filter tasks by title/UID/created; toggle sort by created time.
- Export selected tasks to zip(s). Include editor + plugin in filename prefixes. Open the export folder after completion.
- Import zip(s) to current storage root; handle collisions (copy suffix or overwrite via option).
- Delete with confirmation.
- Open task folder in Finder.

Performance
- Lazy load task rows and detail content; avoid blocking the main thread.
- Use background queues for IO and zipping; show progress.

Testing
- Unit tests for discovery, parsing of ui_messages.json, and export/import manifest.
- UI tests for list filtering, sorting, selection, and detail display.

Extensibility
- Plan for optional JS hooks parity: a sandboxed JS engine to customize list row and detail sections. In the first iteration, focus on core UX.

Deliverables
- Xcode project with Swift Package Manager dependencies.
- Minimal app icon and signing setup for local runs.
- README for building and running the app.

