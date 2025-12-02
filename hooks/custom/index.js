// Hook scripts to enhance task display for a specific fork's data files.
// Loaded via --hooks-dir ./hooks/custom (requires building with -tags js_hooks).

// Minimal helper: safe JSON parse
function parseJSON(s) {
  try { return JSON.parse(s) } catch { return null }
}

function extendTask(task) {
  // If metamove_info.json exists, we can update title or meta
  const infoText = readText(task.path + "/metamove_info.json")
  const metaText = readText(task.path + "/task_metadata.json")
  /** @type {{ extensionVersion?: string, systemPrompts?: any[], createdAt?: number, updatedAt?: number, title?: string } | null} */
  const info = infoText ? parseJSON(infoText) : null
  const meta = metaText ? parseJSON(metaText) : null

  const out = { ...task }
  if (info) {
    out.meta = { ...(out.meta || {}), extensionVersion: info.extensionVersion, createdAtRaw: info.createdAt, updatedAtRaw: info.updatedAt }
    // If hook wants to set a better title, try to use the first user prompt trimmed
    if (!out.title && Array.isArray(info.systemPrompts) && info.systemPrompts.length > 0) {
      // no-op: keep existing
    }
  }
  if (meta && meta.files_in_context) {
    out.meta = { ...(out.meta || {}), filesInContext: Array.isArray(meta.files_in_context) ? meta.files_in_context.length : 0 }
  }
  return out
}

function renderTaskDetail(task) {
  const sections = []
  // Show basic info
  sections.push({ heading: "ID", body: task.id })
  // Try reading extra info files
  const infoText = readText(task.path + "/metamove_info.json")
  if (infoText) {
    const info = parseJSON(infoText)
    if (info) {
      const lines = []
      if (info.extensionVersion) lines.push(`Extension Version: ${info.extensionVersion}`)
      if (info.createdAt) lines.push(`Created: ${new Date(info.createdAt).toISOString()}`)
      if (info.updatedAt) lines.push(`Updated: ${new Date(info.updatedAt).toISOString()}`)
      if (Array.isArray(info.systemPrompts)) {
        lines.push("")
        lines.push("System Prompts:")
        for (const sp of info.systemPrompts) {
          if (sp && sp.systemPrompt) {
            lines.push("- " + String(sp.systemPrompt).slice(0, 200))
          }
        }
      }
      if (lines.length) sections.push({ heading: "Extra", body: lines.join("\n") })
    }
  }
  const metaText = readText(task.path + "/task_metadata.json")
  if (metaText) {
    const meta = parseJSON(metaText)
    if (meta && Array.isArray(meta.files_in_context)) {
      sections.push({ heading: "Files in Context", body: String(meta.files_in_context.length) })
    }
  }
  return { title: task.title || task.id, sections }
}

function decorateTaskRow(task) {
  // Show a compact title if available
  return task.title || task.id
}

function renderTaskListItem(task) {
  const title = task.title || task.summary || task.id
  const metamoveInfoStr = readText(task.path + "/metamove_info.json")
  if (!metamoveInfoStr) return { title, desc: `${new Date(task.createdAt).toLocaleString()} • ${task.id}` }
  const metamoveInfo = parseJSON(metamoveInfoStr);
  const extVersion = metamoveInfo.extensionVersion || "";
  const branch = metamoveInfo.gitInfo?.defaultBranch || "";
  const repo = metamoveInfo.gitInfo?.repositoryName || "";
  const repoUrl = metamoveInfo.gitInfo?.repositoryUrl || "";
  const desc = `${new Date(task.createdAt).toLocaleString()} • ${task.id} • ${extVersion} • ${branch} • ${repo} • ${repoUrl}`
  return { title, desc }
}