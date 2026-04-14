package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func handleEditorSessions(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessions := listSessions()

	if len(sessions) == 0 {
		state, ok := readState(legacyStateFile())
		if ok && isFresh(state.Timestamp) {
			return mcp.NewToolResultText(fmt.Sprintf("1 session (legacy):\n  %s — %s — %s",
				state.IDEName,
				state.Workspace.Name,
				state.ActiveFile.RelativePath,
			)), nil
		}
		return mcp.NewToolResultText("No active editor sessions found."), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d active session(s):\n", len(sessions))
	for _, s := range sessions {
		fmt.Fprintf(&sb, "\n  %s\n    IDE: %s\n    Workspace: %s\n    File: %s\n    Cursor: line %d, col %d\n",
			s.id,
			s.state.IDEName,
			s.state.Workspace.Name,
			s.state.ActiveFile.RelativePath,
			s.state.ActiveFile.CursorLine,
			s.state.ActiveFile.CursorColumn,
		)
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleEditorStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sid, _ := req.GetArguments()["sessionId"].(string)
	s := resolveSession(sid)
	if s == nil {
		if sid != "" {
			return mcp.NewToolResultText("No session found. Use editor_sessions to list active sessions."), nil
		}
		return mcp.NewToolResultText("No editor state found. Is the VS Code/Cursor extension running?"), nil
	}

	st := s.state
	var sb strings.Builder
	fmt.Fprintf(&sb, "Session: %s\nIDE: %s\nWorkspace: %s\n", s.id, st.IDEName, st.Workspace.Name)

	if st.ActiveFile.Path != "" {
		fmt.Fprintf(&sb, "\nActive file: %s\n  Language: %s\n  Cursor: line %d, col %d\n  Lines: %d\n",
			st.ActiveFile.RelativePath,
			st.ActiveFile.LanguageID,
			st.ActiveFile.CursorLine,
			st.ActiveFile.CursorColumn,
			st.ActiveFile.LineCount,
		)
	} else {
		sb.WriteString("\nNo active file\n")
	}

	if st.Selection.Text != "" {
		fmt.Fprintf(&sb, "\nSelection (lines %d-%d):\n", st.Selection.StartLine, st.Selection.EndLine)
		if len(st.Selection.Text) > 2000 {
			sb.WriteString(st.Selection.Text[:2000])
			sb.WriteString("\n... (truncated)")
		} else {
			sb.WriteString(st.Selection.Text)
		}
		sb.WriteByte('\n')
	}

	if len(st.VisibleFiles) > 0 {
		sb.WriteString("\nOpen tabs:\n")
		for _, f := range st.VisibleFiles {
			dirty, active := "", ""
			if f.IsDirty {
				dirty = " [modified]"
			}
			if f.IsActive {
				active = " *"
			}
			fmt.Fprintf(&sb, "  %s%s%s\n", f.Path, dirty, active)
		}
	}

	if len(st.Terminals) > 0 {
		sb.WriteString("\nTerminals:\n")
		for _, t := range st.Terminals {
			act, shell := "", ""
			if t.IsActive {
				act = " *"
			}
			if !t.HasShellIntegration {
				shell = " [no shell integration]"
			}
			fmt.Fprintf(&sb, "  %s%s%s\n", t.Name, act, shell)
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleEditorOpenFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, _ := req.GetArguments()["path"].(string)
	if path == "" {
		return mcp.NewToolResultText("Missing required parameter: path"), nil
	}
	sid, _ := req.GetArguments()["sessionId"].(string)
	s := resolveSession(sid)
	if s == nil {
		return mcp.NewToolResultText("No active editor session found."), nil
	}

	args := map[string]any{"path": path, "preview": false}
	if line, ok := req.GetArguments()["line"]; ok {
		args["line"] = line
	}
	if col, ok := req.GetArguments()["column"]; ok {
		args["column"] = col
	}

	result, err := sendCommand(s, "openFile", args)
	if err != nil {
		return mcp.NewToolResultText("Failed to open file: " + err.Error()), nil
	}
	return mcp.NewToolResultText(result.Result.Message), nil
}

func handleEditorGetSelection(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sid, _ := req.GetArguments()["sessionId"].(string)
	s := resolveSession(sid)
	if s == nil {
		return mcp.NewToolResultText("No editor state found."), nil
	}

	sel := s.state.Selection
	if sel.Text == "" {
		return mcp.NewToolResultText("(no selection)"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Session: %s\nFile: %s\nLines %d-%d:\n\n%s",
		s.id,
		s.state.ActiveFile.RelativePath,
		sel.StartLine,
		sel.EndLine,
		sel.Text,
	)), nil
}

func handleEditorGetDiagnostics(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sid, _ := req.GetArguments()["sessionId"].(string)
	s := resolveSession(sid)
	if s == nil {
		return mcp.NewToolResultText("No active editor session found."), nil
	}

	args := map[string]any{}
	if p, _ := req.GetArguments()["path"].(string); p != "" {
		args["path"] = p
	}

	result, err := sendCommand(s, "getDiagnostics", args)
	if err != nil {
		return mcp.NewToolResultText("Failed to get diagnostics: " + err.Error()), nil
	}

	var files []diagnosticFile
	if err := json.Unmarshal(result.Result.Data, &files); err != nil || len(files) == 0 {
		return mcp.NewToolResultText("No diagnostics found."), nil
	}

	var sb strings.Builder
	for _, f := range files {
		fmt.Fprintf(&sb, "%s:\n", f.Path)
		for _, d := range f.Diagnostics {
			fmt.Fprintf(&sb, "  Line %d [%s]: %s", d.Line, d.Severity, d.Message)
			if d.Source != "" {
				fmt.Fprintf(&sb, " (%s)", d.Source)
			}
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}
	return mcp.NewToolResultText(strings.TrimRight(sb.String(), "\n")), nil
}

func handleEditorRevealLine(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, _ := req.GetArguments()["path"].(string)
	line := req.GetArguments()["line"]
	if path == "" || line == nil {
		return mcp.NewToolResultText("Missing required parameters: path and line"), nil
	}
	sid, _ := req.GetArguments()["sessionId"].(string)
	s := resolveSession(sid)
	if s == nil {
		return mcp.NewToolResultText("No active editor session found."), nil
	}

	result, err := sendCommand(s, "revealLine", map[string]any{"path": path, "line": line})
	if err != nil {
		return mcp.NewToolResultText("Failed to reveal line: " + err.Error()), nil
	}
	return mcp.NewToolResultText(result.Result.Message), nil
}

func handleEditorGetTerminalOutput(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sid, _ := req.GetArguments()["sessionId"].(string)
	s := resolveSession(sid)
	if s == nil {
		return mcp.NewToolResultText("No active editor session found."), nil
	}

	args := map[string]any{}
	if name, _ := req.GetArguments()["terminalName"].(string); name != "" {
		args["terminalName"] = name
	}
	if limit, ok := req.GetArguments()["limit"]; ok {
		args["limit"] = limit
	}

	result, err := sendCommand(s, "getTerminalOutput", args)
	if err != nil {
		return mcp.NewToolResultText("Failed to get terminal output: " + err.Error()), nil
	}

	if result.Result.Data == nil {
		return mcp.NewToolResultText(result.Result.Message), nil
	}

	var data terminalData
	if err := json.Unmarshal(result.Result.Data, &data); err != nil {
		return mcp.NewToolResultText(result.Result.Message), nil
	}

	var sb strings.Builder
	if len(data.Terminals) > 0 {
		fmt.Fprintf(&sb, "Terminals (%d):\n", len(data.Terminals))
		for _, t := range data.Terminals {
			act, shell := "", ""
			if t.IsActive {
				act = " *"
			}
			if !t.HasShellIntegration {
				shell = " [no shell integration]"
			}
			fmt.Fprintf(&sb, "  %s%s%s\n", t.Name, act, shell)
		}
	}

	if len(data.Executions) > 0 {
		fmt.Fprintf(&sb, "\nRecent executions (%d):\n", len(data.Executions))
		for _, e := range data.Executions {
			exit, cwd := "", ""
			if e.ExitCode != nil {
				exit = fmt.Sprintf(" [exit %d]", *e.ExitCode)
			}
			if e.CWD != "" {
				cwd = fmt.Sprintf(" (%s)", e.CWD)
			}
			fmt.Fprintf(&sb, "\n--- %s: $ %s%s%s ---\n", e.TerminalName, e.Command, exit, cwd)
			if e.Output != "" {
				sb.WriteString(e.Output)
			} else {
				sb.WriteString("(no output)\n")
			}
		}
	} else {
		sb.WriteString("\nNo recent terminal executions captured. Shell integration must be active.")
	}

	return mcp.NewToolResultText(strings.TrimRight(sb.String(), "\n")), nil
}
