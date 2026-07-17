import { z } from "zod";

const envSchema = z.object({
  /** Origin of the Steloit Platform API. Empty = same origin (canon mode via MSW). */
  VITE_API_URL: z.string().default(""),
  /** "canon" serves 19-canon fixtures through MSW; "live" talks to a real API. */
  VITE_API_MODE: z.enum(["canon", "live"]).default("canon"),
});

const result = envSchema.safeParse(import.meta.env);
if (!result.success) {
  throw new Error(`Invalid environment variables:\n${z.prettifyError(result.error)}`);
}

export const { VITE_API_URL, VITE_API_MODE } = result.data;
