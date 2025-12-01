# Repository Guidelines

## Project Structure & Module Organization
- Root Go module: `roocode-task-man` (Go 1.23).
- CLI entrypoint: `cmd/roo-task-man/main.go` (binary: `roo-task-man`).
- Internal packages: `internal/{config,hooks,tasks,tui,zipper}`.
- Documentation: `docs/` (e.g., `docs/GUI.md`, `docs/PRD.md`).
- Optional runtime hooks: `hooks/` (e.g., `hooks/custom/`).
- Assets/screens: `screenshot/`.

## Build, Test, and Development Commands
- `make build` — Build the CLI with JS hook support (`-tags js_hooks`). Output: `./roo-task-man`.
- `make run` — Build then run the TUI locally.
- `make test` — Run Go tests across all packages.
- `make tidy` — Sync and prune `go.mod`/`go.sum`.
- `make clean` — Remove the built binary.

Example run with flags:
- `./roo-task-man --config ~/.config/roo-code-man.json --editor Code`
- Batch: `./roo-task-man --export <task-id>:/path/to/out.zip` or `--import /path/to/in.zip`
- Inspect zip: `./roo-task-man --inspect /path/to/archive.zip`

## Coding Style & Naming Conventions
- Use `gofmt` defaults (tabs, standard line width). Run `gofmt -w .` before committing.
- Run `go vet ./...` for static checks.
- Naming: packages lower-case; exported identifiers `CamelCase`; tests end with `_test.go`.
- Keep modules cohesive: cross-package code belongs under `internal/` subpackages.

## Testing Guidelines
- Framework: Go standard `testing`.
- Name tests `TestXxx` in `*_test.go` files; table-driven where practical.
- Commands: `go test ./... -race -cover` for local verification.
- Aim for meaningful coverage of `internal/tasks`, `internal/zipper`, and TUI model logic.

## Commit & Pull Request Guidelines
- Commits: Prefer Conventional Commits (e.g., `feat(tui): add export prompt`, `fix(tasks): handle missing ID`).
- PRs: include description, rationale, and linked issues; show CLI examples; attach screenshots from `screenshot/` when UI/TUI changes.
- Update docs in `docs/` when flags, behavior, or user flows change.

## Security & Configuration Tips
- Default config: `~/.config/roo-code-man.json`. Override with `--config`.
- Use `--hooks-dir` to load trusted JavaScript hooks; review hook code in `hooks/` before use.
- When exporting/importing archives, validate paths and handle sensitive data carefully.
