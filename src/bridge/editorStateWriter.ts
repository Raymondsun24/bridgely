import * as vscode from "vscode";
import * as fs from "fs";
import { EditorState, ActiveFile, Selection, VisibleFile } from "./protocol";
import { TerminalWatcher } from "./terminalWatcher";
import { atomicWriteJson } from "./atomicFile";
import { getSessionStatePath } from "../utils/paths";
import { debounce } from "../utils/debounce";

export class EditorStateWriter implements vscode.Disposable {
  private disposables: vscode.Disposable[] = [];
  private debouncedWrite: () => void;
  private selectionDebouncedWrite: () => void;
  private readonly sessionId: string;
  private readonly terminalWatcher: TerminalWatcher;

  constructor(sessionId: string, terminalWatcher: TerminalWatcher) {
    this.sessionId = sessionId;
    this.terminalWatcher = terminalWatcher;

    const config = vscode.workspace.getConfiguration("bridgely");
    const debounceMs = config.get<number>("debounceMs", 300);
    const selectionDebounceMs = config.get<number>("selectionDebounceMs", 150);

    this.debouncedWrite = debounce(() => this.writeState(), debounceMs);
    this.selectionDebouncedWrite = debounce(
      () => this.writeState(),
      selectionDebounceMs
    );

    this.disposables.push(
      vscode.window.onDidChangeActiveTextEditor(() => this.debouncedWrite()),
      vscode.window.onDidChangeTextEditorSelection(() =>
        this.selectionDebouncedWrite()
      ),
      vscode.workspace.onDidSaveTextDocument(() => this.debouncedWrite()),
      vscode.window.onDidChangeVisibleTextEditors(() => this.debouncedWrite()),
      vscode.workspace.onDidChangeWorkspaceFolders(() => this.debouncedWrite()),
      vscode.window.onDidOpenTerminal(() => this.debouncedWrite()),
      vscode.window.onDidCloseTerminal(() => this.debouncedWrite()),
      vscode.window.onDidChangeActiveTerminal(() => this.debouncedWrite()),
      vscode.window.onDidEndTerminalShellExecution(() => this.debouncedWrite())
    );

    // Write initial state
    this.writeState();
  }

  private getIdeName(): string {
    if (vscode.env.appName.toLowerCase().includes("cursor")) {
      return "Cursor";
    }
    return "VS Code";
  }

  private getActiveFile(): ActiveFile | null {
    const editor = vscode.window.activeTextEditor;
    if (!editor) return null;

    const doc = editor.document;
    const pos = editor.selection.active;
    const workspaceFolder = vscode.workspace.getWorkspaceFolder(doc.uri);
    const relativePath = workspaceFolder
      ? vscode.workspace.asRelativePath(doc.uri, false)
      : doc.uri.fsPath;

    return {
      path: doc.uri.fsPath,
      relativePath,
      languageId: doc.languageId,
      lineCount: doc.lineCount,
      cursorLine: pos.line + 1,
      cursorColumn: pos.character + 1,
    };
  }

  private getSelection(): Selection | null {
    const editor = vscode.window.activeTextEditor;
    if (!editor || editor.selection.isEmpty) return null;

    const config = vscode.workspace.getConfiguration("bridgely");
    const maxLen = config.get<number>("maxSelectionLength", 10000);
    const sel = editor.selection;
    let text = editor.document.getText(sel);

    if (text.length > maxLen) {
      text = text.substring(0, maxLen) + `\n... (truncated at ${maxLen} chars)`;
    }

    return {
      text,
      startLine: sel.start.line + 1,
      startColumn: sel.start.character + 1,
      endLine: sel.end.line + 1,
      endColumn: sel.end.character + 1,
      filePath: editor.document.uri.fsPath,
    };
  }

  private getVisibleFiles(): VisibleFile[] {
    return vscode.window.visibleTextEditors.map((editor) => ({
      path: editor.document.uri.fsPath,
      isActive: editor === vscode.window.activeTextEditor,
      isDirty: editor.document.isDirty,
      viewColumn: editor.viewColumn ?? 1,
    }));
  }

  private async writeState(): Promise<void> {
    const folders = (vscode.workspace.workspaceFolders ?? []).map(
      (f) => f.uri.fsPath
    );
    const workspaceName = vscode.workspace.name ?? "";

    const state: EditorState = {
      version: 1,
      timestamp: Date.now(),
      pid: process.pid,
      sessionId: this.sessionId,
      ideName: this.getIdeName(),
      workspace: {
        folders,
        name: workspaceName,
      },
      activeFile: this.getActiveFile(),
      selection: this.getSelection(),
      visibleFiles: this.getVisibleFiles(),
      terminals: this.terminalWatcher.getTerminals(),
    };

    try {
      await atomicWriteJson(getSessionStatePath(this.sessionId), state);
    } catch (err) {
      console.error("[bridgely] Failed to write editor state:", err);
    }
  }

  dispose(): void {
    this.disposables.forEach((d) => d.dispose());
    // Clean up session file
    try {
      fs.unlinkSync(getSessionStatePath(this.sessionId));
    } catch {
      // Already gone — fine
    }
  }
}
