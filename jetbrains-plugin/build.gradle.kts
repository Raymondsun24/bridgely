plugins {
    id("org.jetbrains.kotlin.jvm") version "1.9.25"
    id("org.jetbrains.intellij.platform") version "2.1.0"
}

group = "com.raymondsun24"
version = "0.1.0"

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    // Gson is bundled in the IntelliJ platform but we declare it explicitly
    // so the compiler can resolve it; the platform provides it at runtime.
    compileOnly("com.google.code.gson:gson:2.10.1")

    intellijPlatform {
        // Target IntelliJ IDEA Community 2024.1 — covers all JetBrains IDEs
        // built on the IntelliJ Platform (PyCharm, WebStorm, GoLand, etc.)
        intellijIdeaCommunity("2024.1")
        instrumentationTools()
    }
}

intellijPlatform {
    pluginConfiguration {
        name = "Bridgely"
        ideaVersion {
            sinceBuild = "241"   // 2024.1
            untilBuild = provider { null }  // no upper bound
        }
    }
    publishing {
        token = providers.environmentVariable("PUBLISH_TOKEN")
    }
}

kotlin {
    jvmToolchain(17)
}
