import { defineConfig } from 'tsdown'

export default defineConfig([
  // Marker functions (assert, is, validate, stringify, serialize) — the
  // package entry point declared in exports. Without this build, dist/ is
  // missing from the published tarball and marker imports fail (issue #223).
  {
    entry: { index: 'src/index.ts' },
    format: ['esm', 'cjs'],
    outDir: 'dist',
    outExtension: ({ format }) => ({ js: format === 'cjs' ? '.cjs' : '.mjs' }),
    platform: 'node',
    target: 'node18',
    dts: true,
    clean: true,
    sourcemap: false,
  },
  {
    entry: { migrate: 'src/migrate/index.ts' },
    format: 'cjs',
    outDir: 'bin',
    outExtension: () => ({ js: '.cjs' }),
    platform: 'node',
    target: 'node18',
    external: ['typescript', 'jsonc-parser'],
    // Bundle ts-morph and all other deps into a single file
    noExternal: [/^(?!typescript|jsonc-parser$)/],
    inlineOnly: false, // Intentionally bundling all deps for zero-install
    dts: false,
    clean: false, // Don't clean bin/ — it has the Go binary and launcher
    sourcemap: false,
  },
])
