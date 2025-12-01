# Known Issues / TODOs

- Selection while filtering on some terminals can still be inconsistent with `Space` due to IME or terminal input handling. Workaround: use `Tab` to toggle selection reliably. We persist selection by task ID across filters, but there may be rare cases where focus and selection drift; needs more robust state sync.

- Hook loader is minimal and strips `export` keywords, but does not support `export default`, `import` statements, or multi-file module semantics. It also lacks CommonJS `require`. Consider adding a tiny module loader and better error surfacing (stack traces) for hook exceptions.

- Hook diagnostics: While `--debug` prints function availability, inputs, and return payloads, there is no on-screen badge per-item indicating that a specific item was transformed via hooks. Consider a small symbol `[H]` per-row when a hook override is applied.

- Filtering tokens: `-d=<part-of-created-time>` matches the rendered local time string with a simple substring; there is no timezone-aware parsing or range queries. Potential enhancements: `-d>=YYYY-MM-DD`, `-d<=...`, or explicit timezone formatting.

- Inspect mode (`--inspect <zip>`): Extracts to a temporary directory for viewing. Edits in inspect mode do not propagate back to the original archive. Consider read-only UI affordances or an explicit "export modifications" flow.

- Export progress: Current implementation shows simple status messages and opens the folder on completion. No granular progress bar. Consider streaming progress with file counts/sizes.

- Theme/accessibility: Help (`?`) is styled slightly brighter, but there is no theme toggle or configurable colors yet. Consider a configuration for color themes and high-contrast mode.

- Tests: We currently have unit tests for discovery and zipper. There are no tests for TUI behavior or JS hooks evaluation. Consider adding a small headless harness for hooks and snapshot tests for list rendering.

