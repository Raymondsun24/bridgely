package com.raymondsun24.bridgely

import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationInfo
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.service
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Disposer

@Service(Service.Level.PROJECT)
class BridgelyService(project: Project) : Disposable {

    private val handlers: CommandHandlers

    init {
        BridgePaths.ensureDirs()

        val sessionId = buildSessionId(project)
        handlers = CommandHandlers(project, sessionId)

        val stateWriter = EditorStateWriter(project, sessionId)
        val commandWatcher = CommandWatcher(project, sessionId, handlers)

        commandWatcher.start()

        Disposer.register(this, stateWriter)
        Disposer.register(this, commandWatcher)
    }

    fun showDiff(path: String) {
        val id = "service-diff-${System.currentTimeMillis()}"
        handlers.dispatch(BridgeCommand(
            version   = 1,
            id        = id,
            timestamp = System.currentTimeMillis(),
            command   = "showDiff",
            args      = mapOf("path" to path),
        ))
    }

    fun closeDiff() {
        val id = "service-closediff-${System.currentTimeMillis()}"
        handlers.dispatch(BridgeCommand(
            version   = 1,
            id        = id,
            timestamp = System.currentTimeMillis(),
            command   = "closeDiff",
            args      = emptyMap(),
        ))
    }

    override fun dispose() {}

    companion object {
        fun getInstance(project: Project): BridgelyService = project.service()

        fun buildSessionId(project: Project): String {
            val ide = ideName()
            val pid = ProcessHandle.current().pid()
            val name = project.name.replace(Regex("[^a-zA-Z0-9_-]"), "_")
            return "$ide-$name-$pid"
        }

        fun ideName(): String {
            val full = ApplicationInfo.getInstance().versionName
            return when {
                full.contains("IntelliJ", ignoreCase = true) -> "IntelliJ"
                full.contains("PyCharm", ignoreCase = true) -> "PyCharm"
                full.contains("WebStorm", ignoreCase = true) -> "WebStorm"
                full.contains("GoLand", ignoreCase = true) -> "GoLand"
                full.contains("PhpStorm", ignoreCase = true) -> "PhpStorm"
                full.contains("CLion", ignoreCase = true) -> "CLion"
                full.contains("Rider", ignoreCase = true) -> "Rider"
                full.contains("DataGrip", ignoreCase = true) -> "DataGrip"
                full.contains("RubyMine", ignoreCase = true) -> "RubyMine"
                full.contains("Aqua", ignoreCase = true) -> "Aqua"
                else -> full.split(" ").firstOrNull() ?: "JetBrains"
            }
        }
    }
}
