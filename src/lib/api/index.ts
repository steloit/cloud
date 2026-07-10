/** The data-layer entrypoint — features import API types and query helpers from here only. */
export { client, errorMessage, errorRemediation, ProblemError } from "./config";
export * from "./generated/@tanstack/react-query.gen";
export * from "./generated/types.gen";
export { queryClient } from "./query-client";
