package main

import (
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	ensureBridgeDir()

	s := server.NewMCPServer("bridgely", "0.1.4")

	s.AddTool(mcp.NewTool("editor_sessions",
		mcp.WithDescription("List all active editor sessions (VS Code, Cursor, etc.)"),
	), handleEditorSessions)

	s.AddTool(mcp.NewTool("editor_status",
		mcp.WithDescription("Get the current editor state: active file, cursor position, selection, open tabs, and workspace info"),
		mcp.WithString("sessionId", mcp.Description("Target a specific session by ID (partial match OK). Omit for most recent.")),
	), handleEditorStatus)

	s.AddTool(mcp.NewTool("editor_open_file",
		mcp.WithDescription("Open a file in VS Code/Cursor at a specific line and column"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the file")),
		mcp.WithNumber("line", mcp.Description("Line number (1-based)")),
		mcp.WithNumber("column", mcp.Description("Column number (1-based)")),
		mcp.WithString("sessionId", mcp.Description("Target session ID (partial match OK)")),
	), handleEditorOpenFile)

	s.AddTool(mcp.NewTool("editor_get_selection",
		mcp.WithDescription("Get the current text selection from the editor"),
		mcp.WithString("sessionId", mcp.Description("Target session ID (partial match OK)")),
	), handleEditorGetSelection)

	s.AddTool(mcp.NewTool("editor_get_diagnostics",
		mcp.WithDescription("Get LSP diagnostics (errors, warnings) from the editor for a specific file or the whole workspace"),
		mcp.WithString("path", mcp.Description("Absolute file path (omit for all workspace diagnostics)")),
		mcp.WithString("sessionId", mcp.Description("Target session ID (partial match OK)")),
	), handleEditorGetDiagnostics)

	s.AddTool(mcp.NewTool("editor_reveal_line",
		mcp.WithDescription("Scroll the editor to a specific line in a file without changing focus"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the file")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Line number to reveal (1-based)")),
		mcp.WithString("sessionId", mcp.Description("Target session ID (partial match OK)")),
	), handleEditorRevealLine)

	s.AddTool(mcp.NewTool("editor_bind",
		mcp.WithDescription("Bind a directory to a specific editor session so that session is always chosen when Claude runs in that directory"),
		mcp.WithString("sessionId", mcp.Description("Session to bind to (partial match OK). Omit to use the most recent session.")),
		mcp.WithString("cwd", mcp.Description("Directory to bind (default: current working directory)")),
	), handleEditorBind)

	s.AddTool(mcp.NewTool("editor_unbind",
		mcp.WithDescription("Remove the directory→session binding for the current or specified directory"),
		mcp.WithString("cwd", mcp.Description("Directory to unbind (default: current working directory)")),
	), handleEditorUnbind)

	s.AddTool(mcp.NewTool("editor_list_bindings",
		mcp.WithDescription("List all directory→session bindings"),
	), handleEditorListBindings)

	s.AddTool(mcp.NewTool("editor_preview_edit",
		mcp.WithDescription("Show a preview diff of a proposed edit in the editor before applying it"),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Absolute path to the file being edited")),
		mcp.WithString("tool_name", mcp.Description("'Write' for full file replacement, 'Edit' for string replacement (default: Edit)")),
		mcp.WithString("old_string", mcp.Description("String to replace (Edit operations)")),
		mcp.WithString("new_string", mcp.Description("Replacement string (Edit operations)")),
		mcp.WithString("content", mcp.Description("Full new file content (Write operations)")),
		mcp.WithString("sessionId", mcp.Description("Target session ID (partial match OK)")),
	), handleEditorPreviewEdit)

	s.AddTool(mcp.NewTool("editor_close_preview",
		mcp.WithDescription("Close the preview diff tab in the editor"),
		mcp.WithString("sessionId", mcp.Description("Target session ID (partial match OK)")),
	), handleEditorClosePreview)

	s.AddTool(mcp.NewTool("editor_show_diff",
		mcp.WithDescription("Show the git diff for a file in the editor's SCM view"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the file")),
		mcp.WithString("sessionId", mcp.Description("Target session ID (partial match OK)")),
	), handleEditorShowDiff)

	s.AddTool(mcp.NewTool("editor_get_terminal_output",
		mcp.WithDescription("Get recent terminal command executions and their output from the editor's integrated terminal"),
		mcp.WithString("terminalName", mcp.Description("Filter by terminal name. Omit for all terminals.")),
		mcp.WithNumber("limit", mcp.Description("Max number of recent executions to return (default 10)")),
		mcp.WithString("sessionId", mcp.Description("Target session ID (partial match OK)")),
	), handleEditorGetTerminalOutput)

	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
