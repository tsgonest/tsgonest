import { defineConfig } from 'tsdown'

export default defineConfig({
  entry: ['src/index.ts', 'src/tags.ts'],
  format: ['esm', 'cjs'],
  dts: true,
  clean: true,
})
