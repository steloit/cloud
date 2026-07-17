# Forms — fields, validation, payloads

Conventions: required unless "(opt)"; errors inline under field, plain language + fix; defaults shown; payloads are the JSON bodies of `08-api/openapi.yaml`.

## Create organization (A5)
name (3–32, `[a-z0-9-]` slug preview, immutable) · home_region (select, default aws/ap-south-1).
`POST /orgs {"name":"Acme","home_region":"aws/ap-south-1"}` → 201 `{id,slug,...}`. Errors: 409 slug taken.

## Invite member (A11, U1)
email (RFC, dedupe) · role (developer default | admin | billing).
`POST /orgs/{org}/invites {"email":"sam@acme.dev","role":"developer"}` → 201 `{id,expires_at}` (7d). 409 already member/pending; 402 seat limit (soft: returns `overage_price`).

## Create project (W2/A8)
name · template_id (opt) · region (opt, defaults org home).
`POST /orgs/{org}/projects {"name":"my-app","template_id":"tpl_9c22e1"}`.

## Create service (C-series)
product · name (unique in env) · shape{cpu,mem,version,extensions[]} · env.
Estimate previewed via `POST /estimates` before `POST /projects/{p}/envs/{e}/services`.

## Create binding (U2)
target_service · scope (`read-only` default | `read-write`).
`POST /services/{id}/bindings {"target":"svc_dbreports","scope":"read-only"}` → 201 `{env_vars:{DATABASE_REPORTS_URL:"…masked"}}`. 409 duplicate binding.

## Lifecycle rule (U3)
prefix (path-like) · action (`expire`|`tier_cold`) · after_days (int ≥1, default 7). Dry-run: `POST …/lifecycle-rules:dryRun` → `{objects,bytes,monthly_savings}`.

## Schedule (U4)
name (slug) · cron (5-field, validated; preview `GET …/schedules:preview?cron=`) · payload_template (JSON, `{{date}}/{{run_id}}` allowed) · tz (default org).

## Custom domain (U5)
domain (FQDN, no wildcard on Free). `POST /services/{id}/domains {"domain":"shop.acme.dev"}` → `{status:"verifying",records:[{type:"CNAME",...},{type:"TXT",...}]}`; poll `GET`.

## Personal token / org key (U7)
name · scope (`full`|`read-only`) · expires_in_days (30|60|90|365, default 90).
`POST /me/tokens` → 201 `{token:"stp_…", shown_once:true, prefix, hash_stored:true}`.

## Alert rule (U8)
query (metric expr) · condition (op+value+unit) · window (1|5|15m) · routes[] (bell,email,webhook) · name auto from query (editable). Backtest: `POST /alert-rules:backtest {query,condition,window,days:7}`.

## Save as template (T3)
name · visibility (`org`|`restricted`) · source `{project,env}` · services[] (subset). Server strips secrets; excluded-target bindings → `required_inputs[]` in the template.

## New dashboard (DB8)
name · scope (`org`|`project:{id}`) · visibility (`personal`|`org`|`restricted`) · start_from (`blank`|`template:{id}`|`duplicate:{id}`).

## Plan change / cancel (B11/B12)
`POST /orgs/{org}/subscription {"plan":"pro"}` (immediate, prorated) · `DELETE …/subscription` (effective at anchor; response echoes the wind-down contract) · optional `{reason_code}`.

## Auth (A1/A2)
email+password (min 12, zxcvbn ≥3) or SSO (Business+); MFA TOTP 6-digit.
