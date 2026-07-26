# Design-to-code mapping

For every frame: components come from `01-design-system` (class contracts), data comes from `08-api`. Pattern per family (the CSV keys the specifics):

| Screen(s) | Components | API calls | Loading | Transitions |
|---|---|---|---|---|
| W1/W2 Home | rail, ctx, snav(home), card grid, spark | GET /orgs/{o}/projects, /billing/overview | skeleton cards | project click → /:org/:project |
| DB1–DB8 | snav(dashboards), chiprow filters, widget cards, modal(DB8), drawer(DB7) | GET/POST /dashboards, /widgets, source queries per widget | per-widget skeleton; filters refetch all | edit⇄view; drag saves on Done |
| C1–C12 | canvas grid, estimate rail, describe card (AI1, policy-gated) | POST /estimates → POST /services | prov pulse after accept | provision → service page |
| S/D/M product | tabs, metric charts, logwell, drawers U2–U5 | GET /services/{id}, /metrics, /events | chart skeletons | ⚑ → drawer prefilled |
| O1–O5 | six-lens nav, chart, tbl, drawer U8 | GET /metrics /logs /traces /alert-rules | stream shimmer | backtest POST inline |
| B1–B12 | billing snav, tbl, matrix(B5), banners, timelines(B10) | GET /billing/*, POST /subscription | number skeletons | plan change → confirm page |
| G/T/X settings | settings snav, tbl, cards, T1 modal | GET/POST respective resources | row skeletons | G7 row → AI3 page |
| AI2/AI4/AI10–12 | assistant snav, thread, evidence rail, prop | /assistant/* | typing indicator | drawer⇄workspace |
| U1–U8 | modal/drawer recipes over parent page | parent's mutation endpoint | button spinner | success → toast + row update |
| A1–A11 | centered card, steps, brand header | /auth, /invites, /orgs | button spinner | stepper advance |
Rule: no screen invents a component; if a screen seems to need one, it's a design-system change first.
