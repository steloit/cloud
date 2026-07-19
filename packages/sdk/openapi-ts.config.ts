import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "../../docs/product/08-api/openapi.yaml",
  output: { path: "./src/generated", tsConfigPath: "off" },
  plugins: [
    { name: "@hey-api/client-fetch" },
    "@hey-api/typescript",
    "@hey-api/sdk",
  ],
});
