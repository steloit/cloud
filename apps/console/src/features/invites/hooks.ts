import { useMutation, useQuery } from "@tanstack/react-query";
import {
  acceptInviteMutation,
  client,
  createInviteMutation,
  declineInviteMutation,
  getInvitePublicOptions,
} from "@/lib/api";

export function useInvitePublic(invite: string) {
  return useQuery({ ...getInvitePublicOptions({ path: { invite } }), retry: false });
}

export function useAcceptInvite() {
  return useMutation(acceptInviteMutation());
}

export function useDeclineInvite() {
  return useMutation(declineInviteMutation());
}

export function useRenewInvite() {
  // openapi.yaml omits the {invite} path parameter on /invites/{invite}/renew
  // (generated RenewInviteData has `path?: never`) — spec defect, surfaced as
  // a finding. Until fixed, the URL is built against the configured client.
  return useMutation({
    mutationFn: (invite: string) => client.post({ url: `/invites/${invite}/renew` }),
  });
}

export function useCreateInvite() {
  return useMutation(createInviteMutation());
}
