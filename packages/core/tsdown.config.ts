import { defineConfig } from 'tsdown'

export default defineConfig({
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
})
