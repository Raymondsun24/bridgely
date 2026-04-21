package com.raymondsun24.bridgely

import com.intellij.codeInsight.daemon.impl.DaemonCodeAnalyzerImpl
import com.intellij.diff.DiffContentFactory
import com.intellij.diff.DiffManager
import com.intellij.diff.requests.SimpleDiffRequest
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.actionSystem.ActionManager
import com.intellij.openapi.actionSystem.ActionPlaces
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.actionSystem.impl.SimpleDataContext
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.fileEditor.FileEditorManager
import com.intellij.openapi.fileEditor.OpenFileDescriptor
import com.intellij.openapi.fileEditor.TextEditor
import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.openapi.wm.ToolWindowManager
import com.intellij.psi.PsiManager
import java.io.File

/**
 * Executes commands dispatched by [CommandWatcher].
 *
 * Runs on the watcher background thread; UI operations use invokeAndWait,
 * read-model operations use runReadAction.
 */
class CommandHandlers(
    private val project: Project,
    private val sessionId: String,
) {
    @Volatile private var lastProposedFile: File? = null
    @Volatile private var lastPreviewFilePath: String? = null
    @Volatile private var lastPreviewLine: Int? = null
    @Volatile private var lastDiffWindow: java.awt.Window? = null
    @Volatile private var lastDiffVFile: com.intellij.openapi.vfs.VirtualFile? = null

    fun dispatch(cmd: BridgeCommand) {
        val result = try {
            when (cmd.command) {
                "openFile"          -> handleOpenFile(cmd.id, cmd.args)
                "revealLine"        -> handleRevealLine(cmd.id, cmd.args)
                "getSelection"      -> handleGetSelection(cmd.id)
                "getDiagnostics"    -> handleGetDiagnostics(cmd.id, cmd.args)
                "showDiff"          -> handleShowDiff(cmd.id, cmd.args)
                "closeDiff"         -> handleCloseDiff(cmd.id)
                "previewEdit"       -> handlePreviewEdit(cmd.id, cmd.args)
                "closePreview"      -> handleClosePreview(cmd.id)
                "getTerminalOutput" -> ok(cmd.id, "Terminal output is not yet supported in the JetBrains plugin")
                else                -> err(cmd.id, "Unknown command: ${cmd.command}")
            }
        } catch (e: Exception) {
            err(cmd.id, "Command failed: ${e.message}")
        }

        runCatching {
            atomicWriteJson(BridgePaths.getSessionCommandResultsPath(sessionId), result)
        }
    }

    // ── openFile ──────────────────────────────────────────────────────────────

    private fun handleOpenFile(id: String, args: Map<String, Any?>): CommandResult {
        val path   = args["path"] as? String  ?: return err(id, "Missing path")
        val line   = (args["line"]   as? Double)?.toInt()
        val column = (args["column"] as? Double)?.toInt() ?: 1

        var msg = ""
        ApplicationManager.getApplication().invokeAndWait {
            val vFile = refreshFind(path) ?: run { msg = "File not found: $path"; return@invokeAndWait }
            val descriptor = if (line != null)
                OpenFileDescriptor(project, vFile, line - 1, column - 1)
            else
                OpenFileDescriptor(project, vFile)
            descriptor.navigate(true)
            msg = "Opened ${vFile.name}${if (line != null) " at line $line" else ""}"
        }

        return if (msg.startsWith("File not found")) err(id, msg) else ok(id, msg)
    }

    // ── revealLine ────────────────────────────────────────────────────────────

    private fun handleRevealLine(id: String, args: Map<String, Any?>): CommandResult {
        val path = args["path"] as? String ?: return err(id, "Missing path")
        val line = (args["line"] as? Double)?.toInt() ?: return err(id, "Missing line")

        var msg = ""
        ApplicationManager.getApplication().invokeAndWait {
            val vFile = refreshFind(path) ?: run { msg = "File not found: $path"; return@invokeAndWait }
            // preserveFocus = false so IDE doesn't steal focus from the terminal
            OpenFileDescriptor(project, vFile, line - 1, 0).navigate(false)
            msg = "Revealed line $line"
        }

        return if (msg.startsWith("File not found")) err(id, msg) else ok(id, msg)
    }

    // ── getSelection ──────────────────────────────────────────────────────────

    private fun handleGetSelection(id: String): CommandResult {
        var data: Map<String, Any?> = mapOf("text" to "", "filePath" to "")

        ApplicationManager.getApplication().runReadAction {
            val fem = FileEditorManager.getInstance(project)
            val ed  = (fem.selectedEditor as? TextEditor)?.editor ?: return@runReadAction
            val sel = ed.selectionModel
            if (!sel.hasSelection()) return@runReadAction

            val startPos = ed.offsetToLogicalPosition(sel.selectionStart)
            val endPos   = ed.offsetToLogicalPosition(sel.selectionEnd)
            data = mapOf(
                "text"      to (sel.selectedText ?: ""),
                "filePath"  to (fem.selectedFiles.firstOrNull()?.path ?: ""),
                "startLine" to (startPos.line + 1),
                "endLine"   to (endPos.line + 1),
            )
        }

        return ok(id, "Selection retrieved", data)
    }

    // ── getDiagnostics ────────────────────────────────────────────────────────

    @Suppress("UnstableApiUsage")
    private fun handleGetDiagnostics(id: String, args: Map<String, Any?>): CommandResult {
        val filterPath = args["path"] as? String
        val results    = mutableListOf<Map<String, Any?>>()

        ApplicationManager.getApplication().runReadAction {
            val psiMgr  = PsiManager.getInstance(project)
            val fdm     = FileDocumentManager.getInstance()
            val fem     = FileEditorManager.getInstance(project)

            val targets = if (filterPath != null) {
                listOfNotNull(refreshFind(filterPath))
            } else {
                fem.openFiles.toList()
            }

            for (vFile in targets) {
                val doc  = fdm.getDocument(vFile) ?: continue
                psiMgr.findFile(vFile) ?: continue  // ensure PSI is available

                val infos = try {
                    DaemonCodeAnalyzerImpl.getHighlights(doc, HighlightSeverity.WARNING, project)
                } catch (_: Exception) {
                    continue
                }

                if (infos.isEmpty()) continue

                results += mapOf(
                    "path" to vFile.path,
                    "diagnostics" to infos.map { info ->
                        mapOf(
                            "line"     to (doc.getLineNumber(info.startOffset) + 1),
                            "severity" to info.severity.name,
                            "message"  to info.description,
                            "source"   to (info.inspectionToolId ?: "JetBrains"),
                        )
                    },
                )
            }
        }

        return ok(id, "${results.size} file(s) with diagnostics", results)
    }

    // ── showDiff ──────────────────────────────────────────────────────────────

    private fun handleShowDiff(id: String, args: Map<String, Any?>): CommandResult {
        val path = args["path"] as? String ?: return err(id, "Missing path")

        ApplicationManager.getApplication().invokeAndWait {
            val vFile = refreshFind(path) ?: return@invokeAndWait
            vFile.refresh(false, false)

            // "Compare.LastVersion" = "Compare with Repository" — works for any VCS
            val action = ActionManager.getInstance().getAction("Compare.LastVersion")
            if (action != null) {
                val ctx = SimpleDataContext.builder()
                    .add(CommonDataKeys.PROJECT, project)
                    .add(CommonDataKeys.VIRTUAL_FILE, vFile)
                    .build()
                val event = AnActionEvent.createFromDataContext(ActionPlaces.UNKNOWN, null, ctx)
                action.actionPerformed(event)
            } else {
                OpenFileDescriptor(project, vFile).navigate(true)
            }
        }

        return ok(id, "Showing diff for $path")
    }

    // ── closeDiff ─────────────────────────────────────────────────────────────

    private fun handleCloseDiff(id: String): CommandResult {
        ApplicationManager.getApplication().invokeAndWait {
            val fem = FileEditorManager.getInstance(project)
            lastDiffVFile?.let { fem.closeFile(it) }
            lastDiffVFile = null
            lastDiffWindow?.dispose()
            lastDiffWindow = null
            ToolWindowManager.getInstance(project).getToolWindow("Diff")?.hide()
        }
        return ok(id, "Diff closed")
    }

    // ── previewEdit ───────────────────────────────────────────────────────────

    private fun handlePreviewEdit(id: String, args: Map<String, Any?>): CommandResult {
        val filePath = args["file_path"] as? String ?: return err(id, "Missing file_path")
        val toolName = args["tool_name"] as? String ?: "Edit"

        val currentFile = File(filePath)
        if (!currentFile.exists()) return err(id, "File not found: $filePath")

        val currentContent = currentFile.readText()

        var editLine: Int? = null
        val proposedContent = when (toolName) {
            "Write" -> args["content"] as? String ?: ""
            else -> {
                val oldStr = args["old_string"] as? String ?: ""
                val newStr = args["new_string"] as? String ?: ""
                val idx = currentContent.indexOf(oldStr)
                if (idx == -1) return err(id, "old_string not found in $filePath")
                editLine = currentContent.substring(0, idx).count { it == '\n' } + 1
                currentContent.substring(0, idx) + newStr + currentContent.substring(idx + oldStr.length)
            }
        }

        val proposedDir = BridgePaths.getProposedDir().toFile().also { it.mkdirs() }
        val base = currentFile.nameWithoutExtension
        val ext  = currentFile.extension.let { if (it.isNotEmpty()) ".$it" else "" }
        val tmp  = File(proposedDir, "$base.proposed$ext")
        tmp.writeText(proposedContent)
        lastProposedFile = tmp
        lastPreviewFilePath = filePath
        lastPreviewLine = editLine

        ApplicationManager.getApplication().invokeAndWait {
            val currentVFile  = refreshFind(filePath) ?: return@invokeAndWait
            val proposedVFile = LocalFileSystem.getInstance()
                .refreshAndFindFileByPath(tmp.absolutePath) ?: return@invokeAndWait

            val factory = DiffContentFactory.getInstance()
            val request = SimpleDiffRequest(
                "${currentFile.name} ← proposed edit",
                factory.create(project, currentVFile),
                factory.create(project, proposedVFile),
                "Current",
                "Proposed",
            )

            val fem = FileEditorManager.getInstance(project)
            val filesBefore = fem.openFiles.toSet()
            DiffManager.getInstance().showDiff(project, request)
            lastDiffVFile = fem.openFiles.firstOrNull { it !in filesBefore }
        }

        return ok(id, "Previewing edit for $filePath")
    }

    // ── closePreview ──────────────────────────────────────────────────────────

    private fun handleClosePreview(id: String): CommandResult {
        lastProposedFile?.delete()
        lastProposedFile = null
        val filePath = lastPreviewFilePath
        val line = lastPreviewLine
        lastPreviewFilePath = null
        lastPreviewLine = null

        val result = handleCloseDiff(id)

        if (filePath != null) {
            ApplicationManager.getApplication().invokeAndWait {
                val vFile = refreshFind(filePath) ?: return@invokeAndWait
                val descriptor = if (line != null)
                    OpenFileDescriptor(project, vFile, line - 1, 0)
                else
                    OpenFileDescriptor(project, vFile)
                descriptor.navigate(true)
            }
        }

        return result
    }

    // ── helpers ───────────────────────────────────────────────────────────────

    private fun refreshFind(path: String) =
        LocalFileSystem.getInstance().findFileByPath(path)
            ?: LocalFileSystem.getInstance().refreshAndFindFileByPath(path)

    private fun ok(id: String, message: String, data: Any? = null) = CommandResult(
        version   = 1,
        id        = id,
        timestamp = System.currentTimeMillis(),
        status    = "ok",
        result    = CommandResultPayload(message, data),
    )

    private fun err(id: String, message: String) = CommandResult(
        version   = 1,
        id        = id,
        timestamp = System.currentTimeMillis(),
        status    = "error",
        result    = CommandResultPayload(message),
    )
}
