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

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assignment-module-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="assignment-module-light.svg">
  <img alt="Assignment module: an assignment descriptor feeds the assignment resource factory, which reconciles the assignee's direct grants through one scope adapter per kind of scope" src="assignment-module-light.svg">
</picture>

Each of the nine assignment resources is one file that declares an assignment descriptor and registers `newAssignmentResource(descriptor)`. The shared assignment resource (`assignment_resource.go`) derives the schema, internal ID, import and lifecycle from it. A new grant type goes into a descriptor's grant types; a new kind of scope needs a new scope adapter (`assignment_adapter_<scope>.go`) plus the `*api.Client` methods it calls. Scope adapters are the only assignment code that calls the API client layer.

Every create, update and delete reconciles against the server. It reads all direct grants at the scope, keeps only the assignee's own, compares their grant types with the configuration, and sends the revokes and grants as one atomic request, or nothing when they match. Grants that other assignees hold at the scope are never touched, and a failed read never leads to a write. Create also fails if the assignee already holds a direct grant at the scope; import it first.

Explorable version: [`assignment-module.html`](assignment-module.html) (open locally).

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

_Not written yet._

### Apply sequence terms

_Not written yet._

### Package layers terms

_Not written yet._

### Assignment module terms

- **Direct grant**: a grant that Lakekeeper stores for an assignee at a scope, as opposed to one the assignee inherits from a role or a parent scope. Assignment resources read and write only direct grants.
- **Assignment descriptor**: the `assignmentDescriptor` value that declares one assignment resource: its identity attributes, allowed grant types and scope adapter.
- **Assignment resource factory**: `newAssignmentResource`, which turns an assignment descriptor into a complete Terraform resource.
- **Scope adapter**: the `scopeAdapter` for one kind of scope. It lists the direct grants at a scope and applies revokes and grants in one request through the API client layer.

### Warehouse module terms

_Not written yet._

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
| [Assignment module](#assignment-module) | `internal/provider/assignment_descriptor.go`, `internal/provider/assignment_resource.go`, `internal/provider/assignment_adapter.go`, `internal/provider/assignment_adapter_*.go`, `internal/provider/resource_*_assignment.go`, `internal/provider/registry.go`, `internal/provider/api/permissions.go`, `internal/provider/api/namespaces.go` | `818c1a6` |
| [Warehouse module](#warehouse-module) | _Not drawn yet._ | |
| [Development workflow](#development-workflow) | _Not drawn yet._ | |
