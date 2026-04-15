package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// emptyReq returns a zero-value CallToolRequest (no arguments set).
func emptyReq() mcp.CallToolRequest {
	return mcp.CallToolRequest{}
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
