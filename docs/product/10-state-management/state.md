# State management
- **Server state**: TanStack Query. Keys mirror URLs: `['org',org,'projects']`, `['env',env,'services']`, `['billing',org,'quotas']`. `env` is part of every project-scoped key (env-as-filter).
- **Client state**: URL first (org/project in path, env in `?env=`, filters in params); ephemeral UI (open drawer/modal, drag state) in local component state; a thin global store only for: session, active context {org,project,env}, command-palette, assistant drawer.
- **Cache**: staleTime 30s lists / 5s metrics; invalidate by prefix on mutation; audit/events append via cursor merge.
- **Optimistic updates**: pin/unpin, dismiss/snooze insight, dashboard layout drag, chip filters. NEVER optimistic: anything provisioning/billing/destructive — those render `--prov` or await server truth.
- **Loading**: skeletons per component; provisioning entities poll (2s→backoff 10s) or SSE on `/events`; async drawers (domain U5) poll independently and survive close.
- **Retry**: idempotent GET ×3 exp backoff; mutations no auto-retry — surface error + manual retry; 429 honors Retry-After; failed apply of AI proposal never auto-reapplies (Law 1).
- **Realtime**: event stream drives toasts, bell badge, status pill flips, deploy markers.
