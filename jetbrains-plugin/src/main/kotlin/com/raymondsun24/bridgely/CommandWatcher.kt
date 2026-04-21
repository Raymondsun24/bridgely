package com.raymondsun24.bridgely

import com.intellij.openapi.Disposable
import com.intellij.openapi.project.Project
import java.nio.file.FileSystems
import java.nio.file.StandardWatchEventKinds
import java.util.concurrent.TimeUnit

/**
 * Watches `{sessionId}.commands.json` for incoming commands written by the MCP
 * server or CLI, then delegates to [CommandHandlers].
 *
 * Uses Java NIO WatchService on a daemon thread — same pattern as the
 * VS Code extension's fs.watch approach.
 */
class CommandWatcher(
    private val project: Project,
    private val sessionId: String,
    private val handlers: CommandHandlers = CommandHandlers(project, sessionId),
) : Disposable {

    private val commandsPath = BridgePaths.getSessionCommandsPath(sessionId)
    private var lastCommandId: String? = null

    @Volatile private var running = false
    private var watcherThread: Thread? = null

    fun start() {
        // Ensure the commands file exists so the watcher has a directory to watch
        val file = commandsPath.toFile()
        if (!file.exists()) file.writeText("{}")

        running = true
        watcherThread = Thread(::watchLoop, "bridgely-watcher-$sessionId").also {
            it.isDaemon = true
            it.start()
        }
    }

    private fun watchLoop() {
        val dir = commandsPath.parent
        val watchService = FileSystems.getDefault().newWatchService()
        try {
            dir.register(
                watchService,
                StandardWatchEventKinds.ENTRY_CREATE,
                StandardWatchEventKinds.ENTRY_MODIFY,
            )

            while (running) {
                val key = watchService.poll(500, TimeUnit.MILLISECONDS) ?: continue

                for (event in key.pollEvents()) {
                    val changed = event.context()?.toString() ?: continue
                    if (changed == commandsPath.fileName.toString()) {
                        // Brief delay to let the atomic rename complete
                        Thread.sleep(50)
                        handleChange()
                    }
                }

                if (!key.reset()) break
            }
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
        } catch (_: Exception) {
            // Watcher died — silently ignore
        } finally {
            runCatching { watchService.close() }
        }
    }

    private fun handleChange() {
        if (project.isDisposed) return
        val raw = safeReadJsonMap(commandsPath) ?: return

        val id = raw["id"] as? String ?: return
        val command = raw["command"] as? String ?: return
        if (id == lastCommandId) return
        lastCommandId = id

        @Suppress("UNCHECKED_CAST")
        val args = (raw["args"] as? Map<String, Any?>) ?: emptyMap()

        val cmd = BridgeCommand(
            version = (raw["version"] as? Double)?.toInt() ?: 1,
            id = id,
            timestamp = (raw["timestamp"] as? Double)?.toLong() ?: System.currentTimeMillis(),
            command = command,
            args = args,
        )

        handlers.dispatch(cmd)
    }

    override fun dispose() {
        running = false
        watcherThread?.interrupt()
        runCatching { commandsPath.toFile().delete() }
        runCatching { BridgePaths.getSessionCommandResultsPath(sessionId).toFile().delete() }
    }
}
