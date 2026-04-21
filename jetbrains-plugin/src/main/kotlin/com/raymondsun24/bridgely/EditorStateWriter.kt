package com.raymondsun24.bridgely

import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.editor.EditorFactory
import com.intellij.openapi.editor.event.CaretEvent
import com.intellij.openapi.editor.event.CaretListener
import com.intellij.openapi.editor.event.SelectionEvent
import com.intellij.openapi.editor.event.SelectionListener
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.fileEditor.FileEditorManager
import com.intellij.openapi.fileEditor.FileEditorManagerEvent
import com.intellij.openapi.fileEditor.FileEditorManagerListener
import com.intellij.openapi.fileEditor.TextEditor
import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.util.Alarm

class EditorStateWriter(
    private val project: Project,
    private val sessionId: String,
) : Disposable {

    private val alarm = Alarm(Alarm.ThreadToUse.POOLED_THREAD, this)
    private val selectionAlarm = Alarm(Alarm.ThreadToUse.POOLED_THREAD, this)

    companion object {
        private const val DEBOUNCE_MS = 300L
        private const val SELECTION_DEBOUNCE_MS = 150L
        private const val MAX_SELECTION_LEN = 10_000
    }

    init {
        // File editor events (open, close, tab switch) — project-scoped
        project.messageBus.connect(this).subscribe(
            FileEditorManagerListener.FILE_EDITOR_MANAGER,
            object : FileEditorManagerListener {
                override fun selectionChanged(event: FileEditorManagerEvent) = scheduleWrite()
                override fun fileOpened(source: FileEditorManager, file: VirtualFile) = scheduleWrite()
                override fun fileClosed(source: FileEditorManager, file: VirtualFile) = scheduleWrite()
            }
        )

        // Caret and selection events — fired across all editors; filter by project
        val multicaster = EditorFactory.getInstance().eventMulticaster

        multicaster.addCaretListener(object : CaretListener {
            override fun caretPositionChanged(event: CaretEvent) {
                if (isOurEditor(event.editor)) scheduleSelectionWrite()
            }
        }, this)

        multicaster.addSelectionListener(object : SelectionListener {
            override fun selectionChanged(event: SelectionEvent) {
                if (isOurEditor(event.editor)) scheduleSelectionWrite()
            }
        }, this)

        // Emit initial state immediately
        writeState()
    }

    private fun isOurEditor(editor: Editor): Boolean =
        editor.project == project && !project.isDisposed

    private fun scheduleWrite() {
        alarm.cancelAllRequests()
        alarm.addRequest(::writeState, DEBOUNCE_MS)
    }

    private fun scheduleSelectionWrite() {
        selectionAlarm.cancelAllRequests()
        selectionAlarm.addRequest(::writeState, SELECTION_DEBOUNCE_MS)
    }

    private fun writeState() {
        if (project.isDisposed) return
        val state = ApplicationManager.getApplication().runReadAction<EditorState?> {
            try { buildState() } catch (_: Exception) { null }
        } ?: return
        try {
            atomicWriteJson(BridgePaths.getSessionStatePath(sessionId), state)
        } catch (_: Exception) { /* best-effort */ }
    }

    private fun buildState(): EditorState {
        val fem = FileEditorManager.getInstance(project)
        val activeEditor = fem.selectedEditor
        val activeFile = activeEditor?.file
        val textEditor = (activeEditor as? TextEditor)?.editor

        val activeFileInfo = activeFile?.let { vf ->
            val doc = textEditor?.document
            val caret = textEditor?.caretModel?.logicalPosition
            ActiveFile(
                path = vf.path,
                relativePath = vf.path.removePrefix(project.basePath ?: "").trimStart('/'),
                languageId = vf.fileType.name.lowercase(),
                lineCount = doc?.lineCount ?: 0,
                cursorLine = (caret?.line ?: 0) + 1,
                cursorColumn = (caret?.column ?: 0) + 1,
            )
        }

        val selectionInfo = textEditor?.let { ed ->
            val sel = ed.selectionModel
            if (!sel.hasSelection()) return@let null
            val raw = sel.selectedText ?: ""
            val text = if (raw.length > MAX_SELECTION_LEN)
                raw.substring(0, MAX_SELECTION_LEN) + "\n... (truncated at $MAX_SELECTION_LEN chars)"
            else raw
            val startPos = ed.offsetToLogicalPosition(sel.selectionStart)
            val endPos = ed.offsetToLogicalPosition(sel.selectionEnd)
            SelectionInfo(
                text = text,
                startLine = startPos.line + 1,
                startColumn = startPos.column + 1,
                endLine = endPos.line + 1,
                endColumn = endPos.column + 1,
                filePath = activeFile?.path ?: "",
            )
        }

        val visibleFiles = fem.openFiles.mapIndexed { idx, vf ->
            VisibleFile(
                path = vf.path,
                isActive = vf == activeFile,
                isDirty = FileDocumentManager.getInstance().isFileModified(vf),
                viewColumn = idx + 1,
            )
        }

        return EditorState(
            version = 1,
            timestamp = System.currentTimeMillis(),
            pid = ProcessHandle.current().pid(),
            sessionId = sessionId,
            ideName = BridgelyService.ideName(),
            workspace = WorkspaceInfo(
                folders = listOfNotNull(project.basePath),
                name = project.name,
            ),
            activeFile = activeFileInfo,
            selection = selectionInfo,
            visibleFiles = visibleFiles,
        )
    }

    override fun dispose() {
        try {
            BridgePaths.getSessionStatePath(sessionId).toFile().delete()
        } catch (_: Exception) { /* already gone */ }
    }
}
