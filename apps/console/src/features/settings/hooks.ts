import { useMutation, useQuery } from "@tanstack/react-query";
import {
  changeMemberRoleMutation,
  createApiKeyMutation,
  createPersonalTokenMutation,
  createPolicyMutation,
  listApiKeysOptions,
  listMembersOptions,
  listPersonalTokensOptions,
  listPoliciesOptions,
  listTemplatesOptions,
  removeMemberMutation,
  revokePersonalTokenMutation,
} from "@/lib/api";

/** Settings-plane data hooks (G1–G12 · T1 · P5) — thin wrappers over generated helpers. */

export function useMembers(org: string) {
  // Full MemberList — G6's header needs seats { included, used } alongside rows.
  return useQuery(listMembersOptions({ path: { org } }));
}

export function useChangeRole() {
  return useMutation(changeMemberRoleMutation());
}

export function useRemoveMember() {
  return useMutation(removeMemberMutation());
}

export function useApiKeys(org: string) {
  return useQuery({ ...listApiKeysOptions({ path: { org } }), select: (r) => r.data ?? [] });
}

export function useCreateApiKey() {
  return useMutation(createApiKeyMutation());
}

export function usePersonalTokens() {
  return useQuery({ ...listPersonalTokensOptions(), select: (r) => r.data ?? [] });
}

export function useCreatePersonalToken() {
  return useMutation(createPersonalTokenMutation());
}

export function useRevokeToken() {
  return useMutation(revokePersonalTokenMutation());
}

export function usePolicies(org: string) {
  return useQuery({ ...listPoliciesOptions({ path: { org } }), select: (r) => r.data ?? [] });
}

export function useCreatePolicy() {
  return useMutation(createPolicyMutation());
}

/**
 * Dry-run variant — POST /policies?dry_run=true returns PolicyImpact:
 * preview before enforce, the governance analog of estimate-before-provision (G9).
 */
export function useCreatePolicyDryRun() {
  return useMutation(createPolicyMutation({ query: { dry_run: true } }));
}

export function useTemplatesList(org: string) {
  return useQuery({ ...listTemplatesOptions({ path: { org } }), select: (r) => r.data ?? [] });
}
