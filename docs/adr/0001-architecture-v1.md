# ADR-0001 · Architecture v1 frozen

**Status:** Accepted · 2026-07-18 · Founder-approved
**Decision:** The production technical architecture in [`docs/architecture.md`](../architecture.md) is frozen as **Architecture v1**. From this date, all implementation, Context Packs, AGENTS.md content, task specifications, and code reviews follow it. Any delta requires a superseding ADR here that names the section it changes and the **measured trigger** that justifies the change — anticipated scale is never a trigger.

**Decision set (index; full text and rationale live in architecture.md — never restated here):**
Go backend (api · cell-agent · CLI) · Vite/React console retained · Postgres everywhere (Neon for customer DBs per INF-001 D3) · Valkey product cache, no internal cache tier · River jobs + reconciler convergence, no brokers · proxied GCS + content eTLD+1 · search deferred, Postgres-first · OTel/Prometheus/Loki/Tempo/Grafana · REST+SSE single contract (no gRPC/GraphQL; WS deferred to the D20 ADR) · owned auth + Dex SSO · sqlc+pgx+golang-migrate · assistant as module, bought inference · GKE zonal + gVisor + cosign + Terraform · modular monolith, lint-enforced boundaries · stdlib net/http · manual DI · caarlos0/env · slog · testify/testcontainers-go/httptest/Playwright · golangci-lint/gofumpt/govulncheck/gitleaks/Renovate/lefthook · one `make gen` OpenAPI pipeline with CI drift check.

**Context:** Produced from the full product corpus (GOV-002, INF-001 A1–A3, the API contract, the built console) after an adversarially-reviewed workflow design (ADR-0002). INF-001 decisions D1–D11 are upstream constraints, not re-decided here.

**Consequences:** SP0-2 (CLI language) is resolved (Go). The engineering-OS artifacts (AGENTS.md, Context Packs, task template) propagate these decisions verbatim. Redesign stops; building starts.
