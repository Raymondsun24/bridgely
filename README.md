# Bridgely – Claude Code Bridge for VS Code

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.4-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org)
[![Node.js](https://img.shields.io/badge/Node.js-18+-5FA04E?logo=nodedotjs&logoColor=white)](https://nodejs.org)
[![VS Code](https://img.shields.io/badge/VS%20Code-%5E1.93-007ACC?logo=visualstudiocode&logoColor=white)](https://marketplace.visualstudio.com)

Bidirectional bridge between VS Code/Cursor and Claude Code running in any terminal. Gives Claude real-time awareness of your editor state and lets it open files, show diffs, and fetch LSP diagnostics.

## Demo

**Claude reading your active file and selection**

<video src="https://github.com/Raymondsun24/bridgely/releases/download/v0.1.2/bridgely-demo.mp4" autoplay loop muted playsinline></video>

**VS Code showing proposed changes from Claude**

<video src="https://github.com/Raymondsun24/bridgely/releases/download/v0.1.2/bridgely-demo-2.mp4" autoplay loop muted playsinline></video>

## Why

When using Claude Code, you have two options — and both involve a trade-off:

- **Use the VS Code/Cursor Claude Code extension.** Claude knows exactly what file you have open, where your cursor is, and what you've selected. But you're stuck using the built-in terminal emulator, which is sluggish, limited in features, and not where you want to spend your day.
- **Run Claude Code in an external terminal like Ghostty.** You get a snappy, fully-featured terminal experience. But Claude loses all editor awareness — it doesn't know what you're looking at, can't tell where your cursor is, and can't open files or diffs back in your editor.

There's no option that gives you both.

Bridgely fixes that. You run Claude Code in whatever terminal you want, and the bridge keeps your editor and Claude in sync — Claude sees your active file and selection on every prompt, and can open files, preview diffs, and pull LSP diagnostics directly in VS Code or Cursor.

## Architecture

```
VS Code/Cursor Extension          Terminal (Claude Code)
┌──────────────────────┐          ┌────────────────────────┐
│ editorStateWriter    │ ──JSON──→│ bridgely hook:context  │
│ (tracks editor state)│          │ (injects into prompt)  │
├──────────────────────┤          ├────────────────────────┤
│ commandWatcher       │ ←──JSON──│ bridgely / MCP         │
│ (executes commands)  │          │ (sends commands)       │
└──────────────────────┘          └────────────────────────┘
         ↕ files at ~/.claude/bridge/sessions/
```

All communication happens via local JSON files — no sockets, no servers.

## Prerequisites

- **VS Code** or **Cursor**
- **Go 1.23+** — for the MCP server (`brew install go`)
- **jq** — for the CLI (`brew install jq`)

## Setup

### 1. Install the VS Code/Cursor extension

Search for **Bridgely** in the Extensions Marketplace and install it, or:

```bash
# Cursor:
cursor --install-extension Raymondsun24.bridgely

# VS Code:
code --install-extension Raymondsun24.bridgely
```

Reload the editor after installing (`Cmd+Shift+P` → `Developer: Reload Window`).

### 2. Install the CLI and MCP server

```bash
git clone https://github.com/Raymondsun24/bridgely.git
cd bridgely
make mcp          # builds the Go MCP server binary
./cli/install.sh  # symlinks bridgely to ~/.local/bin/bridgely
```

If `~/.local/bin` is not in your PATH, add this to `~/.zshrc` or `~/.bashrc`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

> **Developing?** Run `make help` to see all build targets for compiling the extension, packaging a `.vsix`, or running tests.

### 3. Add the MCP server to Claude Code

Run this from the cloned directory:

```bash
claude mcp add bridgely -- "$(pwd)/mcp/bridgely"
```

### 4. Configure Claude Code hooks

With the `bridgely` CLI installed (step 3), add these to `~/.claude/settings.json` under `"hooks"`:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "bridgely hook:context", "timeout": 3 }]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [{ "type": "command", "command": "bridgely hook:preview", "timeout": 5 }]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [{ "type": "command", "command": "bridgely hook:edit", "timeout": 3 }]
      }
    ]
  }
}
```

| Hook | Trigger | What it does |
|------|---------|-------------|
| `bridgely hook:context` | Every prompt | Injects editor state (active file, cursor, selection) into context |
| `bridgely hook:preview` | Before Edit/Write | Opens a preview diff of the proposed changes in the editor |
| `bridgely hook:edit` | After Edit/Write | Closes the preview diff tab once the edit is applied or rejected |

### 5. (Optional) Allow MCP tools in permissions

Add to `~/.claude/settings.json` under `"permissions" > "allow"`:

```json
"mcp__bridgely__*"
```

## Usage

### CLI

```bash
bridgely sessions                    # List active editor sessions
bridgely status                      # Full editor state JSON
bridgely file                        # Active file path
bridgely selection                   # Current text selection
bridgely open /path/to/file.ts 42   # Open file at line 42
bridgely reveal /path/to/file.ts 100 # Scroll to line (no focus change)
bridgely diff /path/to/file.ts      # Show git diff in editor
bridgely diagnostics /path/to/file  # LSP errors/warnings for a file
bridgely diagnostics                 # All workspace diagnostics
bridgely watch                       # Stream editor state changes (requires fswatch)
bridgely bind                        # Bind CWD to an editor session (interactive)
bridgely bind Cursor-5800            # Bind CWD to a specific session
bridgely unbind                      # Remove CWD binding
bridgely bindings                    # List all CWD → session bindings
```

Most commands accept a `-s`/`--session` flag to target a specific editor session:

```bash
bridgely file -s Cursor              # Active file in any Cursor session
bridgely diff src/main.ts -s 5800   # Route diff to a specific session
```

### Multi-session support

When multiple editors are open, the bridge routes commands to the right one:

1. **CWD binding** — `bridgely bind` associates your terminal's working directory with a specific editor
2. **Workspace auto-match** — if your CWD is inside an editor's workspace folder, it routes automatically
3. **Most recent** — falls back to the most recently active editor

### Terminal output capture

Requires [shell integration](https://code.visualstudio.com/docs/terminal/shell-integration) (on by default in VS Code/Cursor). Captures command text, stdout/stderr (ANSI stripped), exit code, and working directory.

**Note:** Shell integration only activates for terminals created _after_ the setting is enabled.

### VS Code/Cursor extension detection

When Claude is launched from the VS Code/Cursor Claude extension (not a terminal), `bridgely hook:context` automatically skips — the extension already has native editor awareness. `hook:preview` and `hook:edit` still run. Detection uses the `CLAUDE_CODE_SSE_PORT` environment variable.

