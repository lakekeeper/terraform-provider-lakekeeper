# Testing

Read this before writing or running tests. It tells you which kind of test to write, how to tag it, which helpers to reuse, and how to run just your test. The commands themselves are in [`CONTRIBUTING.md`](../CONTRIBUTING.md#tests); this guide adds which test to write and how.

The words in **bold** below, and the other terms the guide uses, are defined in the architecture README's [Terms](../architecture/README.md#terms). Internal IDs are defined in [Add a resource or data source](add-resource.md#internal-id).

## Before you start

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="../architecture/development-workflow-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="../architecture/development-workflow-light.svg">
  <img alt="Development workflow: edit, make reviewable, acceptance tests on the local stack, conventional commit, pull request, release-please; docs/ and CHANGELOG.md are generated" src="../architecture/development-workflow-light.svg">
</picture>

Tests sit in two places in the workflow: `make reviewable` runs the unit tests, and `make testacc` runs the **acceptance tests** against the **local stack**. CI runs both on every pull request, the acceptance tests against several Lakekeeper versions. See [Development workflow](../architecture/README.md#development-workflow).

There are two kinds of test. Pick by what you need to prove:

| | Unit test | Acceptance test |
| --- | --- | --- |
| Proves | logic in Go: conversions, ID parsing, the assignment module, a scope adapter's requests | a resource or data source works end to end with Terraform and a real Lakekeeper |
| Build tag | none | `//go:build acceptance` on the first line |
| Needs | only Go | the local stack (`make testacc-up`) |
| Run by | `make test`, and also `make testacc` | `make testacc` only |
| File name | `<subject>_unit_test.go` in `internal/provider`; `<subject>_test.go` in `warehouse/` and `api/` | `resource_<name>_test.go` or `datasource_<name>_test.go` |
| Copy from | [`assignment_unit_test.go`](../internal/provider/assignment_unit_test.go), [`warehouse/refresh_test.go`](../internal/provider/warehouse/refresh_test.go) | [`resource_role_test.go`](../internal/provider/resource_role_test.go), [`datasource_project_test.go`](../internal/provider/datasource_project_test.go) |

Any change to a resource's or data source's behaviour needs an acceptance test. Add unit tests for logic you can check without a server: they are faster and cover error paths a real server rarely produces.

## Steps

Below, `<name>` is the resource type name without the `lakekeeper_` prefix, for example `role`.

1. **Tag the file.** An acceptance test file starts with `//go:build acceptance`, then a blank line, then `package provider`. A unit test file has no build tag. See [Pitfalls](#pitfalls) for what goes wrong otherwise.

2. **Write an acceptance test** in `internal/provider/resource_<name>_test.go`. Copy from [`resource_role_test.go`](../internal/provider/resource_role_test.go).
   - Name it `TestAccLakekeeper<Name>_<case>` (`TestAccDataLakekeeper<Name>_<case>` for a data source), with `_basic` for the main lifecycle.
   - Run it with `resource.ParallelTest`, pass `ProtoV6ProviderFactories: testAccProtoV6ProviderFactories` (from [`provider_test.go`](../internal/provider/provider_test.go)), and set a `CheckDestroy` function that asks the server whether the object is gone, like `testAccCheckLakekeeperRoleDestroy`.
   - Each step has a `Config` and a `Check` built from `resource.ComposeAggregateTestCheckFunc`. Use `acctest.RandString` for names, so parallel tests don't collide.

3. **Create parent objects with the shared helpers.** [`testutil/helpers.go`](../internal/provider/testutil/helpers.go) creates objects directly through the **API client layer**, outside Terraform, and removes them with `t.Cleanup`:
   - `testutil.CreateProject(t)`, `testutil.CreateWarehouse(t, projectID, keyPrefix)`, `testutil.CreateRole(t, projectID)` and `testutil.CreateUser(t, id)`.
   - `testutil.TestLakekeeperClient` is an `*api.Client` for checks your `CheckDestroy` or `Check` functions make against the server.
   - `testutil.DefaultUserID` is the ID of the user the tests log in as.

   [`resource_warehouse_user_assignment_test.go`](../internal/provider/resource_warehouse_user_assignment_test.go) uses several of them. If you need another kind of parent object in more than one test, add a `Create<Object>` helper to `helpers.go` in the same style rather than creating it inline.

4. **Add the import step.** Every importable resource's acceptance test checks import after each step that changes the resource:

   ```go
   {
   	ResourceName:      "lakekeeper_role.foo",
   	ImportState:       true,
   	ImportStateVerify: true,
   },
   ```

   The test imports the resource by its `id`, the [internal ID](add-resource.md#internal-id), and checks that every attribute matches the state Terraform holds. List attributes the server never returns, such as a warehouse's storage credential, in `ImportStateVerifyIgnore`; see [`resource_warehouse_test.go`](../internal/provider/resource_warehouse_test.go). Data sources have no import step.

5. **Test assignment logic with the in-memory scope adapter.** The assignment module's unit tests replace the **scope adapter** with `memoryScopeAdapter` from [`assignment_memory_adapter_test.go`](../internal/provider/assignment_memory_adapter_test.go). It holds the **direct grants** at one scope in memory and, like Lakekeeper, rejects a duplicate grant (409) and a revoke of a missing grant (404).
   - Build it with `newMemoryScopeAdapter(grants...)`, and the resource with `newTestAssignmentResourceFor(descriptor, m)` from [`assignment_helpers_unit_test.go`](../internal/provider/assignment_helpers_unit_test.go). That file also has `testCreateFor`, `testUpdateFor`, `assertStateFor` and `assertOtherGrantsUntouched`.
   - Set `m.listErr` or `m.applyErr` to make every read or write fail. Check `m.applies` for the requests the module sent, and `m.grantsOf(kind, id)` for what the "server" holds afterwards.
   - Copy from [`assignment_warehouse_unit_test.go`](../internal/provider/assignment_warehouse_unit_test.go), which runs every warehouse **assignment descriptor** through schema, lifecycle and import.

   A scope adapter itself is tested against an `httptest` server instead, because its job is the HTTP request: copy `testAdapterServer` from [`assignment_adapter_project_unit_test.go`](../internal/provider/assignment_adapter_project_unit_test.go).

6. **Run just your test.** Unit tests need nothing else:

   ```sh
   RUN=TestWarehouseAssignmentLifecycle make test
   ```

   For an acceptance test, start the local stack once, run your test, and tear the stack down when you are done:

   ```sh
   make testacc-up                          # start the local stack and wait until it is healthy
   RUN='TestAccLakekeeperRole_' make testacc
   make testacc-down                        # stop it and remove its volumes
   ```

   `RUN` is passed to `go test -run`, so it is a regular expression. The stack keeps running between test runs; `make testacc-down` also deletes everything the tests left behind.

## Pitfalls

- **A missing build tag breaks `make test`.** An acceptance test without `//go:build acceptance` is compiled by `make test`. It fails to build, because `testAccProtoV6ProviderFactories` (in [`provider_test.go`](../internal/provider/provider_test.go)) and the `testutil` package exist only under that tag.
- **A wrong build tag hides a unit test.** A unit test with `//go:build acceptance` compiles fine, but `make test` and CI's unit-test job skip it; it runs only in `make testacc`, against the stack. For the same reason a unit test cannot use `testutil`.
- **`make testacc` panics without the stack.** `testutil` connects to Lakekeeper in its `init()` function and panics with `failed to create test client` when the stack isn't running. Run `make testacc-up` first.
- **Run acceptance tests through `make testacc`.** It sets `TF_ACC=1` and the connection settings for the local stack. A plain `go test -tags acceptance` has neither: `resource.Test` skips every test without `TF_ACC`, and `testutil` panics without the connection settings.
- **`RUN` matches prefixes.** `RUN=TestAccLakekeeperRole` also runs `TestAccLakekeeperRoleRoleAssignment_basic` and `TestAccLakekeeperRoleUserAssignment_basic`. End the pattern with `_`, or anchor it with `$`, to run only your resource's tests.
- **`make testacc` also runs every unit test.** Untagged files compile under any tag. A unit test failure therefore also fails the acceptance run.
- **An import step without `ImportStateVerify` checks nothing.** `ImportState: true` alone only checks that import succeeds. Keep `ImportStateVerify: true`, and add to `ImportStateVerifyIgnore` only attributes the server truly never returns.
- **The memory adapter is not Lakekeeper.** It checks the module's logic, not that a request is accepted by a real server. A new assignment resource still needs an acceptance test.

## Before you open the PR

- [ ] `make reviewable` passes, and you committed what it changed.
- [ ] Your acceptance tests pass against the local stack: `make testacc-up`, then `RUN='TestAccLakekeeper<Name>_' make testacc`.
- [ ] Every new acceptance test file starts with `//go:build acceptance`, and every new unit test file has no build tag.
- [ ] If you changed a file listed in the architecture README's [drift table](../architecture/README.md#drift-table) or in the [guides' source table](README.md#guides-and-their-sources), you updated that diagram or guide.
- [ ] `make lint` passes.
