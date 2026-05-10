import { spawnSync } from "child_process";
import { resolve } from "path";

export const PROJECT_ROOT = resolve(__dirname, "..");

// `just build` emits the native binary as `tsgonest-native` on Unix and
// `tsgonest.exe` on Windows (see justfile [unix]/[windows] build recipes).
// The launcher script at `packages/core/bin/tsgonest` is for end users — it
// can't be spawned directly on Windows because it has no extension.
const NATIVE_BIN_NAME =
  process.platform === "win32" ? "tsgonest.exe" : "tsgonest-native";
export const TSGONEST_BIN = resolve(
  PROJECT_ROOT,
  "packages",
  "core",
  "bin",
  NATIVE_BIN_NAME,
);
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
