# AGENTS.md

Guidance for coding agents working in this repository.

## Project overview

A Terraform provider for [Lakekeeper](https://docs.lakekeeper.io/), an Apache Iceberg REST catalog, built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) (SDKv2 APIs do not apply here). It authenticates via OIDC client credentials and manages warehouses, projects, roles, users, namespaces, and permission assignments at server, project, warehouse, role, and namespace level.

## Commands

```sh
make reviewable    # Run before committing: build + fmt + generate + test
make test          # Unit tests; RUN=TestName to select one
make lint          # Lint without modifying files (what CI checks; needs uv for zizmor)
make generate      # Regenerate docs/
```

The `Makefile` has the full target list.

## Contributor workflow

[`CONTRIBUTING.md`](CONTRIBUTING.md) is the source of truth for the development workflow. Read it before writing or running tests (most are acceptance tests behind a build tag), editing docs (`docs/` is generated), committing, or opening a pull request.

## Architecture

All provider code lives in `internal/provider/`: one `resource_*.go` or `datasource_*.go` file per Terraform type, each with its `_test.go`.

- **Registration**: each resource and data source registers itself via `registerResource` / `registerDataSource` in an `init()` (`registry.go`). Adding the file is the whole wiring.
- **Client layer**: all API calls go through `r.client`, an `*api.Client` injected via `Configure`. The `api/` package wraps the Lakekeeper Go SDK ([`github.com/baptistegh/go-lakekeeper`](https://github.com/baptistegh/go-lakekeeper)) and exposes SDK-neutral types (`api.Warehouse`, `api.Assignment`, `api.StorageProfile`, …), enum constants, and `api.IsNotFound`. It is the only package allowed to import the SDK; a depguard rule in `.golangci.yml` enforces this. When an endpoint is missing, add a method to `api/`; do not import the SDK in a resource. Client construction (OIDC, TLS) is in `api/config.go`.
- **Internal IDs**: composite resources store a `/`-joined ID in `id`, which is also the import ID (warehouse: `project_id/warehouse_id`; namespace: `project_id/warehouse_name/name`). `splitInternalID` in `registry.go` parses the two-part form.
- **SDK layer**: `sdk/` holds schema fragments and model types shared between resources and data sources (storage profiles for S3/ADLS/GCS, storage credentials, delete profiles, assignments), along with their Terraform model↔`api` type conversions. The warehouse resource's own conversions are in `warehouse_sdk.go`.
