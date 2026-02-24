#!/usr/bin/env node

/**
 * Compares two benchmark JSON files and generates a Markdown table
 * showing performance changes between base and PR.
 *
 * Usage: node compare-benchmarks.js <base.json> <pr.json>
 */

const { readFileSync } = require("node:fs");

const THRESHOLD = 5; // % change threshold for regression/improvement detection

const [, , baseFile, prFile] = process.argv;

if (!baseFile || !prFile) {
  console.error("Usage: node compare-benchmarks.js <base.json> <pr.json>");
  process.exit(1);
}

const base = JSON.parse(readFileSync(baseFile, "utf-8"));
const pr = JSON.parse(readFileSync(prFile, "utf-8"));

const allKeys = new Set([
  ...Object.keys(base.results || {}),
  ...Object.keys(pr.results || {}),
]);

function formatOps(hz) {
  if (hz >= 1e6) return (hz / 1e6).toFixed(1) + "M";
  if (hz >= 1e3) return (hz / 1e3).toFixed(1) + "K";
  return String(Math.round(hz));
}

function changeIndicator(pct) {
  if (pct > THRESHOLD) return ":arrow_up:";
  if (pct < -THRESHOLD) return ":arrow_down:";
  return "-";
}

let regressions = 0;
const rows = [];

for (const key of [...allKeys].sort()) {
  const baseResult = base.results && base.results[key];
  const prResult = pr.results && pr.results[key];

  if (!baseResult || !prResult) continue;

  const baseHz = baseResult.hz;
  const prHz = prResult.hz;
  const changePct = ((prHz - baseHz) / baseHz) * 100;

  if (changePct < -THRESHOLD) regressions++;

  const sign = changePct > 0 ? "+" : "";
  rows.push(
    "| " + key + " | " + formatOps(baseHz) + " | " + formatOps(prHz) + " | " + sign + changePct.toFixed(1) + "% " + changeIndicator(changePct) + " |"
  );
}

// Output Markdown
console.log("## Benchmark Results");
console.log();
console.log("| Benchmark | Base (ops/s) | PR (ops/s) | Change |");
console.log("|-----------|-------------|-----------|--------|");
for (const row of rows) {
  console.log(row);
}
console.log();

if (regressions > 0) {
  console.log(
    "> :warning: **" + regressions + " regression" + (regressions > 1 ? "s" : "") + " detected** (>" + THRESHOLD + "% slower)"
  );
} else {
  console.log("> :white_check_mark: No regressions detected");
}

console.log();
console.log(
  "<sub>Base: " + (base.node_version || "?") + " | PR: " + (pr.node_version || "?") + " | Threshold: " + THRESHOLD + "%</sub>"
);
