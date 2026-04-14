package main

import "encoding/json"

type editorState struct {
	SessionID    string     `json:"sessionId"`
	Timestamp    int64      `json:"timestamp"`
	IDEName      string     `json:"ideName"`
	Workspace    workspace  `json:"workspace"`
	ActiveFile   activeFile `json:"activeFile"`
	Selection    selection  `json:"selection"`
	VisibleFiles []fileInfo `json:"visibleFiles"`
	Terminals    []terminal `json:"terminals"`
}

type workspace struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type activeFile struct {
	Path         string `json:"path"`
	RelativePath string `json:"relativePath"`
	LanguageID   string `json:"languageId"`
	CursorLine   int    `json:"cursorLine"`
	CursorColumn int    `json:"cursorColumn"`
	LineCount    int    `json:"lineCount"`
}

type selection struct {
	Text      string `json:"text"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
}

type fileInfo struct {
	Path     string `json:"path"`
	IsDirty  bool   `json:"isDirty"`
	IsActive bool   `json:"isActive"`
}

type terminal struct {
	Name                string `json:"name"`
	IsActive            bool   `json:"isActive"`
	HasShellIntegration bool   `json:"hasShellIntegration"`
}

type session struct {
	id    string
	file  string
	state editorState
}

type commandResult struct {
	ID     string         `json:"id"`
	Result commandPayload `json:"result"`
}

type commandPayload struct {
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type diagnosticFile struct {
	Path        string       `json:"path"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

type diagnostic struct {
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source"`
}

type terminalData struct {
	Terminals  []terminal  `json:"terminals"`
	Executions []execution `json:"executions"`
}

type execution struct {
	TerminalName string `json:"terminalName"`
	Command      string `json:"command"`
	Output       string `json:"output"`
	ExitCode     *int   `json:"exitCode"`
	CWD          string `json:"cwd"`
}
