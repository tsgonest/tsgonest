/**
 * Vitest globalSetup — compiles the shared `testdata/integration` fixture
 * exactly once before any test files start. The compiled output is consumed
 * by eventstream.test.ts, eventstream-no-companion.test.ts, and upload.test.ts.
 *
 * Why this exists: the previous in-test `buildIntegrationFixture()` used a
 * filesystem marker to coordinate across parallel Vitest workers, but the
 * check-then-write was TOCTOU-racy. Three test files calling it concurrently
 * could each see the marker missing and all start `rmSync(distDir)` +
 * `runTsgonest` against the same output directory — on Windows, the resulting
 * file-lock collisions cause one or more workers to exit 1 (issue #199
 * Cluster B). globalSetup runs in the main vitest process before workers fork,
 * so the build cannot race with itself.
 *
 * If the build fails, the captured stdout/stderr is included in the thrown
 * error so the failing CI run shows the actual tsgonest diagnostics instead
 * of a bare `expected 1 to be 0`.
 */
import { rmSync, existsSync, writeFileSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

const INTEGRATION_DIR = resolve(FIXTURES_DIR, "integration");
const INTEGRATION_DIST = resolve(INTEGRATION_DIR, "dist");
const MARKER = resolve(INTEGRATION_DIST, ".tsgonest-e2e-built");

export default async function setup() {
  if (existsSync(INTEGRATION_DIST)) {
    rmSync(INTEGRATION_DIST, { recursive: true, force: true });
  }

  const result = runTsgonest([
    "--project",
    "testdata/integration/tsconfig.json",
    "--config",
    "testdata/integration/tsgonest.config.json",
  ]);

  if (result.exitCode !== 0) {
    throw new Error(
      `[e2e globalSetup] tsgonest failed to compile testdata/integration ` +
        `(exit ${result.exitCode}).\n` +
        `--- stdout ---\n${result.stdout}\n` +
        `--- stderr ---\n${result.stderr}\n`,
    );
  }

  if (!existsSync(INTEGRATION_DIST)) {
    throw new Error(
      `[e2e globalSetup] tsgonest exited 0 but ${INTEGRATION_DIST} was not created.\n` +
        `--- stdout ---\n${result.stdout}\n` +
        `--- stderr ---\n${result.stderr}\n`,
    );
  }

  // Marker tells in-test ensureCompiled() helpers that the build is already
  // done. Carries the build output so a stale marker (e.g. from a previous
  // run) is distinguishable from one written this run.
  writeFileSync(
    MARKER,
    JSON.stringify({ at: new Date().toISOString(), pid: process.pid }),
  );
}
