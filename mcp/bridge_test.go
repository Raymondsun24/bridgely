package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupBridgeDir creates a temp bridge dir with a sessions/ subdir and sets
// the env var so all bridge functions use it for the duration of the test.
func setupBridgeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BRIDGELY_BRIDGE_DIR", dir)
	return dir
}

// writeSessionFile marshals state to dir/sessions/<id>.json.
func writeSessionFile(t *testing.T, dir string, state editorState) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sessions", state.SessionID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// ── isFresh ──────────────────────────────────────────────────────────────────

func TestIsFresh(t *testing.T) {
	now := time.Now().UnixMilli()

	tests := []struct {
		name  string
		ts    int64
		fresh bool
	}{
		{"now", now, true},
		{"299s ago", time.Now().Add(-299 * time.Second).UnixMilli(), true},
		{"301s ago", time.Now().Add(-301 * time.Second).UnixMilli(), false},
		{"1 hour ago", time.Now().Add(-1 * time.Hour).UnixMilli(), false},
		{"zero timestamp", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFresh(tc.ts); got != tc.fresh {
				t.Errorf("isFresh(%d) = %v, want %v", tc.ts, got, tc.fresh)
			}
		})
	}
}

// ── readState ─────────────────────────────────────────────────────────────────

func TestReadState(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid JSON", func(t *testing.T) {
		state := editorState{
			SessionID: "VSCode-1234",
			Timestamp: time.Now().UnixMilli(),
			IDEName:   "VSCode",
			Workspace: workspace{Name: "myproject"},
			ActiveFile: activeFile{
				RelativePath: "main.go",
				LanguageID:   "go",
				CursorLine:   10,
				CursorColumn: 5,
			},
		}
		data, _ := json.Marshal(state)
		path := filepath.Join(dir, "session.json")
		os.WriteFile(path, data, 0o600)

		got, ok := readState(path)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got.SessionID != "VSCode-1234" {
			t.Errorf("SessionID = %q, want %q", got.SessionID, "VSCode-1234")
		}
		if got.Workspace.Name != "myproject" {
			t.Errorf("Workspace.Name = %q, want %q", got.Workspace.Name, "myproject")
		}
		if got.ActiveFile.CursorLine != 10 {
			t.Errorf("CursorLine = %d, want 10", got.ActiveFile.CursorLine)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, ok := readState(filepath.Join(dir, "nonexistent.json"))
		if ok {
			t.Error("expected ok=false for missing file")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		path := filepath.Join(dir, "bad.json")
		os.WriteFile(path, []byte("not json {{"), 0o600)
		_, ok := readState(path)
		if ok {
			t.Error("expected ok=false for invalid JSON")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(dir, "empty.json")
		os.WriteFile(path, []byte(""), 0o600)
		_, ok := readState(path)
		if ok {
			t.Error("expected ok=false for empty file")
		}
	})
}

// ── listSessions ──────────────────────────────────────────────────────────────

func TestListSessions(t *testing.T) {
	t.Run("empty dir", func(t *testing.T) {
		setupBridgeDir(t)
		sessions := listSessions()
		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(sessions))
		}
	})

	t.Run("single fresh session", func(t *testing.T) {
		dir := setupBridgeDir(t)
		writeSessionFile(t, dir, editorState{
			SessionID: "VSCode-100",
			Timestamp: time.Now().UnixMilli(),
			IDEName:   "VSCode",
		})

		sessions := listSessions()
		if len(sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(sessions))
		}
		if sessions[0].id != "VSCode-100" {
			t.Errorf("id = %q, want %q", sessions[0].id, "VSCode-100")
		}
	})

	t.Run("stale sessions excluded", func(t *testing.T) {
		dir := setupBridgeDir(t)
		writeSessionFile(t, dir, editorState{
			SessionID: "Fresh-1",
			Timestamp: time.Now().UnixMilli(),
		})
		writeSessionFile(t, dir, editorState{
			SessionID: "Stale-1",
			Timestamp: time.Now().Add(-10 * time.Minute).UnixMilli(),
		})

		sessions := listSessions()
		if len(sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(sessions))
		}
		if sessions[0].id != "Fresh-1" {
			t.Errorf("expected Fresh-1, got %s", sessions[0].id)
		}
	})

	t.Run("sorted newest first", func(t *testing.T) {
		dir := setupBridgeDir(t)
		now := time.Now()
		writeSessionFile(t, dir, editorState{
			SessionID: "Older",
			Timestamp: now.Add(-60 * time.Second).UnixMilli(),
		})
		writeSessionFile(t, dir, editorState{
			SessionID: "Newer",
			Timestamp: now.Add(-10 * time.Second).UnixMilli(),
		})

		sessions := listSessions()
		if len(sessions) != 2 {
			t.Fatalf("expected 2 sessions, got %d", len(sessions))
		}
		if sessions[0].id != "Newer" {
			t.Errorf("expected Newer first, got %s", sessions[0].id)
		}
		if sessions[1].id != "Older" {
			t.Errorf("expected Older second, got %s", sessions[1].id)
		}
	})

	t.Run("command files excluded", func(t *testing.T) {
		dir := setupBridgeDir(t)
		writeSessionFile(t, dir, editorState{
			SessionID: "VSCode-200",
			Timestamp: time.Now().UnixMilli(),
		})
		// Write a commands file that should be ignored
		cmdPath := filepath.Join(dir, "sessions", "VSCode-200.commands.json")
		os.WriteFile(cmdPath, []byte(`{}`), 0o600)

		sessions := listSessions()
		if len(sessions) != 1 {
			t.Fatalf("expected 1 session (commands file ignored), got %d", len(sessions))
		}
	})
}

// ── resolveSession ────────────────────────────────────────────────────────────

func TestResolveSession(t *testing.T) {
	t.Run("no sessions returns nil", func(t *testing.T) {
		setupBridgeDir(t)
		if s := resolveSession(""); s != nil {
			t.Errorf("expected nil, got %+v", s)
		}
	})

	t.Run("empty ID returns most recent", func(t *testing.T) {
		dir := setupBridgeDir(t)
		now := time.Now()
		writeSessionFile(t, dir, editorState{SessionID: "Older", Timestamp: now.Add(-60 * time.Second).UnixMilli()})
		writeSessionFile(t, dir, editorState{SessionID: "Newer", Timestamp: now.UnixMilli()})

		s := resolveSession("")
		if s == nil {
			t.Fatal("expected a session, got nil")
			return
		}
		if s.id != "Newer" {
			t.Errorf("expected Newer, got %s", s.id)
		}
	})

	t.Run("exact ID match", func(t *testing.T) {
		dir := setupBridgeDir(t)
		writeSessionFile(t, dir, editorState{SessionID: "VSCode-42", Timestamp: time.Now().UnixMilli()})
		writeSessionFile(t, dir, editorState{SessionID: "Cursor-99", Timestamp: time.Now().UnixMilli()})

		s := resolveSession("VSCode-42")
		if s == nil {
			t.Fatal("expected a session, got nil")
			return
		}
		if s.id != "VSCode-42" {
			t.Errorf("expected VSCode-42, got %s", s.id)
		}
	})

	t.Run("partial ID match", func(t *testing.T) {
		dir := setupBridgeDir(t)
		writeSessionFile(t, dir, editorState{SessionID: "VSCode-42", Timestamp: time.Now().UnixMilli()})
		writeSessionFile(t, dir, editorState{SessionID: "Cursor-99", Timestamp: time.Now().UnixMilli()})

		s := resolveSession("Cursor")
		if s == nil {
			t.Fatal("expected a session, got nil")
			return
		}
		if s.id != "Cursor-99" {
			t.Errorf("expected Cursor-99, got %s", s.id)
		}
	})

	t.Run("ambiguous partial match returns nil", func(t *testing.T) {
		dir := setupBridgeDir(t)
		writeSessionFile(t, dir, editorState{SessionID: "VSCode-1", Timestamp: time.Now().UnixMilli()})
		writeSessionFile(t, dir, editorState{SessionID: "VSCode-2", Timestamp: time.Now().UnixMilli()})

		s := resolveSession("VSCode")
		if s != nil {
			t.Errorf("expected nil for ambiguous match, got %s", s.id)
		}
	})

	t.Run("no match returns nil", func(t *testing.T) {
		dir := setupBridgeDir(t)
		writeSessionFile(t, dir, editorState{SessionID: "VSCode-42", Timestamp: time.Now().UnixMilli()})

		s := resolveSession("Cursor")
		if s != nil {
			t.Errorf("expected nil for no match, got %s", s.id)
		}
	})

	t.Run("legacy fallback", func(t *testing.T) {
		dir := setupBridgeDir(t)
		// No session files — write a fresh legacy state file
		state := editorState{
			SessionID: "",
			Timestamp: time.Now().UnixMilli(),
			IDEName:   "VSCode",
		}
		data, _ := json.Marshal(state)
		os.WriteFile(filepath.Join(dir, "editor-state.json"), data, 0o600)

		s := resolveSession("")
		if s == nil {
			t.Fatal("expected legacy session, got nil")
			return
		}
		if s.id != "legacy" {
			t.Errorf("expected id=legacy, got %s", s.id)
		}
	})

	t.Run("stale legacy not returned", func(t *testing.T) {
		dir := setupBridgeDir(t)
		state := editorState{
			Timestamp: time.Now().Add(-10 * time.Minute).UnixMilli(),
		}
		data, _ := json.Marshal(state)
		os.WriteFile(filepath.Join(dir, "editor-state.json"), data, 0o600)

		s := resolveSession("")
		if s != nil {
			t.Errorf("expected nil for stale legacy, got %+v", s)
		}
	})
}
