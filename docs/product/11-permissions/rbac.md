# RBAC

Roles are org-level; projects may restrict membership ("restricted" visibility). Tokens act as their user and shrink with role changes; org API keys carry explicit scopes. The matrix (CSV) is authoritative; UI rule: gated actions are visible-but-disabled with the reason (never hidden), see B6. AI reads with the *viewer's* permissions (Law 3) and can never hold apply rights (Law 1).

## The environment dimension — how a role Y becomes a production 403

Authorization is evaluated in two layers, in order:

1. **Role matrix (the ceiling).** A `Y` in `rbac-matrix.csv` means the role *may* perform the action — it is the maximum grant, not the final answer. An `N` is final: no policy can widen it (tighten-only, per G4's inheritance rule).
2. **Policy evaluation (the environment-sensitive layer).** Policies (GOV-002 §1's eighth primitive) attach at org, project, or environment level and may *narrow* a role grant per environment — never widen it. GOV-002 §2.1 sets the pattern: "crossing environments requires elevated permission — this is a Policy default." Environment-sensitive restrictions (production promotion gates, preview-minimal service exclusions, compute ceilings, region allowlists) are **always policies, never role rows** — which is why the matrix has no environment column.

So G2's canon line — "Developer's production limits are E3's 403 working as designed" — decomposes as: the role-level denials Developers feel most in production (`service.delete`, `project.delete`, `policy.manage`, `template.manage` are `N` outright), plus any org policies that tighten remaining grants for the production environment. A denial from either layer renders in E3's grammar: who you are, what's required, who can grant it, denial audited.

**Evaluation contract (for implementers):** `allow = matrix[role][permission] == Y AND policies.evaluate(actor, permission, {org, project, env}) == permit`. Policy denials carry the policy's name into the problem+json `detail` (the E3 requirement that the denial names its source); matrix denials name the missing role. Both are audited as events.

**Special rows:** `ai.apply_proposal` is "per underlying action" — applying a proposal re-runs this whole evaluation for the change the proposal contains; the assistant never short-circuits either layer. `tokens.own` is universal because personal tokens are the user (P5); they inherit this same two-layer evaluation at use time, evaluated against the user's *current* roles.
