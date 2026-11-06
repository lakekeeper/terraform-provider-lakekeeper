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

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="domain-model-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="domain-model-light.svg">
  <img alt="Domain model: server, project, default project, warehouse, namespace, user and role, and an assignment of grant types to an assignee at a scope" src="domain-model-light.svg">
</picture>

The server contains users and projects. A user belongs to the server, never to a project; it only holds grants there. A project contains roles and warehouses, and a warehouse contains namespaces. The default project is an ordinary project; the provider reads it by passing the nil UUID as the project ID (`lakekeeper_default_project`).

An assignment is one assignee holding a set of grant types at one scope. A user or a role can be an assignee. The five kinds of scope are server, project, warehouse, role and namespace, and each has its own grant types. A role appears on both sides: as an assignee it receives grants, and as a scope it has users and other roles assigned to it. Every scope has a `lakekeeper_<scope>_role_assignment` resource, and every scope except namespace also has a `lakekeeper_<scope>_user_assignment` resource.

Explorable version: [`domain-model.html`](domain-model.html) (open locally).

## Apply sequence

_Not drawn yet._

## Package layers

_Not drawn yet._

## Assignment module

_Not drawn yet._

## Warehouse module

_Not drawn yet._

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

- **Project**: a container on the server for warehouses and roles (`lakekeeper_project`).
- **Default project**: the server's default project, an ordinary project that the provider reads by passing the nil UUID as the project ID (`lakekeeper_default_project`).
- **Warehouse**: an Iceberg storage location in a project, with its own storage configuration (`lakekeeper_warehouse`).
- **Namespace**: a group of tables inside a warehouse (`lakekeeper_namespace`).
- **User**: a human or application identity known to the server (`lakekeeper_user`). A user belongs to the server, never to a project.
- **Role**: a named identity that belongs to one project (`lakekeeper_role`). A role is both an assignee, which holds grants, and a scope, which users and other roles are assigned to.
- **Scope**: the server or the project, warehouse, role or namespace that a grant is attached to.
- **Assignee**: the user or role that holds a grant.
- **Grant type**: one permission level allowed at a kind of scope, for example `select` on a warehouse or `admin` on the server.
- **Grant**: one grant type held by one assignee at one scope.
- **Assignment**: the set of grants one assignee holds at one scope. Each `lakekeeper_*_assignment` resource manages one assignment.

### Apply sequence terms

_Not written yet._

### Package layers terms

_Not written yet._

### Assignment module terms

_Not written yet._

### Warehouse module terms

_Not written yet._

### Development workflow terms

_Not written yet._

## Drift table

Each diagram, the source it shows, and the commit it was last checked against.

| Diagram | Source it shows | Checked against |
| --- | --- | --- |
| [System context](#system-context) | `main.go`, `internal/provider/provider.go`, `internal/provider/registry.go`, `internal/provider/api/client.go`, `internal/provider/api/config.go`, `go.mod`, `.golangci.yml` | `fce86e9` |
| [Domain model](#domain-model) | `internal/provider/api/management.go`, `internal/provider/api/permissions.go`, `internal/provider/api/constants.go`, `internal/provider/assignment_adapter.go`, `internal/provider/assignment_descriptor.go`, `internal/provider/resource_user.go`, `internal/provider/resource_role.go`, `internal/provider/resource_project.go`, `internal/provider/resource_namespace.go`, `internal/provider/resource_namespace_role_assignment.go`, `internal/provider/datasource_default_project.go`, `internal/provider/warehouse/warehouse.go` | `818c1a6` |
| [Apply sequence](#apply-sequence) | _Not drawn yet._ | |
| [Package layers](#package-layers) | _Not drawn yet._ | |
| [Assignment module](#assignment-module) | _Not drawn yet._ | |
| [Warehouse module](#warehouse-module) | _Not drawn yet._ | |
| [Development workflow](#development-workflow) | _Not drawn yet._ | |
