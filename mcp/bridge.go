package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const staleness = 300 * time.Second

// ── Paths ─────────────────────────────────────────────────────────────────────

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

func bridgeDir() string {
	if d := os.Getenv("BRIDGELY_BRIDGE_DIR"); d != "" {
		return d
	}
	return filepath.Join(homeDir(), ".claude", "bridge")
}

func sessionsDir() string {
	return filepath.Join(bridgeDir(), "sessions")
}

func legacyStateFile() string {
	return filepath.Join(bridgeDir(), "editor-state.json")
}

func ensureBridgeDir() {
	_ = os.MkdirAll(sessionsDir(), 0o700)
}

// ── Session management ────────────────────────────────────────────────────────

func readState(path string) (editorState, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return editorState{}, false
	}
	var state editorState
	if err := json.Unmarshal(data, &state); err != nil {
		return editorState{}, false
	}
	return state, true
}

func isFresh(ts int64) bool {
	return time.Since(time.UnixMilli(ts)) <= staleness
}

func listSessions() []session {
	entries, err := os.ReadDir(sessionsDir())
	if err != nil {
		return nil
	}

	var sessions []session
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") || strings.Contains(name, ".commands") {
			continue
		}
		path := filepath.Join(sessionsDir(), name)
		state, ok := readState(path)
		if !ok || !isFresh(state.Timestamp) {
			continue
		}
		id := state.SessionID
		if id == "" {
			id = strings.TrimSuffix(name, ".json")
		}
		sessions = append(sessions, session{id: id, file: path, state: state})
	}

	// Newest first
	slices.SortFunc(sessions, func(a, b session) int {
		return cmp.Compare(b.state.Timestamp, a.state.Timestamp)
	})
	return sessions
}

func resolveSession(sessionID string) *session {
	sessions := listSessions()

	if sessionID != "" {
		// Exact match
		if i := slices.IndexFunc(sessions, func(s session) bool {
			return s.id == sessionID
		}); i >= 0 {
			return &sessions[i]
		}
		// Partial match
		lower := strings.ToLower(sessionID)
		var matches []int
		for i, s := range sessions {
			if strings.Contains(strings.ToLower(s.id), lower) {
				matches = append(matches, i)
			}
		}
		if len(matches) == 1 {
			return &sessions[matches[0]]
		}
		return nil
	}

	if len(sessions) > 0 {
		return &sessions[0]
	}

	// Legacy fallback
	state, ok := readState(legacyStateFile())
	if ok && isFresh(state.Timestamp) {
		return &session{id: "legacy", file: legacyStateFile(), state: state}
	}
	return nil
}

// ── Command sending ───────────────────────────────────────────────────────────

func sendCommand(s *session, command string, args map[string]any) (commandResult, error) {
	ensureBridgeDir()

	cmdID := fmt.Sprintf("cmd-%d-%d", os.Getpid(), time.Now().UnixMilli())

	var cmdFile, resultFile string
	if s.file == legacyStateFile() {
		cmdFile = filepath.Join(bridgeDir(), "commands.json")
		resultFile = filepath.Join(bridgeDir(), "command-results.json")
	} else {
		cmdFile = filepath.Join(sessionsDir(), s.id+".commands.json")
		resultFile = filepath.Join(sessionsDir(), s.id+".commands-result.json")
	}

	payload := map[string]any{
		"version":   1,
		"id":        cmdID,
		"timestamp": time.Now().UnixMilli(),
		"command":   command,
		"args":      args,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return commandResult{}, fmt.Errorf("marshal command: %w", err)
	}

	tmp := fmt.Sprintf("%s.%d.tmp", cmdFile, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return commandResult{}, fmt.Errorf("write command: %w", err)
	}
	if err := os.Rename(tmp, cmdFile); err != nil {
		return commandResult{}, fmt.Errorf("atomic rename: %w", err)
	}

	// Poll up to 5s (50 × 100ms)
	for range 50 {
		time.Sleep(100 * time.Millisecond)
		data, err := os.ReadFile(resultFile)
		if err != nil {
			continue
		}
		var result commandResult
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}
		if result.ID == cmdID {
			return result, nil
		}
	}
	return commandResult{}, fmt.Errorf("timeout waiting for editor response")
}
