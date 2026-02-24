import { spawnSync } from "child_process";
import { resolve } from "path";

export const PROJECT_ROOT = resolve(__dirname, "..");
export const TSGONEST_BIN = resolve(PROJECT_ROOT, "tsgonest");
export const FIXTURES_DIR = resolve(PROJECT_ROOT, "testdata");

export function runTsgonest(args: string[], opts?: { cwd?: string }) {
  const result = spawnSync(TSGONEST_BIN, args, {
    encoding: "utf-8",
    cwd: opts?.cwd ?? PROJECT_ROOT,
    timeout: 30000,
  });
  return {
    stdout: result.stdout || "",
    stderr: result.stderr || "",
    exitCode: result.status ?? 1,
  };
}
