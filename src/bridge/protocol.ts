export interface ActiveFile {
  path: string;
  relativePath: string;
  languageId: string;
  lineCount: number;
  cursorLine: number;
  cursorColumn: number;
}

export interface Selection {
  text: string;
  startLine: number;
  startColumn: number;
  endLine: number;
  endColumn: number;
  filePath: string;
}

export interface VisibleFile {
  path: string;
  isActive: boolean;
  isDirty: boolean;
  viewColumn: number;
}

export interface TerminalInfo {
  name: string;
  isActive: boolean;
  hasShellIntegration: boolean;
}

export interface TerminalExecution {
  command: string;
  output: string;
  exitCode: number | undefined;
  cwd: string | undefined;
  timestamp: number;
  terminalName: string;
}

export interface EditorState {
  version: 1;
  timestamp: number;
  pid: number;
  sessionId: string;
  ideName: string;
  workspace: {
    folders: string[];
    name: string;
  };
  activeFile: ActiveFile | null;
  selection: Selection | null;
  visibleFiles: VisibleFile[];
  terminals: TerminalInfo[];
}

export type CommandType = "openFile" | "revealLine" | "getSelection" | "getDiagnostics" | "showDiff" | "previewEdit" | "closePreview" | "getTerminalOutput";

export interface OpenFileArgs {
  path: string;
  line?: number;
  column?: number;
  preview?: boolean;
}

export interface RevealLineArgs {
  path: string;
  line: number;
}

export interface GetSelectionArgs {}

export interface GetDiagnosticsArgs {
  path?: string;
}

export interface ShowDiffArgs {
  path: string;
}

export interface PreviewEditArgs {
  file_path: string;
  tool_name: string;
  old_string?: string;
  new_string?: string;
  content?: string;
}

export interface GetTerminalOutputArgs {
  terminalName?: string;
  limit?: number;
}

export type CommandArgs = OpenFileArgs | RevealLineArgs | GetSelectionArgs | GetDiagnosticsArgs | ShowDiffArgs | PreviewEditArgs | GetTerminalOutputArgs;

export interface BridgeCommand {
  version: 1;
  id: string;
  timestamp: number;
  command: CommandType;
  args: CommandArgs;
}

export interface CommandResult {
  version: 1;
  id: string;
  timestamp: number;
  status: "ok" | "error";
  result: {
    message: string;
    data?: unknown;
  };
}
