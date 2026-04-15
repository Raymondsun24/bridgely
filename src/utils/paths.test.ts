import { describe, it, expect } from "vitest";
import * as os from "os";
import * as path from "path";
import {
  getBridgeDir,
  getSessionsDir,
  getEditorStatePath,
  getCommandsPath,
  getCommandResultsPath,
  getSessionStatePath,
  getSessionCommandsPath,
  getSessionCommandResultsPath,
} from "./paths";

const home = os.homedir();
const bridgeDir = path.join(home, ".claude", "bridge");
const sessionsDir = path.join(bridgeDir, "sessions");

describe("paths", () => {
  it("getBridgeDir returns ~/.claude/bridge", () => {
    expect(getBridgeDir()).toBe(bridgeDir);
  });

  it("getSessionsDir returns ~/.claude/bridge/sessions", () => {
    expect(getSessionsDir()).toBe(sessionsDir);
  });

  it("getEditorStatePath returns legacy state file", () => {
    expect(getEditorStatePath()).toBe(path.join(bridgeDir, "editor-state.json"));
  });

  it("getCommandsPath returns legacy commands file", () => {
    expect(getCommandsPath()).toBe(path.join(bridgeDir, "commands.json"));
  });

  it("getCommandResultsPath returns legacy results file", () => {
    expect(getCommandResultsPath()).toBe(path.join(bridgeDir, "command-results.json"));
  });

  describe("getSessionStatePath", () => {
    it("returns sessions/<id>.json", () => {
      expect(getSessionStatePath("VSCode-123")).toBe(
        path.join(sessionsDir, "VSCode-123.json")
      );
    });

    it("handles session IDs with hyphens", () => {
      const result = getSessionStatePath("Cursor-99999");
      expect(result).toBe(path.join(sessionsDir, "Cursor-99999.json"));
    });
  });

  describe("getSessionCommandsPath", () => {
    it("returns sessions/<id>.commands.json", () => {
      expect(getSessionCommandsPath("VSCode-123")).toBe(
        path.join(sessionsDir, "VSCode-123.commands.json")
      );
    });
  });

  describe("getSessionCommandResultsPath", () => {
    it("returns sessions/<id>.commands-result.json", () => {
      expect(getSessionCommandResultsPath("VSCode-123")).toBe(
        path.join(sessionsDir, "VSCode-123.commands-result.json")
      );
    });
  });

  it("all session paths are inside sessionsDir", () => {
    const id = "VSCode-1";
    for (const p of [
      getSessionStatePath(id),
      getSessionCommandsPath(id),
      getSessionCommandResultsPath(id),
    ]) {
      expect(p.startsWith(sessionsDir)).toBe(true);
    }
  });
});
