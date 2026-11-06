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

[`architecture/README.md`](architecture/README.md) has the diagrams of this structure; read it to orient yourself before exploring code.

**Diagram drift rule**: when a change touches source listed in that README's drift table, update the diagram as [Reading and updating the diagrams](architecture/README.md#reading-and-updating-the-diagrams) describes.

- **Registration**: each resource and data source registers itself via `registerResource` / `registerDataSource` in an `init()` (`registry.go`). Adding the file is the whole wiring.
- **Client layer**: all API calls go through `r.client`, an `*api.Client` injected via `Configure`. The `api/` package wraps the Lakekeeper Go SDK ([`github.com/baptistegh/go-lakekeeper`](https://github.com/baptistegh/go-lakekeeper)) and exposes SDK-neutral types (`api.Warehouse`, `api.Assignment`, `api.StorageProfile`, …), enum constants, and `api.IsNotFound`. It is the only package allowed to import the SDK; a depguard rule in `.golangci.yml` enforces this. When an endpoint is missing, add a method to `api/`; do not import the SDK in a resource. Client construction (OIDC, TLS) is in `api/config.go`.
- **Internal IDs**: composite resources store a `/`-joined ID in `id`, which is also the import ID (warehouse: `project_id/warehouse_id`; namespace: `project_id/warehouse_name/name`). `splitInternalID` in `registry.go` parses the two-part form; assignment resources build and parse their own IDs instead.
- **Assignment module**: assignment resources are just an `assignmentDescriptor` (identity attributes, grant types, scope adapter) passed to `newAssignmentResource`. `assignment_resource.go` derives schema, ID, import and lifecycle from it; a `scopeAdapter` per scope (`assignment_adapter_*.go`) is the only assignment code that calls `api.Client`.
- **Warehouse module**: `warehouse/` owns the warehouse's resource and data-source schemas, both models, and all Terraform model↔`api` conversions for the storage profile (S3/ADLS/GCS), storage credential and delete profile. Its surface is `ResourceSchema`, `DataSourceSchema`, `ResourceModel.WarehouseRequest`, `Refresh(prior, warehouse)` and `DataSourceModelFromAPI`; conversions return Go errors, not diagnostics. Both storage-profile schemas render from one description (`storage_profile_schema.go`). `resource_warehouse.go` and `datasource_warehouse.go` keep only lifecycle orchestration. Its unit tests need no server.
- **SDK layer**: `sdk/` holds the assignment schema fragment and model shared between the assignments data sources.
