import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "./src/lib/api/openapi.yaml",
  output: { path: "./src/lib/api/generated", tsConfigPath: "off" },
  plugins: [
    { name: "@hey-api/client-fetch", runtimeConfigPath: "./src/lib/api/runtime-config.ts" },
    "@hey-api/typescript",
    "@hey-api/sdk",
    {
      name: "@tanstack/react-query",
      queryOptions: true,
      mutationOptions: true,
      queryKeys: true,
    },
  ],
});
