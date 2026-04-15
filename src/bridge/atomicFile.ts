import * as fs from "fs";
import * as path from "path";

export async function atomicWriteJson(
  filePath: string,
  data: unknown
): Promise<void> {
  const dir = path.dirname(filePath);
  const tmpPath = path.join(
    dir,
    `.${path.basename(filePath)}.${process.pid}.tmp`
  );
  const json = JSON.stringify(data, null, 2);
  await fs.promises.writeFile(tmpPath, json, { mode: 0o600, encoding: "utf-8" });
  await fs.promises.rename(tmpPath, filePath);
}

export async function safeReadJson<T>(filePath: string): Promise<T | null> {
  try {
    const raw = await fs.promises.readFile(filePath, "utf-8");
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}
