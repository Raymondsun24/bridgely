package com.raymondsun24.bridgely

import java.nio.file.Path
import java.nio.file.Paths

object BridgePaths {
    private val home: String = System.getProperty("user.home")
    private val bridgeDir: Path = Paths.get(home, ".claude", "bridge")
    private val sessionsDir: Path = bridgeDir.resolve("sessions")
    private val proposedDir: Path = bridgeDir.resolve("proposed")

    fun getSessionsDir(): Path = sessionsDir
    fun getProposedDir(): Path = proposedDir

    fun getSessionStatePath(sessionId: String): Path =
        sessionsDir.resolve("$sessionId.json")

    fun getSessionCommandsPath(sessionId: String): Path =
        sessionsDir.resolve("$sessionId.commands.json")

    fun getSessionCommandResultsPath(sessionId: String): Path =
        sessionsDir.resolve("$sessionId.commands-result.json")

    fun ensureDirs() {
        sessionsDir.toFile().mkdirs()
        proposedDir.toFile().mkdirs()
    }
}
