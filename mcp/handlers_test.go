package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// emptyReq returns a zero-value CallToolRequest (no arguments set).
func emptyReq() mcp.CallToolRequest {
	return mcp.CallToolRequest{}
}

// reqWith returns a CallToolRequest populated with the given arguments.
func reqWith(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

// ── handleEditorSessions ──────────────────────────────────────────────────────

func TestHandleEditorSessions_NoSessions(t *testing.T) {
	setupBridgeDir(t) // empty dir
	result, err := handleEditorSessions(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "No active editor sessions") {
		t.Errorf("expected no-sessions message, got: %q", text)
	}
}

func TestHandleEditorSessions_SingleSession(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{
		SessionID: "VSCode-1",
		Timestamp: time.Now().UnixMilli(),
		IDEName:   "VSCode",
		Workspace: workspace{Name: "myproject"},
		ActiveFile: activeFile{
			RelativePath: "main.go",
			CursorLine:   5,
			CursorColumn: 10,
		},
	})

	result, err := handleEditorSessions(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "1 active session") {
		t.Errorf("expected session count, got: %q", text)
	}
	if !strings.Contains(text, "VSCode-1") {
		t.Errorf("expected session ID, got: %q", text)
	}
	if !strings.Contains(text, "myproject") {
		t.Errorf("expected workspace name, got: %q", text)
	}
}

func TestHandleEditorSessions_MultipleSessions(t *testing.T) {
	dir := setupBridgeDir(t)
	now := time.Now()
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-1", Timestamp: now.UnixMilli(), IDEName: "VSCode"})
	writeSessionFile(t, dir, editorState{SessionID: "Cursor-2", Timestamp: now.Add(-30 * time.Second).UnixMilli(), IDEName: "Cursor"})

	result, err := handleEditorSessions(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "2 active session") {
		t.Errorf("expected 2 sessions, got: %q", text)
	}
	if !strings.Contains(text, "VSCode-1") || !strings.Contains(text, "Cursor-2") {
		t.Errorf("expected both session IDs, got: %q", text)
	}
}

// ── handleEditorStatus ────────────────────────────────────────────────────────

func TestHandleEditorStatus_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorStatus(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "No editor state") {
		t.Errorf("expected no-state message, got: %q", text)
	}
}

func TestHandleEditorStatus_WithSession(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{
		SessionID: "VSCode-5",
		Timestamp: time.Now().UnixMilli(),
		IDEName:   "VSCode",
		Workspace: workspace{Name: "bridgely"},
		ActiveFile: activeFile{
			Path:         "/home/user/bridgely/main.go",
			RelativePath: "main.go",
			LanguageID:   "go",
			CursorLine:   42,
			CursorColumn: 7,
			LineCount:    100,
		},
	})

	result, err := handleEditorStatus(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	for _, want := range []string{"VSCode-5", "bridgely", "main.go", "go", "42", "100"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in output, got: %q", want, text)
		}
	}
}

func TestHandleEditorStatus_SelectionTruncated(t *testing.T) {
	dir := setupBridgeDir(t)
	longText := strings.Repeat("x", 3000)
	writeSessionFile(t, dir, editorState{
		SessionID:  "VSCode-6",
		Timestamp:  time.Now().UnixMilli(),
		ActiveFile: activeFile{Path: "/file.go", RelativePath: "file.go"},
		Selection: selection{
			Text:      longText,
			StartLine: 1,
			EndLine:   100,
		},
	})

	result, err := handleEditorStatus(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "truncated") {
		t.Errorf("expected truncation notice for long selection, got: %q", text)
	}
	if strings.Contains(text, longText) {
		t.Error("full long text should not appear — should be truncated")
	}
}

func TestHandleEditorStatus_ShortSelectionNotTruncated(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{
		SessionID:  "VSCode-7",
		Timestamp:  time.Now().UnixMilli(),
		ActiveFile: activeFile{Path: "/file.go", RelativePath: "file.go"},
		Selection: selection{
			Text:      "hello world",
			StartLine: 5,
			EndLine:   5,
		},
	})

	result, err := handleEditorStatus(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if strings.Contains(text, "truncated") {
		t.Error("short selection should not be truncated")
	}
	if !strings.Contains(text, "hello world") {
		t.Errorf("expected selection text in output, got: %q", text)
	}
}

func TestHandleEditorStatus_NoActiveFile(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{
		SessionID: "VSCode-8",
		Timestamp: time.Now().UnixMilli(),
		IDEName:   "VSCode",
	})

	result, err := handleEditorStatus(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "No active file") {
		t.Errorf("expected 'No active file', got: %q", text)
	}
}

// ── handleEditorGetSelection ──────────────────────────────────────────────────

func TestHandleEditorGetSelection_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorGetSelection(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "No editor state") {
		t.Errorf("expected no-state message, got: %q", text)
	}
}

func TestHandleEditorGetSelection_NoSelection(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{
		SessionID: "VSCode-9",
		Timestamp: time.Now().UnixMilli(),
	})

	result, err := handleEditorGetSelection(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "no selection") {
		t.Errorf("expected '(no selection)', got: %q", text)
	}
}

func TestHandleEditorGetSelection_WithSelection(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{
		SessionID:  "VSCode-10",
		Timestamp:  time.Now().UnixMilli(),
		ActiveFile: activeFile{RelativePath: "main.go"},
		Selection: selection{
			Text:      "func main() {}",
			StartLine: 10,
			EndLine:   10,
		},
	})

	result, err := handleEditorGetSelection(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "func main() {}") {
		t.Errorf("expected selection text, got: %q", text)
	}
	if !strings.Contains(text, "main.go") {
		t.Errorf("expected file name, got: %q", text)
	}
}

// ── handleEditorBind / Unbind / ListBindings ──────────────────────────────────

func TestHandleEditorBind_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorBind(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No active editor session") {
		t.Errorf("expected no-session message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorBind_BindsCurrentDir(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-bind", Timestamp: time.Now().UnixMilli()})

	result, err := handleEditorBind(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "VSCode-bind") {
		t.Errorf("expected session ID in output, got: %q", text)
	}

	bindings := readBindings()
	cwd, _ := os.Getwd()
	if bindings[cwd] != "VSCode-bind" {
		t.Errorf("binding not persisted: %v", bindings)
	}
}

func TestHandleEditorBind_BindsExplicitDir(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-explicit", Timestamp: time.Now().UnixMilli()})

	result, err := handleEditorBind(context.Background(), reqWith(map[string]any{"cwd": "/custom/path"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "VSCode-explicit") {
		t.Errorf("expected session ID in output, got: %q", extractText(t, result))
	}
	if readBindings()["/custom/path"] != "VSCode-explicit" {
		t.Errorf("expected /custom/path binding, got: %v", readBindings())
	}
}

func TestHandleEditorUnbind_NoBinding(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorUnbind(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No binding") {
		t.Errorf("expected no-binding message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorUnbind_RemovesBinding(t *testing.T) {
	dir := setupBridgeDir(t)
	cwd, _ := os.Getwd()
	data, _ := json.Marshal(map[string]string{cwd: "VSCode-1", "/other": "Cursor-2"})
	os.WriteFile(filepath.Join(dir, "bindings.json"), data, 0o600)

	result, err := handleEditorUnbind(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "VSCode-1") {
		t.Errorf("expected removed session in output, got: %q", extractText(t, result))
	}

	bindings := readBindings()
	if _, ok := bindings[cwd]; ok {
		t.Error("binding should have been removed")
	}
	if bindings["/other"] != "Cursor-2" {
		t.Error("other binding should be preserved")
	}
}

func TestHandleEditorListBindings_Empty(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorListBindings(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No bindings") {
		t.Errorf("expected empty message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorListBindings_ShowsAll(t *testing.T) {
	dir := setupBridgeDir(t)
	cwd, _ := os.Getwd()
	data, _ := json.Marshal(map[string]string{cwd: "VSCode-1", "/other/path": "Cursor-2"})
	os.WriteFile(filepath.Join(dir, "bindings.json"), data, 0o600)

	result, err := handleEditorListBindings(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "VSCode-1") || !strings.Contains(text, "Cursor-2") {
		t.Errorf("expected both session IDs, got: %q", text)
	}
	if !strings.Contains(text, "current directory") {
		t.Errorf("expected current directory marker, got: %q", text)
	}
}

// ── handleEditorOpenFile ──────────────────────────────────────────────────────

func TestHandleEditorOpenFile_MissingPath(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorOpenFile(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "Missing required parameter: path") {
		t.Errorf("expected missing-path error, got: %q", extractText(t, result))
	}
}

func TestHandleEditorOpenFile_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorOpenFile(context.Background(), reqWith(map[string]any{"path": "/foo/bar.go"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No active editor session") {
		t.Errorf("expected no-session message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorOpenFile_Success(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-open", Timestamp: time.Now().UnixMilli()})
	simulateEditorResponse(t, dir, "VSCode-open", commandPayload{Message: "Opened bar.go at line 5"})

	result, err := handleEditorOpenFile(context.Background(), reqWith(map[string]any{
		"path": "/foo/bar.go",
		"line": float64(5),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "bar.go") {
		t.Errorf("expected file name in response, got: %q", extractText(t, result))
	}
}

// ── handleEditorRevealLine ────────────────────────────────────────────────────

func TestHandleEditorRevealLine_MissingParams(t *testing.T) {
	setupBridgeDir(t)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"no args", map[string]any{}},
		{"missing line", map[string]any{"path": "/foo.go"}},
		{"missing path", map[string]any{"line": float64(10)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := handleEditorRevealLine(context.Background(), reqWith(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(extractText(t, result), "Missing required parameters") {
				t.Errorf("expected missing-params error, got: %q", extractText(t, result))
			}
		})
	}
}

func TestHandleEditorRevealLine_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorRevealLine(context.Background(), reqWith(map[string]any{
		"path": "/foo.go",
		"line": float64(10),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No active editor session") {
		t.Errorf("expected no-session message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorRevealLine_Success(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-reveal", Timestamp: time.Now().UnixMilli()})
	simulateEditorResponse(t, dir, "VSCode-reveal", commandPayload{Message: "Revealed line 42"})

	result, err := handleEditorRevealLine(context.Background(), reqWith(map[string]any{
		"path": "/foo.go",
		"line": float64(42),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "Revealed line 42") {
		t.Errorf("expected reveal confirmation, got: %q", extractText(t, result))
	}
}

// ── handleEditorGetDiagnostics ────────────────────────────────────────────────

func TestHandleEditorGetDiagnostics_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorGetDiagnostics(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No active editor session") {
		t.Errorf("expected no-session message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorGetDiagnostics_Empty(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-diag-empty", Timestamp: time.Now().UnixMilli()})

	emptyData, _ := json.Marshal([]diagnosticFile{})
	simulateEditorResponse(t, dir, "VSCode-diag-empty", commandPayload{
		Message: "0 file(s) with diagnostics",
		Data:    json.RawMessage(emptyData),
	})

	result, err := handleEditorGetDiagnostics(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No diagnostics found") {
		t.Errorf("expected no-diagnostics message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorGetDiagnostics_WithDiagnostics(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-diag", Timestamp: time.Now().UnixMilli()})

	files := []diagnosticFile{
		{
			Path: "/src/main.go",
			Diagnostics: []diagnostic{
				{Line: 10, Severity: "Error", Message: "undefined: foo", Source: "compiler"},
				{Line: 20, Severity: "Warning", Message: "unused import", Source: "staticcheck"},
			},
		},
	}
	diagData, _ := json.Marshal(files)
	simulateEditorResponse(t, dir, "VSCode-diag", commandPayload{
		Message: "1 file(s) with diagnostics",
		Data:    json.RawMessage(diagData),
	})

	result, err := handleEditorGetDiagnostics(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	for _, want := range []string{"main.go", "Line 10", "Error", "undefined: foo", "Line 20", "Warning", "unused import"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in output, got: %q", want, text)
		}
	}
}

// ── handleEditorGetTerminalOutput ─────────────────────────────────────────────

func TestHandleEditorGetTerminalOutput_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorGetTerminalOutput(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No active editor session") {
		t.Errorf("expected no-session message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorGetTerminalOutput_WithData(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-term", Timestamp: time.Now().UnixMilli()})

	exitCode := 0
	data := terminalData{
		Terminals: []terminal{{Name: "bash", IsActive: true, HasShellIntegration: true}},
		Executions: []execution{{
			TerminalName: "bash",
			Command:      "go test ./...",
			Output:       "ok  bridgely",
			ExitCode:     &exitCode,
			CWD:          "/home/user/bridgely",
		}},
	}
	termJSON, _ := json.Marshal(data)
	simulateEditorResponse(t, dir, "VSCode-term", commandPayload{
		Message: "1 terminal(s), 1 recent execution(s)",
		Data:    json.RawMessage(termJSON),
	})

	result, err := handleEditorGetTerminalOutput(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(t, result)
	for _, want := range []string{"bash", "go test ./...", "ok  bridgely", "exit 0"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in output, got: %q", want, text)
		}
	}
}

// ── handleEditorPreviewEdit ───────────────────────────────────────────────────

func TestHandleEditorPreviewEdit_MissingFilePath(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorPreviewEdit(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "Missing required parameter: file_path") {
		t.Errorf("expected missing file_path error, got: %q", extractText(t, result))
	}
}

func TestHandleEditorPreviewEdit_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorPreviewEdit(context.Background(), reqWith(map[string]any{
		"file_path": "/src/main.go",
		"tool_name": "Edit",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No active editor session") {
		t.Errorf("expected no-session message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorPreviewEdit_EditSuccess(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-preview", Timestamp: time.Now().UnixMilli()})
	simulateEditorResponse(t, dir, "VSCode-preview", commandPayload{Message: "Previewing edit for main.go"})

	result, err := handleEditorPreviewEdit(context.Background(), reqWith(map[string]any{
		"file_path":  "/src/main.go",
		"tool_name":  "Edit",
		"old_string": "func hello()",
		"new_string": "func greet()",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "main.go") {
		t.Errorf("expected file name in response, got: %q", extractText(t, result))
	}
}

func TestHandleEditorPreviewEdit_WriteSuccess(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-preview-write", Timestamp: time.Now().UnixMilli()})
	simulateEditorResponse(t, dir, "VSCode-preview-write", commandPayload{Message: "Previewing edit for main.go"})

	result, err := handleEditorPreviewEdit(context.Background(), reqWith(map[string]any{
		"file_path": "/src/main.go",
		"tool_name": "Write",
		"content":   "package main\n\nfunc main() {}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "main.go") {
		t.Errorf("expected file name in response, got: %q", extractText(t, result))
	}
}

func TestHandleEditorPreviewEdit_DefaultsToolNameToEdit(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-preview-default", Timestamp: time.Now().UnixMilli()})

	cmdPath := filepath.Join(dir, "sessions", "VSCode-preview-default.commands.json")
	resultPath := filepath.Join(dir, "sessions", "VSCode-preview-default.commands-result.json")

	capturedArgs := make(chan map[string]any, 1)
	go func() {
		for range 200 {
			time.Sleep(10 * time.Millisecond)
			data, err := os.ReadFile(cmdPath)
			if err != nil || len(data) == 0 {
				continue
			}
			var cmd struct {
				ID   string         `json:"id"`
				Args map[string]any `json:"args"`
			}
			if err := json.Unmarshal(data, &cmd); err != nil || cmd.ID == "" {
				continue
			}
			capturedArgs <- cmd.Args
			result := commandResult{ID: cmd.ID, Result: commandPayload{Message: "ok"}}
			out, _ := json.Marshal(result)
			_ = os.WriteFile(resultPath, out, 0o600)
			return
		}
	}()

	if _, err := handleEditorPreviewEdit(context.Background(), reqWith(map[string]any{
		"file_path": "/src/main.go",
	})); err != nil {
		t.Fatal(err)
	}

	select {
	case args := <-capturedArgs:
		if args["tool_name"] != "Edit" {
			t.Errorf("tool_name = %v, want Edit", args["tool_name"])
		}
	default:
		t.Error("no command args captured")
	}
}

// ── handleEditorClosePreview ──────────────────────────────────────────────────

func TestHandleEditorClosePreview_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorClosePreview(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No active editor session") {
		t.Errorf("expected no-session message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorClosePreview_Success(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-close", Timestamp: time.Now().UnixMilli()})
	simulateEditorResponse(t, dir, "VSCode-close", commandPayload{Message: "Preview closed"})

	result, err := handleEditorClosePreview(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "Preview closed") {
		t.Errorf("expected close confirmation, got: %q", extractText(t, result))
	}
}

// ── handleEditorShowDiff ──────────────────────────────────────────────────────

func TestHandleEditorShowDiff_MissingPath(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorShowDiff(context.Background(), emptyReq())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "Missing required parameter: path") {
		t.Errorf("expected missing-path error, got: %q", extractText(t, result))
	}
}

func TestHandleEditorShowDiff_NoSession(t *testing.T) {
	setupBridgeDir(t)
	result, err := handleEditorShowDiff(context.Background(), reqWith(map[string]any{"path": "/src/main.go"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "No active editor session") {
		t.Errorf("expected no-session message, got: %q", extractText(t, result))
	}
}

func TestHandleEditorShowDiff_Success(t *testing.T) {
	dir := setupBridgeDir(t)
	writeSessionFile(t, dir, editorState{SessionID: "VSCode-diff", Timestamp: time.Now().UnixMilli()})
	simulateEditorResponse(t, dir, "VSCode-diff", commandPayload{Message: "Showing diff for main.go"})

	result, err := handleEditorShowDiff(context.Background(), reqWith(map[string]any{"path": "/src/main.go"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractText(t, result), "main.go") {
		t.Errorf("expected file name in response, got: %q", extractText(t, result))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// extractText pulls the text content from an MCP tool result.
func extractText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
		return ""
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatal("no text content in result")
	return ""
}
