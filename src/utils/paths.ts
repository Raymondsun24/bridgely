import * as fs from "fs";
import * as path from "path";
import * as os from "os";

const BRIDGE_DIR = path.join(os.homedir(), ".claude", "bridge");
const SESSIONS_DIR = path.join(BRIDGE_DIR, "sessions");
const PROPOSED_DIR = path.join(BRIDGE_DIR, "proposed");

export function getBridgeDir(): string {
  return BRIDGE_DIR;
}

export function getSessionsDir(): string {
  return SESSIONS_DIR;
}

export function getProposedDir(): string {
  return PROPOSED_DIR;
}

// Legacy single-file paths (for backward compat readers)
export function getEditorStatePath(): string {
  return path.join(BRIDGE_DIR, "editor-state.json");
}

export function getCommandsPath(): string {
  return path.join(BRIDGE_DIR, "commands.json");
}

export function getCommandResultsPath(): string {
  return path.join(BRIDGE_DIR, "command-results.json");
}

// Per-session paths
export function getSessionStatePath(sessionId: string): string {
  return path.join(SESSIONS_DIR, `${sessionId}.json`);
}

export function getSessionCommandsPath(sessionId: string): string {
  return path.join(SESSIONS_DIR, `${sessionId}.commands.json`);
}

export function getSessionCommandResultsPath(sessionId: string): string {
  return path.join(SESSIONS_DIR, `${sessionId}.commands-result.json`);
}

export async function ensureBridgeDir(): Promise<void> {
  await fs.promises.mkdir(SESSIONS_DIR, { recursive: true, mode: 0o700 });
  await fs.promises.mkdir(PROPOSED_DIR, { recursive: true, mode: 0o700 });
}
