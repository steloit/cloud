/**
 * Invitations are bound to the address they were sent to; the public read
 * only exposes a masked hint (m•••o@acme.dev). The client can still detect
 * the wrong-account state (A7) by matching the visible characters.
 */
export function matchesHint(email: string, hint: string): boolean {
  const [hintLocal, hintDomain] = hint.split("@");
  const [local, domain] = email.split("@");
  if (!hintLocal || !hintDomain || !local || !domain) return false;
  if (domain !== hintDomain) return false;
  const first = hintLocal[0];
  const last = hintLocal[hintLocal.length - 1];
  return local.startsWith(first ?? "") && local.endsWith(last ?? "");
}
