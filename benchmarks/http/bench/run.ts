import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import autocannon from 'autocannon';

// ── Types ──────────────────────────────────────────────────

interface BenchConfig {
  name: string;
  command: string;
  args: string[];
  cwd: string;
}

interface ScenarioResult {
  reqPerSec: number;
  latencyP50: number;
  latencyP99: number;
  throughputMBps: number;
}

interface BenchmarkResults {
  adapter: string;
  scenarios: Record<string, ScenarioResult>;
}

// ── Configuration ──────────────────────────────────────────

const PORT = 3000;
const BASE_URL = `http://localhost:${PORT}`;
const WARMUP_DURATION = 5;
const BENCH_DURATION = 15;
// Parse --connections=N from CLI args, default 100
const CONNECTIONS = (() => {
  const arg = process.argv.find(a => a.startsWith('--connections='));
  return arg ? parseInt(arg.split('=')[1], 10) : 100;
})();

const rootDir = new URL('../../..', import.meta.url).pathname;
const httpDir = new URL('..', import.meta.url).pathname;

const ADAPTERS: BenchConfig[] = [
  {
    name: 'Express (Node)',
    command: 'node',
    args: ['--enable-source-maps', `${httpDir}dist/main-express.js`],
    cwd: rootDir,
  },
  {
    name: 'Fastify (Node)',
    command: 'node',
    args: ['--enable-source-maps', `${httpDir}dist/main-fastify.js`],
    cwd: rootDir,
  },
  {
    name: 'Bun Adapter',
    command: 'bun',
    args: [`${httpDir}dist/main-bun.js`],
    cwd: rootDir,
  },
];

const CREATE_USER_BODY = JSON.stringify({
  name: 'Benchmark User',
  email: 'bench@example.com',
  age: 30,
  isActive: true,
  role: 'user',
});

// Scenarios include both tsgonest-powered (/users) and plain JSON.stringify (/plain) endpoints
const ALL_SCENARIOS: Array<{
  name: string;
  method: string;
  path: string;
  body?: string;
  headers?: Record<string, string>;
}> = [
  // ── tsgonest serialization (generated stringify) ──
  { name: 'GET /users (tsgonest, list 20)', method: 'GET', path: '/users' },
  { name: 'POST /users (tsgonest, validate+serialize)', method: 'POST', path: '/users', body: CREATE_USER_BODY, headers: { 'content-type': 'application/json' } },
  { name: 'GET /users/:id (tsgonest, single)', method: 'GET', path: '/users/550e8400-e29b-41d4-a716-446655440000' },
  // ── plain JSON.stringify (no tsgonest) ──
  { name: 'GET /plain (JSON.stringify, list 20)', method: 'GET', path: '/plain' },
  { name: 'POST /plain (no validation)', method: 'POST', path: '/plain', body: CREATE_USER_BODY, headers: { 'content-type': 'application/json' } },
  { name: 'GET /plain/:id (JSON.stringify, single)', method: 'GET', path: '/plain/550e8400-e29b-41d4-a716-446655440000' },
];

// --quick flag: only tsgonest scenarios (skip plain)
const quickMode = process.argv.includes('--quick');
const SCENARIOS = quickMode ? ALL_SCENARIOS.filter(s => s.path.startsWith('/users')) : ALL_SCENARIOS;

// ── Helpers ────────────────────────────────────────────────

async function startServer(config: BenchConfig): Promise<ChildProcess> {
  const proc = spawn(config.command, config.args, {
    cwd: config.cwd,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env, PORT: String(PORT) },
  });

  return new Promise<ChildProcess>((resolve, reject) => {
    const timeout = setTimeout(() => {
      proc.kill();
      reject(new Error(`Server ${config.name} failed to start within 10s`));
    }, 10000);

    let output = '';
    proc.stdout!.on('data', (chunk: Buffer) => {
      output += chunk.toString();
      if (output.includes('LISTENING')) {
        clearTimeout(timeout);
        resolve(proc);
      }
    });

    proc.stderr!.on('data', () => {});

    proc.on('error', (err) => {
      clearTimeout(timeout);
      reject(err);
    });

    proc.on('exit', (code) => {
      if (!output.includes('LISTENING')) {
        clearTimeout(timeout);
        reject(new Error(`Server ${config.name} exited with code ${code} before LISTENING`));
      }
    });
  });
}

async function killServer(proc: ChildProcess): Promise<void> {
  if (!proc.killed) {
    proc.kill('SIGTERM');
    const exitPromise = once(proc, 'exit');
    const timeout = setTimeout(() => proc.kill('SIGKILL'), 5000);
    await exitPromise;
    clearTimeout(timeout);
  }
}

async function runScenario(
  scenario: typeof SCENARIOS[number],
): Promise<ScenarioResult> {
  // Warmup
  await new Promise<void>((resolve) => {
    const inst = autocannon({
      url: BASE_URL + scenario.path,
      method: scenario.method as any,
      body: scenario.body,
      headers: scenario.headers,
      duration: WARMUP_DURATION,
      connections: CONNECTIONS,
    });
    inst.on('done', () => resolve());
  });

  // Actual benchmark
  return new Promise<ScenarioResult>((resolve) => {
    const inst = autocannon({
      url: BASE_URL + scenario.path,
      method: scenario.method as any,
      body: scenario.body,
      headers: scenario.headers,
      duration: BENCH_DURATION,
      connections: CONNECTIONS,
    });
    inst.on('done', (result: any) => {
      resolve({
        reqPerSec: result.requests.average,
        latencyP50: result.latency.p50,
        latencyP99: result.latency.p99,
        throughputMBps: Math.round(result.throughput.average / 1024 / 1024 * 100) / 100,
      });
    });
  });
}

// ── Main ───────────────────────────────────────────────────

async function main() {
  console.log('=== tsgonest HTTP Benchmark (v2 — optimized) ===\n');
  console.log(`Duration: ${WARMUP_DURATION}s warmup + ${BENCH_DURATION}s measurement`);
  console.log(`Connections: ${CONNECTIONS}\n`);

  const allResults: BenchmarkResults[] = [];

  for (const adapter of ADAPTERS) {
    console.log(`\n--- ${adapter.name} ---\n`);

    let proc: ChildProcess;
    try {
      proc = await startServer(adapter);
    } catch (err: any) {
      console.log(`  SKIP: ${err.message}`);
      continue;
    }

    const results: Record<string, ScenarioResult> = {};

    for (const scenario of SCENARIOS) {
      process.stdout.write(`  ${scenario.name}... `);
      try {
        results[scenario.name] = await runScenario(scenario);
        console.log(`${results[scenario.name].reqPerSec.toLocaleString()} req/s`);
      } catch (err: any) {
        console.log(`ERROR: ${err.message}`);
      }
    }

    await killServer(proc);
    allResults.push({ adapter: adapter.name, scenarios: results });
  }

  // ── Print results ──

  // Group scenarios by type
  const tsgonestScenarios = SCENARIOS.filter(s => s.name.includes('tsgonest') || s.path.startsWith('/users'));
  const plainScenarios = SCENARIOS.filter(s => s.name.includes('JSON.stringify') || s.name.includes('no validation'));

  console.log('\n\n## With tsgonest serialization\n');
  console.log('| Scenario | Adapter | req/s | p50 (ms) | p99 (ms) | MB/s |');
  console.log('|----------|---------|------:|--------:|---------:|-----:|');
  for (const scenario of tsgonestScenarios) {
    for (const result of allResults) {
      const s = result.scenarios[scenario.name];
      if (!s) continue;
      console.log(`| ${scenario.name} | ${result.adapter} | ${s.reqPerSec.toLocaleString()} | ${s.latencyP50} | ${s.latencyP99} | ${s.throughputMBps} |`);
    }
  }

  console.log('\n## Without tsgonest (plain JSON.stringify)\n');
  console.log('| Scenario | Adapter | req/s | p50 (ms) | p99 (ms) | MB/s |');
  console.log('|----------|---------|------:|--------:|---------:|-----:|');
  for (const scenario of plainScenarios) {
    for (const result of allResults) {
      const s = result.scenarios[scenario.name];
      if (!s) continue;
      console.log(`| ${scenario.name} | ${result.adapter} | ${s.reqPerSec.toLocaleString()} | ${s.latencyP50} | ${s.latencyP99} | ${s.throughputMBps} |`);
    }
  }

  // ── Speedup tables ──
  const expressResults = allResults.find((r) => r.adapter.includes('Express'));
  if (expressResults) {
    console.log('\n## Speedup vs Express\n');
    for (const result of allResults) {
      if (result === expressResults) continue;
      console.log(`### ${result.adapter}\n`);
      for (const scenario of SCENARIOS) {
        const e = expressResults.scenarios[scenario.name];
        const s = result.scenarios[scenario.name];
        if (!e || !s) continue;
        const ratio = s.reqPerSec / e.reqPerSec;
        console.log(`  ${scenario.name}: ${ratio.toFixed(2)}x`);
      }
      console.log('');
    }
  }

  // ── tsgonest vs JSON.stringify comparison (per adapter) ──
  console.log('\n## tsgonest serialize vs JSON.stringify (same adapter)\n');
  for (const result of allResults) {
    const tsList = result.scenarios['GET /users (tsgonest, list 20)'];
    const plainList = result.scenarios['GET /plain (JSON.stringify, list 20)'];
    const tsSingle = result.scenarios['GET /users/:id (tsgonest, single)'];
    const plainSingle = result.scenarios['GET /plain/:id (JSON.stringify, single)'];

    if (tsList && plainList) {
      const ratio = tsList.reqPerSec / plainList.reqPerSec;
      console.log(`  ${result.adapter} — list 20: tsgonest ${ratio > 1 ? ratio.toFixed(2) + 'x faster' : (1/ratio).toFixed(2) + 'x slower'} (${tsList.reqPerSec.toLocaleString()} vs ${plainList.reqPerSec.toLocaleString()} req/s)`);
    }
    if (tsSingle && plainSingle) {
      const ratio = tsSingle.reqPerSec / plainSingle.reqPerSec;
      console.log(`  ${result.adapter} — single: tsgonest ${ratio > 1 ? ratio.toFixed(2) + 'x faster' : (1/ratio).toFixed(2) + 'x slower'} (${tsSingle.reqPerSec.toLocaleString()} vs ${plainSingle.reqPerSec.toLocaleString()} req/s)`);
    }
  }
}

main().catch(console.error);
