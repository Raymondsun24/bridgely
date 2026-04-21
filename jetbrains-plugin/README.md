# Bridgely – JetBrains Plugin

Bridgely plugin for all IDEs built on the IntelliJ Platform: IntelliJ IDEA, PyCharm, WebStorm, GoLand, PhpStorm, CLion, Rider, and more.

Gives Claude Code real-time awareness of your editor state and lets it open files, preview diffs (with auto-close and jump-to-line), and fetch LSP diagnostics — all from your terminal.

## Prerequisites

- JDK 17+
- Gradle (or use the included wrapper after bootstrapping)
- The shared Bridgely CLI and MCP server — see the [main README](../README.md#setup)

## Installation

### 1. Build

```bash
cd jetbrains-plugin

# First time only: bootstrap the Gradle wrapper (requires Gradle installed globally)
gradle wrapper --gradle-version 8.8

# Build the plugin .zip
./gradlew buildPlugin
# Output: build/distributions/bridgely-jetbrains-*.zip
```

### 2. Install in your IDE

1. Open **Settings → Plugins**
2. Click the **⚙️** gear icon → **Install Plugin from Disk…**
3. Select the `.zip` from `build/distributions/`
4. **Restart the IDE**

The plugin activates automatically on project open — no additional configuration required.

### 3. Verify

Open a project, then run in your terminal:

```bash
bridgely sessions   # should list an IntelliJ-{project}-{PID} session
bridgely status     # shows active file, cursor, and workspace
```

Session IDs follow the pattern `{IDEName}-{projectName}-{PID}` (e.g. `IntelliJ-myblog-12345`).

## Features

| Feature | Supported |
|---------|-----------|
| Editor state (active file, cursor, selection) | ✅ |
| Open file at line | ✅ |
| Reveal line (no focus change) | ✅ |
| Get selection | ✅ |
| LSP diagnostics | ✅ |
| Git diff view | ✅ |
| Preview edit (diff before applying) | ✅ |
| Auto-close diff after edit | ✅ |
| Jump to edited line after diff closes | ✅ |
| Terminal output capture | 🔜 future |

## Hooks

The same Claude Code hooks used for VS Code work here:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      { "matcher": "", "hooks": [{ "type": "command", "command": "bridgely hook:context", "timeout": 3 }] }
    ],
    "PreToolUse": [
      { "matcher": "Edit|Write", "hooks": [{ "type": "command", "command": "bridgely hook:preview", "timeout": 5 }] }
    ],
    "PostToolUse": [
      { "matcher": "Edit|Write", "hooks": [{ "type": "command", "command": "bridgely hook:edit", "timeout": 3 }] }
    ]
  }
}
```

Add these to `~/.claude/settings.json`. See the [main README](../README.md#4-configure-claude-code-hooks) for the full hooks setup.
