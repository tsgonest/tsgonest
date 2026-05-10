import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    testTimeout: 30000,
    // Compile testdata/integration once before any test files start. See
    // e2e/global-setup.ts for why this lives outside individual test files.
    globalSetup: ["./global-setup.ts"],
  },
});
