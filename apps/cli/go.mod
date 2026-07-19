module github.com/steloit/cloud/apps/cli

go 1.26

require github.com/steloit/cloud/packages/contracts/go v0.0.0

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/oapi-codegen/runtime v1.6.0 // indirect
)

replace github.com/steloit/cloud/packages/contracts/go => ../../packages/contracts/go
