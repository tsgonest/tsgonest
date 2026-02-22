#!/usr/bin/env node
"use strict";

/**
 * Postinstall script for the `tsgonest` npm package.
 *
 * Production (npm publish):
 *   Copies the pre-built Go binary from the matching @tsgonest/cli-<platform>
 *   optional dependency into bin/tsgonest (or bin/tsgonest.exe on Windows).
 *
 * Development (pnpm workspace / local build):
 *   The cli-* packages don't ship a real binary in source — CI populates them
 *   before publishing.  If the binary can't be found the script warns and exits
 *   cleanly so workspace installs don't fail.
 *
 *   Build the binary manually for local development:
 *     go build -o packages/core/bin/tsgonest ./cmd/tsgonest
 */

const { existsSync, mkdirSync, copyFileSync, chmodSync } = require("fs");
const { join } = require("path");

const PLATFORMS = {
  "darwin-arm64": { pkg: "@tsgonest/cli-darwin-arm64", bin: "tsgonest"     },
  "darwin-x64":   { pkg: "@tsgonest/cli-darwin-x64",   bin: "tsgonest"     },
  "linux-x64":    { pkg: "@tsgonest/cli-linux-x64",    bin: "tsgonest"     },
  "linux-arm64":  { pkg: "@tsgonest/cli-linux-arm64",  bin: "tsgonest"     },
  "win32-x64":    { pkg: "@tsgonest/cli-win32-x64",    bin: "tsgonest.exe" },
};

function getPlatformEntry() {
  return PLATFORMS[`${process.platform}-${process.arch}`] || null;
}

/** True when running inside a pnpm workspace install (no real binaries yet). */
function isWorkspaceDev() {
  return (
    typeof process.env.npm_config_user_agent === "string" &&
    process.env.npm_config_user_agent.startsWith("pnpm/")
  );
}

function install() {
  const entry = getPlatformEntry();

  if (!entry) {
    console.warn(
      `tsgonest: unsupported platform ${process.platform}-${process.arch} ` +
      `(no pre-built binary available)`
    );
    return; // don't hard-fail — library exports still work
  }

  const { pkg, bin } = entry;
  const binDir = join(__dirname, "bin");
  const dest   = join(binDir, bin);

  // If a binary is already present (e.g. dev built via `go build`) leave it.
  if (existsSync(dest)) {
    return;
  }

  try {
    // require.resolve resolves the exact file path inside the npm package.
    // bin name must match what CI wrote: tsgonest on Unix, tsgonest.exe on Windows.
    const src = require.resolve(`${pkg}/bin/${bin}`);

    if (!existsSync(binDir)) {
      mkdirSync(binDir, { recursive: true });
    }

    copyFileSync(src, dest);

    if (process.platform !== "win32") {
      chmodSync(dest, 0o755);
    }
  } catch (err) {
    if (isWorkspaceDev()) {
      // Expected: cli-* packages have empty bin/ stubs until CI populates them.
      console.warn(
        "tsgonest: binary not found in workspace — build it with:\n" +
        "  go build -o packages/core/bin/tsgonest ./cmd/tsgonest"
      );
    } else {
      console.error(`tsgonest: failed to install binary from ${pkg}`);
      console.error(err.message);
      process.exit(1);
    }
  }
}

install();
