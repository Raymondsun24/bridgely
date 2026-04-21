package com.raymondsun24.bridgely

import com.intellij.openapi.project.DumbAware
import com.intellij.openapi.project.Project
import com.intellij.openapi.startup.StartupActivity

/**
 * Forces [BridgelyService] to initialize when the project opens.
 * Project services in IntelliJ are lazy — without this the service
 * would never activate unless something else requests it.
 */
class BridgelyStartupActivity : StartupActivity, DumbAware {
    override fun runActivity(project: Project) {
        BridgelyService.getInstance(project)
    }
}
