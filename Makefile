# §17: one generation pipeline. Generated code is committed; CI regenerates
# and fails on drift. Generator pinned — upgrades are reviewed diffs.
OAPI := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
SPEC := docs/product/08-api/openapi.yaml

.PHONY: gen gen-go gen-ts

gen: gen-go gen-ts

gen-go:
	cd services/api && $(OAPI) -config oapi.cfg.yaml ../../$(SPEC)
	cd packages/contracts/go && $(OAPI) -config oapi.cfg.yaml ../../../$(SPEC)

gen-ts:
	cp $(SPEC) apps/console/src/lib/api/openapi.yaml
	pnpm --filter console gen:api
