import { describe, it, expect } from "vitest";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { atomicWriteJson, safeReadJson } from "./atomicFile";

function makeTempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "bridgely-test-"));
}

describe("atomicWriteJson", () => {
  it("writes valid JSON to the target file", async () => {
    const dir = makeTempDir();
    const file = path.join(dir, "state.json");
    const data = { sessionId: "VSCode-1", timestamp: 123456 };

    await atomicWriteJson(file, data);

    const raw = fs.readFileSync(file, "utf-8");
    expect(JSON.parse(raw)).toEqual(data);
  });

  it("writes pretty-printed JSON", async () => {
    const dir = makeTempDir();
    const file = path.join(dir, "state.json");
    await atomicWriteJson(file, { a: 1 });

    const raw = fs.readFileSync(file, "utf-8");
    expect(raw).toContain("\n");
  });

  it("leaves no tmp file after writing", async () => {
    const dir = makeTempDir();
    const file = path.join(dir, "state.json");
    await atomicWriteJson(file, { done: true });

    const files = fs.readdirSync(dir);
    expect(files.filter((f) => f.endsWith(".tmp"))).toHaveLength(0);
  });

  it("overwrites an existing file", async () => {
    const dir = makeTempDir();
    const file = path.join(dir, "state.json");

    await atomicWriteJson(file, { v: 1 });
    await atomicWriteJson(file, { v: 2 });

    const raw = fs.readFileSync(file, "utf-8");
    expect(JSON.parse(raw)).toEqual({ v: 2 });
  });

  it("handles nested objects and arrays", async () => {
    const dir = makeTempDir();
    const file = path.join(dir, "state.json");
    const data = {
      workspace: { folders: ["/a", "/b"], name: "myproject" },
      activeFile: null,
      terminals: [],
    };

    await atomicWriteJson(file, data);
    const raw = fs.readFileSync(file, "utf-8");
    expect(JSON.parse(raw)).toEqual(data);
  });
});

describe("safeReadJson", () => {
  it("returns parsed object for valid JSON", async () => {
    const dir = makeTempDir();
    const file = path.join(dir, "data.json");
    fs.writeFileSync(file, JSON.stringify({ key: "value" }));

    const result = await safeReadJson<{ key: string }>(file);
    expect(result).toEqual({ key: "value" });
  });

  it("returns null for a missing file", async () => {
    const result = await safeReadJson("/nonexistent/path/file.json");
    expect(result).toBeNull();
  });

  it("returns null for invalid JSON", async () => {
    const dir = makeTempDir();
    const file = path.join(dir, "bad.json");
    fs.writeFileSync(file, "not valid json {{");

    const result = await safeReadJson(file);
    expect(result).toBeNull();
  });

  it("returns null for an empty file", async () => {
    const dir = makeTempDir();
    const file = path.join(dir, "empty.json");
    fs.writeFileSync(file, "");

    const result = await safeReadJson(file);
    expect(result).toBeNull();
  });

  it("preserves type parameter on successful read", async () => {
    const dir = makeTempDir();
    const file = path.join(dir, "typed.json");
    const data = { timestamp: 999, sessionId: "abc" };
    fs.writeFileSync(file, JSON.stringify(data));

    const result = await safeReadJson<typeof data>(file);
    expect(result?.timestamp).toBe(999);
    expect(result?.sessionId).toBe("abc");
  });
});
