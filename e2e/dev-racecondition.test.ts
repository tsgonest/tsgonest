import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { spawn, ChildProcess } from "child_process";
import { existsSync, writeFileSync, mkdirSync, rmSync, copyFileSync } from "fs";
import { resolve, join } from "path";
import { TSGONEST_BIN, FIXTURES_DIR } from "./helpers";
import { tmpdir } from "os";

const FIXTURE_SRC = resolve(FIXTURES_DIR, "dev-racecondition");

/**
 * Waits for a specific pattern to appear in the accumulated output.
 * Returns the full output collected so far.
 */
function waitForOutput(
  proc: ChildProcess,
  pattern: string | RegExp,
  timeoutMs: number = 30_000
): Promise<string> {
  return new Promise((resolve, reject) => {
    let output = "";
    const timer = setTimeout(() => {
      cleanup();
      reject(
        new Error(
          `Timed out waiting for "${pattern}" after ${timeoutMs}ms.\nOutput so far:\n${output}`
        )
      );
    }, timeoutMs);

    const onData = (chunk: Buffer) => {
      output += chunk.toString();
      const matches =
        typeof pattern === "string"
          ? output.includes(pattern)
          : pattern.test(output);
      if (matches) {
        cleanup();
        resolve(output);
      }
    };

    const onExit = (code: number | null) => {
      cleanup();
      reject(
        new Error(
          `Process exited with code ${code} before pattern "${pattern}" matched.\nOutput:\n${output}`
        )
      );
    };

    const cleanup = () => {
      clearTimeout(timer);
      proc.stdout?.off("data", onData);
      proc.stderr?.off("data", onData);
      proc.off("exit", onExit);
    };

    proc.stdout?.on("data", onData);
    proc.stderr?.on("data", onData);
    proc.on("exit", onExit);
  });
}

/**
 * Collects all output from a process for a given duration.
 */
function collectOutput(proc: ChildProcess, durationMs: number): Promise<string> {
  return new Promise((resolve) => {
    let output = "";
    const onData = (chunk: Buffer) => {
      output += chunk.toString();
    };
    proc.stdout?.on("data", onData);
    proc.stderr?.on("data", onData);
    setTimeout(() => {
      proc.stdout?.off("data", onData);
      proc.stderr?.off("data", onData);
      resolve(output);
    }, durationMs);
  });
}

/**
 * Kill the spawned process and resolve only after it has fully exited.
 *
 * Awaiting the `exit` event matters on Windows: the OS holds file handles
 * (fsnotify ReadDirectoryChangesW handles on the watched src/ directory,
 * the Job Object that contains the child node process, etc.) until the
 * process is fully reaped. If the test calls `rmSync(tmpDir, ...)` before
 * the spawned tsgonest has actually exited, Windows refuses the unlink
 * with EPERM. POSIX permits unlink of an open file and defers cleanup,
 * which is why this race only bites on Windows.
 *
 * Sends SIGTERM (which Node maps to TerminateProcess on Windows) and
 * waits up to ~5s; if the process is still alive after that, escalates
 * to SIGKILL so the test cannot hang on a wedged child.
 */
function killProc(proc: ChildProcess): Promise<void> {
  return new Promise((resolveOuter) => {
    if (proc.exitCode !== null || proc.signalCode !== null) {
      resolveOuter();
      return;
    }

    const cleanup = () => {
      proc.off("exit", onExit);
      clearTimeout(forceKillTimer);
    };
    const onExit = () => {
      cleanup();
      resolveOuter();
    };
    proc.on("exit", onExit);

    // Escalate to SIGKILL if the process refuses to exit. 5s mirrors the
    // graceful-shutdown window the runner itself uses internally.
    const forceKillTimer = setTimeout(() => {
      try {
        if (proc.pid) {
          process.kill(-proc.pid, "SIGKILL");
        } else {
          proc.kill("SIGKILL");
        }
      } catch {
        try {
          proc.kill("SIGKILL");
        } catch {
          // already dead — onExit will fire (or already has)
        }
      }
    }, 5_000);

    try {
      // Kill the process group to clean up child processes (Unix). On
      // Windows, Node maps signals to TerminateProcess on the target pid;
      // the negative-pid trick is not meaningful but the catch below
      // falls back to a single-process kill.
      if (proc.pid) {
        process.kill(-proc.pid, "SIGTERM");
      } else {
        proc.kill("SIGTERM");
      }
    } catch {
      try {
        proc.kill("SIGTERM");
      } catch {
        // already dead — onExit will fire (or already has)
        cleanup();
        resolveOuter();
      }
    }
  });
}

/**
 * Copies the fixture to a temp directory so we can modify files freely.
 */
function copyFixture(): string {
  const tmp = join(
    tmpdir(),
    `tsgonest-dev-race-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  );
  mkdirSync(join(tmp, "src"), { recursive: true });

  copyFileSync(
    join(FIXTURE_SRC, "tsconfig.json"),
    join(tmp, "tsconfig.json")
  );
  for (const file of ["main.ts", "service.ts", "helper.ts"]) {
    copyFileSync(join(FIXTURE_SRC, "src", file), join(tmp, "src", file));
  }
  return tmp;
}

describe("tsgonest dev race condition", () => {
  let tmpDir: string;
  let devProc: ChildProcess | null = null;

  beforeAll(() => {
    tmpDir = copyFixture();
  });

  afterAll(async () => {
    // Await process exit before deleting the temp dir. On Windows the OS
    // refuses to unlink files inside a directory while any process still
    // holds open handles to them — and tsgonest holds fsnotify and Job
    // Object handles right up to the moment its main goroutine returns.
    if (devProc) {
      await killProc(devProc);
      devProc = null;
    }
    if (tmpDir && existsSync(tmpDir)) {
      rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it("should not produce MODULE_NOT_FOUND when rapidly modifying multiple files", async () => {
    // Start tsgonest dev
    devProc = spawn(TSGONEST_BIN, ["dev", "--project", "tsconfig.json"], {
      cwd: tmpDir,
      stdio: ["ignore", "pipe", "pipe"],
      detached: true, // create process group for clean kill
    });

    // Wait for initial build + process start
    await waitForOutput(devProc, "watching for changes");

    // Give the child node process a moment to fully start
    await new Promise((r) => setTimeout(r, 500));

    // Now rapidly modify ALL source files — simulates IDE refactor / save-all.
    // This is the scenario that triggers the race condition with a short debounce.
    const files = ["helper.ts", "service.ts", "main.ts"];
    const modifications: Record<string, string> = {
      "helper.ts": `export function formatName(first: string, last: string): string {
  return \`\${first} \${last}\`;
}

export function add(a: number, b: number): number {
  return a + b;
}

export function multiply(a: number, b: number): number {
  return a * b;
}
`,
      "service.ts": `import { formatName, add, multiply } from "./helper.js";

export class UserService {
  greet(first: string, last: string): string {
    return \`Hello, \${formatName(first, last)}!\`;
  }

  sum(a: number, b: number): number {
    return add(a, b);
  }

  product(a: number, b: number): number {
    return multiply(a, b);
  }
}
`,
      "main.ts": `import { UserService } from "./service.js";

const svc = new UserService();
console.log(svc.greet("Test", "User"));
console.log("sum:", svc.sum(1, 2));
console.log("product:", svc.product(3, 4));
console.log("DEV_READY");
`,
    };

    // Write all files as fast as possible (no delays between writes)
    for (const file of files) {
      writeFileSync(join(tmpDir, "src", file), modifications[file]);
    }

    // Wait for rebuild + restart to complete
    const output = await waitForOutput(devProc, "restarting...", 30_000);

    // Collect output for a bit more to catch any errors from the restarted process
    const postRestartOutput = await collectOutput(devProc, 3_000);
    const fullOutput = output + postRestartOutput;

    // The key assertion: no MODULE_NOT_FOUND errors
    expect(fullOutput).not.toContain("MODULE_NOT_FOUND");
    expect(fullOutput).not.toContain("Cannot find module");
    expect(fullOutput).not.toContain("ENOENT");

    // Should see exactly one "restarting..." (not multiple rapid restarts)
    const restartCount = (fullOutput.match(/restarting/g) || []).length;
    expect(restartCount).toBe(1);

    await killProc(devProc);
    devProc = null;
  }, 60_000);

  it("should coalesce staggered file writes into a single rebuild", async () => {
    // Re-copy the fixture for a fresh state. The previous test already
    // awaited its devProc to fully exit before reaching this point, so
    // the rmSync is safe on Windows (no open handles inside tmpDir).
    if (existsSync(tmpDir)) {
      rmSync(tmpDir, { recursive: true, force: true });
    }
    tmpDir = copyFixture();

    devProc = spawn(TSGONEST_BIN, ["dev", "--project", "tsconfig.json"], {
      cwd: tmpDir,
      stdio: ["ignore", "pipe", "pipe"],
      detached: true,
    });

    await waitForOutput(devProc, "watching for changes");
    await new Promise((r) => setTimeout(r, 500));

    // Write files with small gaps (50ms apart) — within the 300ms debounce window.
    // All three should be coalesced into a single rebuild.
    const staggeredWrites = [
      {
        file: "helper.ts",
        content: `export function formatName(first: string, last: string): string {
  return \`\${last}, \${first}\`;
}

export function add(a: number, b: number): number {
  return a + b;
}
`,
      },
      {
        file: "service.ts",
        content: `import { formatName, add } from "./helper.js";

export class UserService {
  greet(first: string, last: string): string {
    return \`Hi, \${formatName(first, last)}!\`;
  }

  sum(a: number, b: number): number {
    return add(a, b);
  }
}
`,
      },
      {
        file: "main.ts",
        content: `import { UserService } from "./service.js";

const svc = new UserService();
console.log(svc.greet("Stagger", "Test"));
console.log("sum:", svc.sum(10, 20));
console.log("DEV_READY");
`,
      },
    ];

    for (const { file, content } of staggeredWrites) {
      writeFileSync(join(tmpDir, "src", file), content);
      await new Promise((r) => setTimeout(r, 50));
    }

    // Wait for rebuild
    const output = await waitForOutput(devProc, "restarting...", 30_000);
    const postRestartOutput = await collectOutput(devProc, 3_000);
    const fullOutput = output + postRestartOutput;

    // No errors
    expect(fullOutput).not.toContain("MODULE_NOT_FOUND");
    expect(fullOutput).not.toContain("Cannot find module");

    // The 3 changes should be coalesced into a single "detected N change(s)" line
    // with N >= 2 (all batched by the 300ms debounce), not 3 separate rebuilds.
    const rebuildMatches = fullOutput.match(/detected (\d+) change/g) || [];
    expect(rebuildMatches.length).toBe(1);

    await killProc(devProc);
    devProc = null;
  }, 60_000);
});
