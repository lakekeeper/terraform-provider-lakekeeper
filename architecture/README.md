# Architecture

Diagrams of how the Terraform provider for Lakekeeper is built, ordered from concepts to code. Read them in this order:

1. [System context](#system-context): what the provider talks to
2. [Domain model](#domain-model): what the provider manages
3. [Apply sequence](#apply-sequence): how one `terraform apply` reaches Lakekeeper
4. [Package layers](#package-layers): where code goes
5. [Assignment module](#assignment-module): how assignment resources are built
6. [Warehouse module](#warehouse-module): how a warehouse is converted to and from the API
7. [Development workflow](#development-workflow): from edit to release

The [Terms](#terms) section defines the words the diagrams use. The [drift table](#drift-table) lists the source each diagram shows.

## Reading and updating the diagrams

Each diagram has three files here, named after its slug:

- `<slug>-light.svg` and `<slug>-dark.svg`: the images shown below, matching your GitHub theme
- `<slug>.html`: an explorable version with zoom, focus, search and themes. GitHub shows its source, so clone the repository or download the file and open it locally in a browser.
- `<slug>.json`: the diagram's source. Review diagram changes in this file.

Reading and reviewing the diagrams needs nothing beyond a browser. Re-rendering one needs [Archify](https://github.com/tt-a1i/archify) and Chrome. When a change touches source listed in the [drift table](#drift-table), update that diagram:

1. Copy `<slug>.json` to `.archify/<type>-<slug>-<YYYYMMDD-HHMMSS>/candidate.json`, where `<type>` is its `diagram_type` and the timestamp is the current local time. `.archify/` is gitignored.
2. Edit the candidate. Set `meta.output` to `<that folder>/<slug>.html` and `meta.repository.revision` to the commit you checked the code at.
3. Render it: `archify finalize <type> <folder>/candidate.json <folder>/<slug>.html --repo-root . --quality showcase`. The `--repo-root` check fails when a cited source file or line range no longer exists at that commit.
4. Export the images: `node scripts/export-diagram-svg.mjs <folder>`.
5. Run `make diagrams`. It copies the newest render of each diagram into this folder under stable names; it does not run Archify.
6. Set the diagram's commit in the drift table to the short form of `meta.repository.revision`.

## System context

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="system-context-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="system-context-light.svg">
  <img alt="System context: Terraform CLI, the provider, the API client layer, the Lakekeeper Go SDK, the Lakekeeper server and the OIDC identity provider" src="system-context-light.svg">
</picture>

Terraform runs the provider as a plugin. The provider's resources and data sources call the API client layer (`internal/provider/api`), the only package that imports the Lakekeeper Go SDK; a `depguard` rule in `.golangci.yml` enforces this. The SDK sends HTTPS requests to the Lakekeeper server's REST API. It authenticates each request with a token that it fetches from the OIDC identity provider using the client credentials grant.

The provider configuration (`endpoint`, `auth_url`, `client_id`, `client_secret`, `scopes`, `cacert_file`, `insecure`) is read once in `Configure`, which builds one `*api.Client` for every resource and data source.

Explorable version: [`system-context.html`](system-context.html) (open locally).

## Domain model

_Not drawn yet._

## Apply sequence

_Not drawn yet._

## Package layers

_Not drawn yet._

## Assignment module

_Not drawn yet._

## Warehouse module

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="warehouse-module-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="warehouse-module-light.svg">
  <img alt="Warehouse module: the plan becomes a warehouse request for Lakekeeper, and Refresh combines the prior state with the server's warehouse into the new state" src="warehouse-module-light.svg">
</picture>

The warehouse module (`internal/provider/warehouse`) owns the warehouse's schemas, models and conversions; `resource_warehouse.go` and `datasource_warehouse.go` only orchestrate. On the way out, `WarehouseRequest` turns the plan's resource model into a warehouse request: exactly one storage profile (S3 or S3-compatible, ADLS or GCS), its storage credential, and the delete profile (`hard`, or `soft` with an expiration; an absent one is sent as `hard`). On the way back, `Refresh` builds the new state from the server's warehouse plus the prior (the plan in Create and Update, the state in Read). The data source reads the same server fields through `DataSourceModelFromAPI` and shares the module's schemas.

Lakekeeper never returns the storage credential. `Refresh` copies it from the prior, and only when the storage profile kept its type; after an import, or when the type changed on the server, it stays null. The data source never holds one.

Explorable version: [`warehouse-module.html`](warehouse-module.html) (open locally).

## Development workflow

_Not drawn yet._

## Terms

The diagrams use these terms exactly. Each group lists the terms a diagram introduces; a term is defined only where it first appears.

### System context terms

- **Server**: one Lakekeeper installation, reached through its REST API at the provider's `endpoint`.
- **Provider**: this Terraform provider. Terraform runs it as a plugin process.
- **API client layer**: the `internal/provider/api` package. It wraps the Lakekeeper Go SDK and gives resources and data sources SDK-neutral types.
- **Lakekeeper Go SDK**: the Go client library for the Lakekeeper REST API ([`go-lakekeeper`](https://github.com/baptistegh/go-lakekeeper)).
- **OIDC identity provider**: the OpenID Connect server that issues the provider's access tokens, for example Keycloak. The provider authenticates with the OAuth 2.0 client credentials grant at `auth_url`.

### Domain model terms

_Not written yet._

### Apply sequence terms

_Not written yet._

### Package layers terms

_Not written yet._

### Assignment module terms

_Not written yet._

### Warehouse module terms

- **Storage profile**: where a warehouse stores its data: exactly one of S3 (or an S3-compatible store), ADLS or GCS, with its location settings.
- **Storage credential**: the secret Lakekeeper uses to access the storage profile. The provider sends it but never reads it back, so it comes only from configuration or prior state.
- **Delete profile**: what happens when a table or view in the warehouse is dropped: `hard` deletes it at once, `soft` keeps it for an expiration period first.
- **Resource model**: the Terraform model of a `lakekeeper_warehouse` resource (`ResourceModel`), read from the plan or state. The data source has its own model without a storage credential.
- **Warehouse request**: what the provider sends to create or update a warehouse: name, storage profile, storage credential and delete profile.
- **Prior state**: the warehouse as Terraform last knew it, passed to `Refresh`: the plan in Create and Update, the stored state in Read.

### Development workflow terms

_Not written yet._

## Drift table

Each diagram, the source it shows, and the commit it was last checked against.

| Diagram | Source it shows | Checked against |
| --- | --- | --- |
| [System context](#system-context) | `main.go`, `internal/provider/provider.go`, `internal/provider/registry.go`, `internal/provider/api/client.go`, `internal/provider/api/config.go`, `go.mod`, `.golangci.yml` | `fce86e9` |
| [Domain model](#domain-model) | _Not drawn yet._ | |
| [Apply sequence](#apply-sequence) | _Not drawn yet._ | |
| [Package layers](#package-layers) | _Not drawn yet._ | |
| [Assignment module](#assignment-module) | _Not drawn yet._ | |
| [Warehouse module](#warehouse-module) | `internal/provider/warehouse/warehouse.go`, `internal/provider/warehouse/storage_profile.go`, `internal/provider/warehouse/storage_credential.go`, `internal/provider/warehouse/delete_profile.go`, `internal/provider/warehouse/schema.go`, `internal/provider/warehouse/storage_profile_schema.go`, `internal/provider/resource_warehouse.go`, `internal/provider/datasource_warehouse.go`, `internal/provider/api/warehouses.go` | `818c1a6` |
| [Development workflow](#development-workflow) | _Not drawn yet._ | |
