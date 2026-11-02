---
name: verify
description: Run full local validation (lint, generated docs, unit tests, and acceptance tests for touched resources). Use before marking a task complete or before opening a PR.
---

From the repo root:

1. `make reviewable`: builds, formats, regenerates `docs/`, and runs unit tests. If it modifies files, include those changes in the work.
2. `make lint`: golangci-lint, `terraform fmt` and `shfmt` checks, plus the stale-docs check (which needs a clean git workspace, so run it after committing or report that it was skipped).
3. If resource or data source behaviour changed, run the acceptance tests for each touched type: `make testacc-up` if the stack is not running, then `RUN=TestAcc<Name> make testacc`.

Done when every step passes, or when each failure is reported with its exact output. Fix only trivially obvious failures in code you just touched (e.g. an unused import); confirm any other fix with the user first.
