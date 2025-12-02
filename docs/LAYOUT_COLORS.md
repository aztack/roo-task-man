# Task List Layout & Colors

This document locks down the current list layout and color scheme used by the TUI. It is intentionally simple and legible in both light and dark terminals.

## Layout

- 2‑panel list view:
  - Left: `Tasks` — the current task list (filterable). Shows title on the first line, and a dim second line with `created • id`.
  - Right: `Prompts` — human prompts extracted from the selected task’s history (`ui_messages.json`, entries with `images` present are treated as user prompts). It updates when the left selection changes.
  - Panels split 50/50 by default and resize with the terminal. Each panel scrolls independently inside its component.

- Detail view:
  - Enter opens a Markdown detail viewport for the selected task. Use the existing navigation keys (J/K, [/], {/}, /, n/N, etc.).

## Colors

- Selection prefix: green `[x]` on selected items in the left list.
- Hook badge: yellow `[H]` prefix on titles overridden by hooks.
- Help text: subtle gray (adaptive) and bolded for readability.
- Search highlight (detail view): magenta, bold.

Rationale: defer most styling to Bubble Tea defaults for wide compatibility; only apply minimal, high‑contrast accents for state. This keeps the interface readable across terminals and themes.

## Behaviors

- Filtering: performed on the left `Tasks` list only. Search matches title, ID, created time, and the user prompts corpus. Right `Prompts` list reflects the currently selected task after filtering.
- Title & prompt lines are single‑line only (newlines collapsed to spaces).
- Sorting toggle (`S`) applies to the left `Tasks` list; right `Prompts` shows prompts in historical order.
