package com.raymondsun24.bridgely

import com.google.gson.Gson
import com.google.gson.reflect.TypeToken
import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption

private val gson = Gson()

/**
 * Write [obj] to [path] atomically by writing to a sibling tmp file and
 * renaming — same approach as the VS Code extension's atomicWriteJson.
 */
fun atomicWriteJson(path: Path, obj: Any) {
    val tmp = File(path.parent.toFile(), ".tmp.${path.fileName}")
    try {
        tmp.writeText(gson.toJson(obj))
        Files.move(tmp.toPath(), path, StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING)
    } catch (e: Exception) {
        tmp.delete()
        throw e
    }
}

/**
 * Read [path] as JSON into a [Map]. Returns null on any parse or IO error.
 */
fun safeReadJsonMap(path: Path): Map<String, Any?>? {
    return try {
        val text = path.toFile().readText().trim()
        if (text.isEmpty() || text == "{}") return null
        val type = object : TypeToken<Map<String, Any?>>() {}.type
        gson.fromJson(text, type)
    } catch (_: Exception) {
        null
    }
}
