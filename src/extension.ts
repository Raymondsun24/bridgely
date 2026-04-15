import * as vscode from "vscode";
import { ensureBridgeDir } from "./utils/paths";
import { EditorStateWriter } from "./bridge/editorStateWriter";
import { CommandWatcher } from "./bridge/commandWatcher";
import { TerminalWatcher } from "./bridge/terminalWatcher";

let stateWriter: EditorStateWriter | undefined;
let commandWatcher: CommandWatcher | undefined;
let terminalWatcher: TerminalWatcher | undefined;

function getIdeName(): string {
  if (vscode.env.appName.toLowerCase().includes("cursor")) {
    return "Cursor";
  }
  return "VSCode";
}

export async function activate(
  context: vscode.ExtensionContext
): Promise<void> {
  const config = vscode.workspace.getConfiguration("bridgely");
  if (!config.get<boolean>("enabled", true)) {
    return;
  }

  await ensureBridgeDir();

  const sessionId = `${getIdeName()}-${process.pid}`;

  const maxTerminalExecutions = config.get<number>("maxTerminalExecutions", 50);
  const maxTerminalOutputLength = config.get<number>("maxTerminalOutputLength", 20000);

  terminalWatcher = new TerminalWatcher(maxTerminalExecutions, maxTerminalOutputLength);
  context.subscriptions.push(terminalWatcher);

  stateWriter = new EditorStateWriter(sessionId, terminalWatcher);
  context.subscriptions.push(stateWriter);

  commandWatcher = new CommandWatcher(sessionId, terminalWatcher);
  commandWatcher.start();
  context.subscriptions.push(commandWatcher);

  // Manual commands
  context.subscriptions.push(
    vscode.commands.registerCommand("bridgely.sendSelection", () => {
      // Force an immediate state write by triggering the writer
      // The selection is already captured in the regular state updates,
      // but this command lets users explicitly push selection to the bridge
      const editor = vscode.window.activeTextEditor;
      if (editor && !editor.selection.isEmpty) {
        vscode.window.showInformationMessage(
          `Bridgely: Selection sent (${editor.selection.end.line - editor.selection.start.line + 1} lines)`
        );
      } else {
        vscode.window.showInformationMessage(
          "Bridgely: No text selected"
        );
      }
    }),

    vscode.commands.registerCommand("bridgely.sendFile", () => {
      const editor = vscode.window.activeTextEditor;
      if (editor) {
        const rel = vscode.workspace.asRelativePath(
          editor.document.uri,
          false
        );
        vscode.window.showInformationMessage(
          `Bridgely: File context sent (${rel})`
        );
      } else {
        vscode.window.showInformationMessage("Bridgely: No active file");
      }
    })
  );

  console.log(`[bridgely] Extension activated (session: ${sessionId})`);
}

export function deactivate(): void {
  terminalWatcher?.dispose();
  stateWriter?.dispose();
  commandWatcher?.dispose();
  console.log("[bridgely] Extension deactivated");
}
