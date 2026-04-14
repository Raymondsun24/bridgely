import * as vscode from "vscode";
import { TerminalInfo, TerminalExecution } from "./protocol";

/** Strip ANSI escape sequences and VS Code shell integration OSC markers. */
function stripEscapes(raw: string): string {
  return (
    raw
      // OSC sequences: \x1b] ... \x07  or  \x1b] ... \x1b\\
      .replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, "")
      // CSI sequences: \x1b[ ... <final byte>
      .replace(/\x1b\[[0-9;?]*[A-Za-z]/g, "")
      // Remaining bare escapes
      .replace(/\x1b[^[\]]/g, "")
      // Carriage returns (overwrite lines)
      .replace(/\r(?!\n)/g, "")
  );
}

interface PendingExecution {
  terminal: vscode.Terminal;
  execution: vscode.TerminalShellExecution;
  output: string;
  truncated: boolean;
}

export class TerminalWatcher implements vscode.Disposable {
  private disposables: vscode.Disposable[] = [];
  private executions: TerminalExecution[] = [];
  private pending: Map<vscode.TerminalShellExecution, PendingExecution> =
    new Map();
  private readonly maxExecutions: number;
  private readonly maxOutputLength: number;

  constructor(maxExecutions = 50, maxOutputLength = 20000) {
    this.maxExecutions = maxExecutions;
    this.maxOutputLength = maxOutputLength;

    this.disposables.push(
      vscode.window.onDidStartTerminalShellExecution((e) => {
        this.startCapture(e);
      }),
      vscode.window.onDidEndTerminalShellExecution((e) => {
        this.finalizeCapture(e);
      })
    );
  }

  private startCapture(e: vscode.TerminalShellExecutionStartEvent): void {
    const pending: PendingExecution = {
      terminal: e.terminal,
      execution: e.execution,
      output: "",
      truncated: false,
    };

    this.pending.set(e.execution, pending);

    // Read the stream in the background
    (async () => {
      try {
        for await (const chunk of e.execution.read()) {
          if (pending.truncated) break;
          pending.output += chunk;
          if (pending.output.length > this.maxOutputLength) {
            pending.output =
              pending.output.substring(0, this.maxOutputLength) +
              "\n... (truncated)";
            pending.truncated = true;
          }
        }
      } catch {
        // Stream closed or terminal disposed
      }
    })();
  }

  private finalizeCapture(e: vscode.TerminalShellExecutionEndEvent): void {
    const pending = this.pending.get(e.execution);
    this.pending.delete(e.execution);

    const commandLine =
      e.execution.commandLine?.value ?? pending?.execution.commandLine?.value;

    const cleanOutput = stripEscapes(pending?.output ?? "").trim();

    this.executions.push({
      command: commandLine ?? "(unknown)",
      output: cleanOutput,
      exitCode: e.exitCode,
      cwd: e.execution.cwd?.fsPath,
      timestamp: Date.now(),
      terminalName: (pending?.terminal ?? e.terminal).name,
    });

    // Trim ring buffer
    if (this.executions.length > this.maxExecutions) {
      this.executions = this.executions.slice(-this.maxExecutions);
    }
  }

  getTerminals(): TerminalInfo[] {
    const activeTerminal = vscode.window.activeTerminal;
    return vscode.window.terminals.map((t) => ({
      name: t.name,
      isActive: t === activeTerminal,
      hasShellIntegration: !!t.shellIntegration,
    }));
  }

  getRecentExecutions(terminalName?: string, limit = 10): TerminalExecution[] {
    let execs = this.executions;
    if (terminalName) {
      execs = execs.filter((e) => e.terminalName === terminalName);
    }
    return execs.slice(-limit);
  }

  dispose(): void {
    this.disposables.forEach((d) => d.dispose());
  }
}
