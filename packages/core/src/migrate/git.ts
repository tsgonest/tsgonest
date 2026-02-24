/**
 * Git working directory checks for the migrate command.
 * Uses child_process to shell out to git — no dependencies needed.
 */

import { execSync } from "child_process";

/**
 * Check if the current directory is inside a git repository.
 */
export function isGitRepo(cwd: string): boolean {
  try {
    execSync("git rev-parse --is-inside-work-tree", {
      cwd,
      stdio: "pipe",
      encoding: "utf-8",
    });
    return true;
  } catch {
    return false;
  }
}

/**
 * Check if the git working directory is clean (no uncommitted changes).
 * Returns true if clean, false if dirty.
 * Returns true if not in a git repo (nothing to check).
 */
export function isGitClean(cwd: string): boolean {
  if (!isGitRepo(cwd)) return true;

  try {
    const status = execSync("git status --porcelain", {
      cwd,
      stdio: "pipe",
      encoding: "utf-8",
    });
    return status.trim().length === 0;
  } catch {
    // If git status fails, don't block the migration
    return true;
  }
}

/**
 * Get a human-readable summary of uncommitted changes.
 */
export function getGitStatus(cwd: string): string {
  try {
    return execSync("git status --short", {
      cwd,
      stdio: "pipe",
      encoding: "utf-8",
    }).trim();
  } catch {
    return "";
  }
}
