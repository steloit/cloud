import { client } from "./generated/client.gen";

/**
 * Errors are problem+json (RFC 9457) and always carry `remediation` — a next
 * step, never a dead end (08-api x-conventions). This module normalizes every
 * non-2xx response into a typed ProblemError at the client layer.
 */
export interface Problem {
  type?: string;
  title: string;
  status: number;
  detail?: string;
  remediation: string;
  errors?: Array<{ field: string; message: string }>;
  reasons?: string[];
  required_plan?: string;
}

export class ProblemError extends Error {
  readonly problem: Problem;

  constructor(problem: Problem) {
    super(problem.detail ?? problem.title);
    this.name = "ProblemError";
    this.problem = problem;
  }
}

function isProblem(body: unknown): body is Problem {
  return (
    typeof body === "object" &&
    body !== null &&
    "title" in body &&
    "status" in body &&
    "remediation" in body
  );
}

export function errorMessage(error: unknown): string {
  if (error instanceof ProblemError) return error.message;
  if (error instanceof Error) return error.message;
  return "Request failed";
}

/** The remediation line every failure surface must name (16-qa). */
export function errorRemediation(error: unknown): string | undefined {
  return error instanceof ProblemError ? error.problem.remediation : undefined;
}

async function normalizeResponse(response: Response): Promise<Response> {
  if (response.ok) return response;
  let body: unknown = null;
  try {
    body = await response.clone().json();
  } catch {
    body = null;
  }
  if (isProblem(body)) throw new ProblemError(body);
  throw new ProblemError({
    title: response.statusText || "Request failed",
    status: response.status,
    remediation: "Retry, and contact support if it persists.",
  });
}

client.interceptors.response.use(normalizeResponse);

export { client };
