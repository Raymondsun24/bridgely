# Bridget — Editor Bridge for Claude Code

Bidirectional bridge between VS Code/Cursor and Claude Code running in any terminal. Gives Claude real-time awareness of your editor state and lets it open files, show diffs, and fetch LSP diagnostics.

## Architecture

```
VS Code/Cursor Extension          Terminal (Claude Code)
┌──────────────────────┐          ┌──────────────────────┐
│ editorStateWriter    │ ──JSON──→│ bridget hook:context  │
│ (tracks editor state)│          │ (injects into prompt)│
├──────────────────────┤          ├──────────────────────┤
│ commandWatcher       │ ←─JSON──│ bridget / MCP        │
│ (executes commands)  │          │ (sends commands)     │
└──────────────────────┘          └──────────────────────┘
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
git clone https://github.com/Raymondsun24/bridget.git
cd bridget
npm install   # installs TypeScript toolchain for the extension
make          # builds the VS Code extension + Go MCP binary
```

### 2. Install the VS Code/Cursor extension

```bash
make package
# Creates bridget-0.1.0.vsix

# Install in Cursor:
cursor --install-extension bridget-0.1.0.vsix

# Or in VS Code:
code --install-extension bridget-0.1.0.vsix
```

Reload the editor after installing (`Cmd+Shift+P` → `Developer: Reload Window`).

### 3. Install the CLI

```bash
./cli/install.sh
# Symlinks bridget.sh to ~/.local/bin/bridget
```

If `~/.local/bin` is not in your PATH, add this to `~/.zshrc` or `~/.bashrc`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### 4. Add the MCP server to Claude Code

Run this from the project directory:

```bash
claude mcp add bridget -- "$(pwd)/mcp/bridget"
```

### 5. Configure Claude Code hooks

With the `bridget` CLI installed (step 3), add these to `~/.claude/settings.json` under `"hooks"`:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [{ "type": "command", "command": "bridget hook:context", "timeout": 3 }]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [{ "type": "command", "command": "bridget hook:preview", "timeout": 5 }]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [{ "type": "command", "command": "bridget hook:edit", "timeout": 3 }]
      }
    ]
  }
}
```

| Hook | Trigger | What it does |
|------|---------|-------------|
| `bridget hook:context` | Every prompt | Injects editor state (active file, cursor, selection) into context |
| `bridget hook:preview` | Before Edit/Write | Opens a preview diff of the proposed changes in the editor |
| `bridget hook:edit` | After Edit/Write | Closes the preview diff tab once the edit is applied or rejected |

### 6. (Optional) Allow MCP tools in permissions

Add to `~/.claude/settings.json` under `"permissions" > "allow"`:

```json
"mcp__bridget__*"
```

## Usage

### CLI

```bash
bridget sessions                    # List active editor sessions
bridget status                      # Full editor state JSON
bridget file                        # Active file path
bridget selection                   # Current text selection
bridget open /path/to/file.ts 42   # Open file at line 42
bridget reveal /path/to/file.ts 100 # Scroll to line (no focus change)
bridget diff /path/to/file.ts      # Show git diff in editor
bridget diagnostics /path/to/file  # LSP errors/warnings for a file
bridget diagnostics                 # All workspace diagnostics
bridget watch                       # Stream editor state changes (requires fswatch)
bridget bind                        # Bind CWD to an editor session (interactive)
bridget bind Cursor-5800            # Bind CWD to a specific session
bridget unbind                      # Remove CWD binding
bridget bindings                    # List all CWD → session bindings
```

Most commands accept a `-s`/`--session` flag to target a specific editor session:

```bash
bridget file -s Cursor              # Active file in any Cursor session
bridget diff src/main.ts -s 5800   # Route diff to a specific session
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

1. **CWD binding** — `bridget bind` associates your terminal's working directory with a specific editor
2. **Workspace auto-match** — if your CWD is inside an editor's workspace folder, it routes automatically
3. **Most recent** — falls back to the most recently active editor

### Terminal output capture

Requires [shell integration](https://code.visualstudio.com/docs/terminal/shell-integration) (on by default in VS Code/Cursor). Captures command text, stdout/stderr (ANSI stripped), exit code, and working directory.

**Note:** Shell integration only activates for terminals created _after_ the setting is enabled.

### VS Code/Cursor extension detection

When Claude is launched from the VS Code/Cursor Claude extension (not a terminal), `bridget hook:context` automatically skips — the extension already has native editor awareness. `hook:preview` and `hook:edit` still run. Detection uses the `CLAUDE_CODE_SSE_PORT` environment variable.

## Extension settings

| Setting | Default | Description |
|---------|---------|-------------|
| `bridget.enabled` | `true` | Enable/disable the bridge |
| `bridget.debounceMs` | `300` | Debounce for editor state updates (ms) |
| `bridget.selectionDebounceMs` | `150` | Debounce for selection changes (ms) |
| `bridget.maxSelectionLength` | `10000` | Max characters from text selections |
| `bridget.maxTerminalExecutions` | `50` | Max terminal command executions to buffer |
| `bridget.maxTerminalOutputLength` | `20000` | Max characters per terminal command output |

## Development

```bash
make              # Build everything (extension + MCP binary)
make extension    # Build VS Code extension only (tsc)
make mcp          # Build Go MCP binary only
make watch        # Watch mode for extension TypeScript
make package      # Create .vsix package
make clean        # Remove build artifacts
```

After recompiling the extension, reload the editor (`Cmd+Shift+P` → `Developer: Reload Window`).

### Live development (extension symlink)

To avoid reinstalling the `.vsix` on every change, symlink the project directly into the extensions folder:

```bash
# Cursor
ln -s "$(pwd)" ~/.cursor/extensions/raymondsun24.bridget-0.1.0

# VS Code
ln -s "$(pwd)" ~/.vscode/extensions/raymondsun24.bridget-0.1.0
```

Then `make extension` + reload is all you need on each change.
