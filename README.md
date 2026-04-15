# Bridgely – Claude Code Bridge for VS Code

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.4-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org)
[![Node.js](https://img.shields.io/badge/Node.js-18+-5FA04E?logo=nodedotjs&logoColor=white)](https://nodejs.org)
[![VS Code](https://img.shields.io/badge/VS%20Code-%5E1.93-007ACC?logo=visualstudiocode&logoColor=white)](https://marketplace.visualstudio.com)

Bidirectional bridge between VS Code/Cursor and Claude Code running in any terminal. Gives Claude real-time awareness of your editor state and lets it open files, show diffs, and fetch LSP diagnostics.

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
- **Node.js + npm** — for compiling the VS Code extension (`brew install node`)
- **jq** — for the CLI (`brew install jq`)

## Setup

### 1. Clone and build

```bash
git clone https://github.com/Raymondsun24/bridgely.git
cd bridgely
npm install   # installs TypeScript toolchain for the extension
make          # builds the VS Code extension + Go MCP binary
```

### 2. Install the VS Code/Cursor extension

```bash
make package
# Creates bridgely-0.1.0.vsix

# Install in Cursor:
cursor --install-extension bridgely-0.1.0.vsix

# Or in VS Code:
code --install-extension bridgely-0.1.0.vsix
```

Reload the editor after installing (`Cmd+Shift+P` → `Developer: Reload Window`).

### 3. Install the CLI

```bash
./cli/install.sh
# Symlinks bridgely.sh to ~/.local/bin/bridgely
```

If `~/.local/bin` is not in your PATH, add this to `~/.zshrc` or `~/.bashrc`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### 4. Add the MCP server to Claude Code

Run this from the project directory:

```bash
claude mcp add bridgely -- "$(pwd)/mcp/bridgely"
```

### 5. Configure Claude Code hooks

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

### 6. (Optional) Allow MCP tools in permissions

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

### MCP tools (available to Claude)

| Tool | Description |
|------|-------------|
| `editor_sessions` | List active editor sessions |
| `editor_status` | Get editor state (file, cursor, selection, tabs, terminals) |
| `editor_open_file` | Open a file at a specific line |
| `editor_reveal_line` | Scroll to a line without focus change |
| `editor_get_selection` | Get the current text selection |
| `editor_get_diagnostics` | Get LSP diagnostics |
| `editor_get_terminal_output` | Get recent terminal command executions and output |

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

## Extension settings

| Setting | Default | Description |
|---------|---------|-------------|
| `bridgely.enabled` | `true` | Enable/disable the bridge |
| `bridgely.debounceMs` | `300` | Debounce for editor state updates (ms) |
| `bridgely.selectionDebounceMs` | `150` | Debounce for selection changes (ms) |
| `bridgely.maxSelectionLength` | `10000` | Max characters from text selections |
| `bridgely.maxTerminalExecutions` | `50` | Max terminal command executions to buffer |
| `bridgely.maxTerminalOutputLength` | `20000` | Max characters per terminal command output |

## Development

```bash
make              # Build everything (extension + MCP binary)
make extension    # Build VS Code extension only (tsc)
make mcp          # Build Go MCP binary only
make watch        # Watch mode for extension TypeScript
make package      # Create .vsix package
make clean        # Remove build artifacts
```
