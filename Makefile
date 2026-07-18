# §17: one generation pipeline. Generated code is committed; CI regenerates
# and fails on drift. Generator pinned — upgrades are reviewed diffs.
OAPI := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
SPEC := docs/product/08-api/openapi.yaml

.PHONY: gen gen-go gen-ts

gen: gen-go gen-ts

gen-go:
	cp docs/product/11-permissions/rbac-matrix.csv services/api/internal/identity/rbac/matrix.csv
	cd services/api && $(OAPI) -config oapi-types.cfg.yaml ../../$(SPEC)
	cd services/api && $(OAPI) -config oapi-server.cfg.yaml ../../$(SPEC)
	cd packages/contracts/go && $(OAPI) -config oapi.cfg.yaml ../../../$(SPEC)

SQLC := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0

gen-sql:
	cd services/api && $(SQLC) generate

gen-ts:
	cp $(SPEC) apps/console/src/lib/api/openapi.yaml
	pnpm --filter console gen:api

MIGRATE := go run github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
MIGRATIONS_DIR := services/api/internal/platform/db/migrations

# New migration pair with a real-clock timestamp version (founder rule:
# always via the migrate CLI, never hand-named).
migrate-new:
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -format 20060102150405 $(name)
