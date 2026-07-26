/**
 * Layer 1 of the two-layer AuthZ model (11-permissions/rbac.md): the role
 * matrix is the ceiling; policies (layer 2) only narrow, and are evaluated
 * server-side. UI rule: gated actions are visible-but-disabled with the
 * stated reason, never hidden (B6 grammar).
 */
export type Role = "owner" | "admin" | "developer" | "billing";

export type Permission =
  | "project.create"
  | "project.delete"
  | "env.manage"
  | "service.create"
  | "service.scale"
  | "service.delete"
  | "binding.manage"
  | "deploy.promote"
  | "deploy.rollback"
  | "observe.read"
  | "members.invite"
  | "billing.view"
  | "audit.read";

/** Verbatim rows from 11-permissions/rbac-matrix.csv (Phase-1 subset). */
const MATRIX: Record<Permission, Record<Role, boolean>> = {
  "project.create": { owner: true, admin: true, developer: true, billing: false },
  "project.delete": { owner: true, admin: true, developer: false, billing: false },
  "env.manage": { owner: true, admin: true, developer: true, billing: false },
  "service.create": { owner: true, admin: true, developer: true, billing: false },
  "service.scale": { owner: true, admin: true, developer: true, billing: false },
  "service.delete": { owner: true, admin: true, developer: false, billing: false },
  "binding.manage": { owner: true, admin: true, developer: true, billing: false },
  "deploy.promote": { owner: true, admin: true, developer: true, billing: false },
  "deploy.rollback": { owner: true, admin: true, developer: true, billing: false },
  "observe.read": { owner: true, admin: true, developer: true, billing: false },
  "members.invite": { owner: true, admin: true, developer: false, billing: false },
  "billing.view": { owner: true, admin: true, developer: false, billing: true },
  "audit.read": { owner: true, admin: true, developer: false, billing: false },
};

export function can(role: Role, permission: Permission): boolean {
  return MATRIX[permission][role];
}

/** E3 grammar: the denial names what's required. */
export function denialReason(permission: Permission): string {
  return `Requires a role with ${permission} — ask an org owner or admin.`;
}
