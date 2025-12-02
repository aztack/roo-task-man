# Import compressed task into workspace

## Step 1
- Determine full path of workspace where the task will be imported
- Open the 'global state' sqlite3 database
  - MacOS: `/Users/<username>/Library/Application Support/<editor>/User/globalStorage/state.vscdb`
  - Windows: ?? You need to find the path on your own
- Get settings for extension with <extension-id> as key
- Prepare data for inserting a task record in settings.taskHistory
  - User task UID as id
  - Increment the number of the last task record in the history by 1
  - Use task create time stamp as ts
  - Use task tokensIn, tokensOut, cacheWrites, cacheReads, totalCost, size as they are
  - Use current workspace as the workspace path
- Print success/failed message to user

Here is the TaskHistory type definition:
```ts
type TaskHistory = Array<{
  "id": string, // "da8895bf-19df-4946-81d7-c9c18f00091a",
  "number": number, // 1,
  "ts": number, // 1754275108849,
  "task": string, // "load memory from @/tools/pivot-mcp/docs/memory.md  and continue",
  "tokensIn": number, // 204,
  "tokensOut": number, // 31106,
  "cacheWrites": number, // 555665,
  "cacheReads": number, // 3177668,
  "totalCost": number, // 3.5042461499999993,
  "size": number, // 6435871,
  "workspace": string // "/Users/wangweihua/closesource/byted/growth/pivot"
}>