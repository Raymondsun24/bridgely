import * as fs from "fs";
import * as vscode from "vscode";
import * as os from "os";
import * as path from "path";
import {
  BridgeCommand,
  CommandResult,
  GetTerminalOutputArgs,
  OpenFileArgs,
  PreviewEditArgs,
  RevealLineArgs,
  ShowDiffArgs,
} from "./protocol";
import { TerminalWatcher } from "./terminalWatcher";
import { safeReadJson, atomicWriteJson } from "./atomicFile";
import {
  getSessionCommandsPath,
  getSessionCommandResultsPath,
} from "../utils/paths";

export class CommandWatcher implements vscode.Disposable {
  private watcher: fs.FSWatcher | null = null;
  private changeTimer: ReturnType<typeof setTimeout> | null = null;
  private lastCommandId: string | null = null;
  private lastProposedTmpPath: string | null = null;
  private readonly sessionId: string;
  private readonly terminalWatcher: TerminalWatcher;

  constructor(sessionId: string, terminalWatcher: TerminalWatcher) {
    this.sessionId = sessionId;
    this.terminalWatcher = terminalWatcher;
  }

  start(): void {
    this.watchCommandsFile();
  }

  private watchCommandsFile(): void {
    const commandsPath = getSessionCommandsPath(this.sessionId);

    if (!fs.existsSync(commandsPath)) {
      fs.writeFileSync(commandsPath, "{}");
    }

    this.watcher?.close();
    this.watcher = fs.watch(commandsPath, (eventType) => {
      if (eventType === "change" || eventType === "rename") {
        // Debounce: shell `>` redirect truncates the file before writing,
        // which can trigger a "change" event on the empty file. Wait briefly
        // to let the write complete before reading.
        if (this.changeTimer) clearTimeout(this.changeTimer);
        this.changeTimer = setTimeout(() => {
          this.changeTimer = null;
          this.handleChange();
        }, 50);
        // On macOS, kqueue watches by inode. When the MCP server does an
        // atomic write (rename), the old inode is replaced, killing the
        // watch. Re-establish after rename events.
        if (eventType === "rename") {
          setTimeout(() => this.watchCommandsFile(), 100);
        }
      }
    });
    this.watcher.on("error", () => {
      this.watcher?.close();
      setTimeout(() => this.watchCommandsFile(), 500);
    });
  }

  private async handleChange(): Promise<void> {
    const cmd = await safeReadJson<BridgeCommand>(
      getSessionCommandsPath(this.sessionId)
    );
    if (!cmd || !cmd.id || !cmd.command || cmd.id === this.lastCommandId)
      return;

    this.lastCommandId = cmd.id;
    await this.dispatch(cmd);
  }

  private async dispatch(cmd: BridgeCommand): Promise<void> {
    let result: CommandResult;

    try {
      switch (cmd.command) {
        case "openFile":
          result = await this.handleOpenFile(cmd.id, cmd.args as OpenFileArgs);
          break;
        case "revealLine":
          result = await this.handleRevealLine(
            cmd.id,
            cmd.args as RevealLineArgs
          );
          break;
        case "getSelection":
          result = await this.handleGetSelection(cmd.id);
          break;
        case "getDiagnostics":
          result = await this.handleGetDiagnostics(cmd.id, cmd.args);
          break;
        case "showDiff":
          result = await this.handleShowDiff(cmd.id, cmd.args as ShowDiffArgs);
          break;
        case "previewEdit":
          result = await this.handlePreviewEdit(
            cmd.id,
            cmd.args as PreviewEditArgs
          );
          break;
        case "closePreview":
          result = await this.handleClosePreview(cmd.id);
          break;
        case "getTerminalOutput":
          result = this.handleGetTerminalOutput(
            cmd.id,
            cmd.args as GetTerminalOutputArgs
          );
          break;
        default:
          result = this.makeResult(cmd.id, "error", {
            message: `Unknown command: ${cmd.command}`,
          });
      }
    } catch (err) {
      result = this.makeResult(cmd.id, "error", {
        message: `Command failed: ${err}`,
      });
    }

    await atomicWriteJson(getSessionCommandResultsPath(this.sessionId), result);
  }

  private async handleOpenFile(
    id: string,
    args: OpenFileArgs
  ): Promise<CommandResult> {
    const uri = vscode.Uri.file(args.path);
    const doc = await vscode.workspace.openTextDocument(uri);
    const editor = await vscode.window.showTextDocument(doc, {
      preview: args.preview ?? true,
    });

    if (args.line) {
      const line = Math.max(0, args.line - 1);
      const col = Math.max(0, (args.column ?? 1) - 1);
      const pos = new vscode.Position(line, col);
      editor.selection = new vscode.Selection(pos, pos);
      editor.revealRange(
        new vscode.Range(pos, pos),
        vscode.TextEditorRevealType.InCenter
      );
    }

    const rel = vscode.workspace.asRelativePath(uri, false);
    return this.makeResult(id, "ok", {
      message: `Opened ${rel}${args.line ? ` at line ${args.line}` : ""}`,
    });
  }

  private async handleRevealLine(
    id: string,
    args: RevealLineArgs
  ): Promise<CommandResult> {
    const uri = vscode.Uri.file(args.path);
    const doc = await vscode.workspace.openTextDocument(uri);
    const editor = await vscode.window.showTextDocument(doc, {
      preserveFocus: true,
      preview: true,
    });

    const line = Math.max(0, args.line - 1);
    const pos = new vscode.Position(line, 0);
    editor.revealRange(
      new vscode.Range(pos, pos),
      vscode.TextEditorRevealType.InCenter
    );

    return this.makeResult(id, "ok", {
      message: `Revealed line ${args.line}`,
    });
  }

  private async handleGetSelection(id: string): Promise<CommandResult> {
    const editor = vscode.window.activeTextEditor;
    if (!editor || editor.selection.isEmpty) {
      return this.makeResult(id, "ok", {
        message: "No selection",
        data: { text: "", filePath: "" },
      });
    }

    const text = editor.document.getText(editor.selection);
    return this.makeResult(id, "ok", {
      message: "Selection retrieved",
      data: {
        text,
        filePath: editor.document.uri.fsPath,
        startLine: editor.selection.start.line + 1,
        endLine: editor.selection.end.line + 1,
      },
    });
  }

  private async handleGetDiagnostics(
    id: string,
    args: unknown
  ): Promise<CommandResult> {
    const diagArgs = args as { path?: string };
    let diagnostics: [vscode.Uri, vscode.Diagnostic[]][];

    if (diagArgs.path) {
      const uri = vscode.Uri.file(diagArgs.path);
      const fileDiags = vscode.languages.getDiagnostics(uri);
      diagnostics = [[uri, fileDiags]];
    } else {
      diagnostics = vscode.languages.getDiagnostics() as [
        vscode.Uri,
        vscode.Diagnostic[],
      ][];
    }

    const formatted = diagnostics
      .filter(([, diags]) => diags.length > 0)
      .map(([uri, diags]) => ({
        path: uri.fsPath,
        diagnostics: diags.map((d) => ({
          line: d.range.start.line + 1,
          severity: vscode.DiagnosticSeverity[d.severity],
          message: d.message,
          source: d.source,
        })),
      }));

    return this.makeResult(id, "ok", {
      message: `${formatted.length} file(s) with diagnostics`,
      data: formatted,
    });
  }

  private async handleShowDiff(
    id: string,
    args: ShowDiffArgs
  ): Promise<CommandResult> {
    const uri = vscode.Uri.file(args.path);

    // Refresh git state and reload the document from disk so the diff
    // reflects the external edit Claude just made.
    try {
      await vscode.commands.executeCommand("git.refresh");
    } catch { /* git extension may not be available */ }
    const doc = await vscode.workspace.openTextDocument(uri);
    await vscode.window.showTextDocument(doc, { preview: false });
    await vscode.commands.executeCommand("workbench.action.files.revert");

    // Wait for git to finish processing the refresh
    await new Promise((resolve) => setTimeout(resolve, 500));

    // Try to open the SCM diff view (works for git-tracked files)
    try {
      await vscode.commands.executeCommand("git.openChange", uri);
    } catch {
      // Git extension unavailable or file not tracked — file is already open
    }

    const rel = vscode.workspace.asRelativePath(uri, false);
    return this.makeResult(id, "ok", {
      message: `Showing diff for ${rel}`,
    });
  }

  private async handlePreviewEdit(
    id: string,
    args: PreviewEditArgs
  ): Promise<CommandResult> {
    const filePath = args.file_path;
    const currentUri = vscode.Uri.file(filePath);
    const rel = vscode.workspace.asRelativePath(currentUri, false);

    // Read the current file content
    let currentContent: string;
    try {
      currentContent = fs.readFileSync(filePath, "utf-8");
    } catch {
      return this.makeResult(id, "error", {
        message: `Cannot read file: ${rel}`,
      });
    }

    // Compute the proposed content
    let proposedContent: string;
    if (args.tool_name === "Write") {
      proposedContent = args.content ?? "";
    } else {
      // Edit tool: replace old_string with new_string
      const oldStr = args.old_string ?? "";
      const newStr = args.new_string ?? "";
      const idx = currentContent.indexOf(oldStr);
      if (idx === -1) {
        return this.makeResult(id, "error", {
          message: `old_string not found in ${rel}`,
        });
      }
      proposedContent =
        currentContent.substring(0, idx) +
        newStr +
        currentContent.substring(idx + oldStr.length);
    }

    // Write proposed content to a temp file
    const tmpDir = os.tmpdir();
    const ext = path.extname(filePath);
    const baseName = path.basename(filePath, ext);
    const tmpPath = path.join(tmpDir, `${baseName}.proposed${ext}`);
    fs.writeFileSync(tmpPath, proposedContent);
    const proposedUri = vscode.Uri.file(tmpPath);

    // Open side-by-side diff: current (left) vs proposed (right)
    await vscode.commands.executeCommand(
      "vscode.diff",
      currentUri,
      proposedUri,
      `${rel} ← proposed edit`
    );

    this.lastProposedTmpPath = tmpPath;

    return this.makeResult(id, "ok", {
      message: `Previewing edit for ${rel}`,
    });
  }

  private async handleClosePreview(id: string): Promise<CommandResult> {
    if (!this.lastProposedTmpPath) {
      return this.makeResult(id, "ok", {
        message: "No preview to close",
      });
    }

    const tmpUri = vscode.Uri.file(this.lastProposedTmpPath);

    // Find and close tabs showing the proposed temp file
    for (const group of vscode.window.tabGroups.all) {
      for (const tab of group.tabs) {
        const input = tab.input;
        // Diff tabs have a TabInputTextDiff input type
        if (
          input instanceof vscode.TabInputTextDiff &&
          (input.modified.fsPath === tmpUri.fsPath ||
            input.original.fsPath === tmpUri.fsPath)
        ) {
          await vscode.window.tabGroups.close(tab);
        } else if (
          input instanceof vscode.TabInputText &&
          input.uri.fsPath === tmpUri.fsPath
        ) {
          await vscode.window.tabGroups.close(tab);
        }
      }
    }

    // Clean up the temp file
    try {
      fs.unlinkSync(this.lastProposedTmpPath);
    } catch {
      // Already gone
    }

    this.lastProposedTmpPath = null;

    return this.makeResult(id, "ok", {
      message: "Preview closed",
    });
  }

  private handleGetTerminalOutput(
    id: string,
    args: GetTerminalOutputArgs
  ): CommandResult {
    const terminals = this.terminalWatcher.getTerminals();
    const executions = this.terminalWatcher.getRecentExecutions(
      args.terminalName,
      args.limit ?? 10
    );

    return this.makeResult(id, "ok", {
      message: `${terminals.length} terminal(s), ${executions.length} recent execution(s)`,
      data: { terminals, executions },
    });
  }

  private makeResult(
    id: string,
    status: "ok" | "error",
    result: { message: string; data?: unknown }
  ): CommandResult {
    return {
      version: 1,
      id,
      timestamp: Date.now(),
      status,
      result,
    };
  }

  dispose(): void {
    this.watcher?.close();
    // Clean up session command files
    try {
      fs.unlinkSync(getSessionCommandsPath(this.sessionId));
    } catch { /* already gone */ }
    try {
      fs.unlinkSync(getSessionCommandResultsPath(this.sessionId));
    } catch { /* already gone */ }
  }
}
