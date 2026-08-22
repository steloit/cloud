# §17: one generation pipeline. Generated code is committed; CI regenerates
# and fails on drift. Generator pinned — upgrades are reviewed diffs.
OAPI := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
SPEC := docs/product/08-api/openapi.yaml

.PHONY: gen gen-go gen-sql gen-ts gen-canon migrate-new

gen: gen-go gen-ts

gen-go:
	cp docs/product/11-permissions/rbac-matrix.csv services/api/internal/identity/rbac/matrix.csv
	cd services/api && $(OAPI) -config oapi-types.cfg.yaml ../../$(SPEC)
	cd services/api && $(OAPI) -config oapi-server.cfg.yaml ../../$(SPEC)
	cd packages/contracts/go && $(OAPI) -config oapi.cfg.yaml ../../../$(SPEC)

SQLC := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0

# The rm is not tidiness — it is what makes the CI drift gate able to see a
# DELETED query. sqlc writes output but never removes stale output, and the gate
# can only diff what the generator writes. So deleting db/queries/mfa.sql left
# internal/identity/store/mfa.sql.go on disk, still compiling and still callable
# with six live query functions, and the gate reported GREEN. That is US-1.3a's
# property ("the SQL that executes is the SQL that was reviewed") approached from
# deletion instead of edit. A RENAME was already caught, because the new file
# appears and stages; only deletion slipped through.
#
# Sweeping the directory and regenerating makes a deleted query show up as a
# staged deletion, which the gate's `git diff --cached --exit-code` fails on.
gen-sql:
	rm -f services/api/internal/identity/store/*.sql.go 	      services/api/internal/identity/store/models.go 	      services/api/internal/identity/store/db.go
	cd services/api && $(SQLC) generate

# gen-canon syncs the canon fixtures into every consumer copy. Pure shell (no
# toolchain), so the go job can run it and git-diff the result — the same
# drift-fails-CI contract as rbac-matrix.csv.
gen-canon:
	cp docs/product/19-canon/fixtures.json packages/canon/src/fixtures.json
	cp docs/product/19-canon/fixtures.json apps/console/src/lib/canon/fixtures.json

gen-ts: gen-canon
	cp $(SPEC) apps/console/src/lib/api/openapi.yaml
	pnpm --filter console gen:api
	pnpm --filter @steloit/sdk gen:api

MIGRATE := go run github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
MIGRATIONS_DIR := services/api/internal/platform/db/migrations

# New migration pair with a real-clock timestamp version (founder rule:
# always via the migrate CLI, never hand-named).
migrate-new:
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -format 20060102150405 $(name)
