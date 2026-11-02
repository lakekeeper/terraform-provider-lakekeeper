# AGENTS.md

Guidance for coding agents working in this repository.

## Project Overview

This is a Terraform provider for [Lakekeeper](https://docs.lakekeeper.io/), an Apache Iceberg REST catalog. It is built using the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) (not the older SDKv2). The provider authenticates via OIDC client credentials and manages Lakekeeper resources: warehouses, projects, roles, users, namespaces, and permission assignments at server/project/warehouse/role levels.

## Commands

```sh
make build         # Build provider binary into ./bin/
make test          # Run unit tests (no external dependencies)
make fmt           # Format Go + Terraform + shell files, and fix lint issues
make lint          # Lint without modifying files
make generate      # Re-generate docs from templates (required before committing doc changes)
make reviewable    # Run before committing: build + fmt + generate + test
```

### Running a single test

```sh
# Unit test
RUN=TestFunctionName make test

# Acceptance test (requires testacc-up first)
RUN=TestAccFunctionName make testacc
```

### Acceptance tests

```sh
make testacc-up    # Start Lakekeeper + Keycloak + PostgreSQL + OpenFGA via Docker Compose
make testacc       # Run acceptance tests against local instance (sets TF_ACC=1)
make testacc-down  # Tear down, removing volumes
```

Acceptance tests are tagged with `//go:build acceptance`. Env vars used: `LAKEKEEPER_ENDPOINT`, `LAKEKEEPER_AUTH_URL`, `LAKEKEEPER_CLIENT_ID`, `LAKEKEEPER_CLIENT_SECRET`.

## Architecture

### Code layout

```
internal/provider/
  provider.go          # Provider schema, configuration, client construction
  registry.go          # registerResource / registerDataSource helpers + splitInternalID / DiffTypedStrings utilities
  resource_*.go        # One file per Terraform resource
  datasource_*.go      # One file per Terraform data source
  warehouse_sdk.go     # Model↔SDK conversion helpers for the warehouse resource
  api/config.go        # OIDC + TLS client construction (wraps go-lakekeeper client)
  sdk/                 # Reusable schema fragments and model types (storage profiles, credentials, delete profiles, assignments)
```

### Registration pattern

Every resource and data source calls `registerResource` / `registerDataSource` in an `init()` function. The provider's `Resources()` and `DataSources()` methods return `allResources` / `allDataSources` from `registry.go`. No manual wiring needed when adding a new resource — just add the `init()` call.

### Internal IDs

Composite resources (assignments, namespaces) use a `/`-separated compound internal ID stored in `id` (e.g. `project_id/role_id`). The `splitInternalID` helper in `registry.go` parses these. Warehouse IDs use `:` as a separator (`project_id:warehouse_id`).

### SDK layer (`internal/provider/sdk/`)

Shared schema fragments for complex nested types (storage profiles for S3/ADLS/GCS, storage credentials, delete profiles, assignments) live here so they can be reused across resources and data sources. Conversion between Terraform models and the upstream `go-lakekeeper` SDK types happens in `warehouse_sdk.go` and within each `sdk/` file.

### Upstream client

The provider wraps `github.com/baptistegh/go-lakekeeper`, a typed Go client for the Lakekeeper management API. All API calls go through `r.client` (a `*lakekeeper.Client`) injected via `Configure`.

### Documentation

Docs are generated with `tfplugindocs` from `//go:generate` directives and templates in `templates/`. Run `make generate` after changing schema descriptions. The `lint-generated` target fails CI if docs are stale.

## Commits and branches

- [Conventional Commits](https://www.conventionalcommits.org/), parsed by release-please for versioning and `CHANGELOG.md`. Use only `feat`, `fix`, `docs`, `chore`: the only types release-please is configured for.
- Branches: `type/name`, using the commit types above (e.g. `fix/role-list-pagination`).
- `CHANGELOG.md` and `.release-please-manifest.json` belong to release-please; leave them untouched.
