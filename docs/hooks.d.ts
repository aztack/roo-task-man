// RooCode Task Manager Hook Types

export interface Task {
  id: string;
  title: string;
  summary?: string;
  createdAt: string; // ISO 8601
  path: string;
  meta?: Record<string, unknown>;
}

export interface DetailSection {
  heading: string;
  body: string;
}

export interface DetailView {
  title: string;
  sections: DetailSection[];
}

// Minimal helper provided by runtime when built with js_hooks
export function readText(path: string): string | null;

// Hook points
export function decorateTaskRow(task: Task): string;
export function extendTask(task: Task): Task;
export function renderTaskDetail(task: Task): DetailView;
export function discoverCandidates(root: string): string[];
export function renderTaskListItem(task: Task): { title: string; desc?: string } | undefined;
// For fork-specific extra fields rendered into the list/detail, use extendTask to enrich task.meta
// Example consumers may read fork JSON files and populate meta fields consumed by renderers.
