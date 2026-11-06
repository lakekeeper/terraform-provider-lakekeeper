# Contributor guides

Task guides for changing the provider: which files to create or edit, in what order, and which existing file to copy from. For how the provider is built and why, see the diagrams in [`architecture/README.md`](../architecture/README.md); the guides link there rather than repeat it. For setup, tests, commits and pull requests, see [`CONTRIBUTING.md`](../CONTRIBUTING.md).

Every guide has the same four sections: **Before you start** (the relevant diagram), **Steps**, **Pitfalls** and **Before you open the PR** (a checklist).

## Guides

- [Add a resource or data source](add-resource.md): read before adding a new `lakekeeper_*` resource or data source, other than an assignment resource. It also defines the internal ID.
- [Add an assignment resource](add-assignment-resource.md): read before adding or changing a `lakekeeper_*_assignment` resource: a new grant type, a new assignee at an existing kind of scope, or a new kind of scope.

## Guides and their sources

Each guide and the source files its steps follow. When your change touches one of these files, check that the guide still matches, and update it in the same pull request.

| Guide | Source it follows |
| --- | --- |
| [Add a resource or data source](add-resource.md) | `internal/provider/registry.go`, `internal/provider/provider.go`, `internal/provider/provider_test.go`, `internal/provider/resource_project.go`, `internal/provider/resource_user.go`, `internal/provider/resource_role.go`, `internal/provider/resource_namespace.go`, `internal/provider/datasource_role.go`, `internal/provider/datasource_project.go`, `internal/provider/resource_role_test.go`, `internal/provider/resource_project_test.go`, `internal/provider/datasource_project_test.go`, `internal/provider/resource_warehouse_test.go`, `internal/provider/testutil/helpers.go`, `examples/README.md`, `tools/tools.go`, `Makefile` |
| [Add an assignment resource](add-assignment-resource.md) | `internal/provider/assignment_descriptor.go`, `internal/provider/assignment_resource.go`, `internal/provider/assignment_adapter.go`, `internal/provider/assignment_adapter_*.go`, `internal/provider/assignment_validators.go`, `internal/provider/resource_*_assignment.go`, `internal/provider/resource_*_assignment_test.go`, `internal/provider/assignment_*_unit_test.go`, `internal/provider/assignment_memory_adapter_test.go`, `internal/provider/api/constants.go`, `internal/provider/api/permissions.go`, `internal/provider/api/namespaces.go`, `examples/resources/lakekeeper_*_assignment/` |
