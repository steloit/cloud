import { VITE_API_URL } from "@/lib/env";
import type { CreateClientConfig } from "./generated/client.gen";

export const createClientConfig: CreateClientConfig = (config) => ({
  ...config,
  // Server base is <origin>/v1 (08-api: servers[0] = https://api.steloit.app/v1).
  // Empty VITE_API_URL = same origin, where MSW serves the canon world.
  baseUrl: `${VITE_API_URL.replace(/\/+$/, "")}/v1`,
  throwOnError: true,
});
