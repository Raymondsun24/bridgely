package com.raymondsun24.bridgely

data class ActiveFile(
    val path: String,
    val relativePath: String,
    val languageId: String,
    val lineCount: Int,
    val cursorLine: Int,
    val cursorColumn: Int,
)

data class SelectionInfo(
    val text: String,
    val startLine: Int,
    val startColumn: Int,
    val endLine: Int,
    val endColumn: Int,
    val filePath: String,
)

data class VisibleFile(
    val path: String,
    val isActive: Boolean,
    val isDirty: Boolean,
    val viewColumn: Int,
)

data class WorkspaceInfo(
    val folders: List<String>,
    val name: String,
)

data class EditorState(
    val version: Int = 1,
    val timestamp: Long,
    val pid: Long,
    val sessionId: String,
    val ideName: String,
    val workspace: WorkspaceInfo,
    val activeFile: ActiveFile?,
    val selection: SelectionInfo?,
    val visibleFiles: List<VisibleFile>,
)

/** Incoming command written by the MCP server or CLI. */
data class BridgeCommand(
    val version: Int,
    val id: String,
    val timestamp: Long,
    val command: String,
    val args: Map<String, Any?>,
)

data class CommandResultPayload(
    val message: String,
    val data: Any? = null,
)

/** Outgoing result written by the plugin after executing a command. */
data class CommandResult(
    val version: Int = 1,
    val id: String,
    val timestamp: Long,
    val status: String,   // "ok" | "error"
    val result: CommandResultPayload,
)
