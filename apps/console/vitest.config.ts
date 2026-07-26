import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  test: {
    environment: "node",
    pool: "forks",
    isolate: true,
    globals: false,
    clearMocks: true,
    restoreMocks: true,
    include: ["tests/**/*.test.ts"],
  },
});
