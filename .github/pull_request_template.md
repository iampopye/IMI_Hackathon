<!--
Thanks for contributing to Adaptive Search.

This is a research prototype, so accuracy of claims matters as much as
correctness of code. Please fill in what applies and delete what does not.
-->

## What this changes

<!-- A short description. If it closes an issue, write "Closes #123". -->

## Why

<!--
What problem, gap, or open question does this address? If it relates to something
the README flags as untested or incomplete, link to that section.
-->

## Type of change

- [ ] Bug fix
- [ ] New capability
- [ ] Experiment, benchmark, or measurement
- [ ] Evidence correcting a documented conclusion
- [ ] Documentation
- [ ] Build, CI, or tooling
- [ ] Breaking change

## How this was verified

<!--
Be specific. "Tests pass" is not enough if the DB-backed tests skipped.
-->

- [ ] `gofmt -l .` prints nothing
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test ./... -race` passes
- [ ] Database-backed tests ran **with `TEST_POSTGRES_DSN` set** (they skip silently otherwise)
- [ ] Chaos tests run under `-race` (`go test ./... -run Chaos -race`)
- [ ] `search-ui` builds (`npm ci && npm run build`) — if the dashboard changed
- [ ] Not applicable

<!-- If you measured something, paste the numbers and say what hardware produced them. -->

## Claims and evidence

<!-- Only if this PR adds or changes a factual statement in the README or docs/. -->

- [ ] Any performance number I quote is reproducible from what is in this repository
- [ ] I have distinguished measured results from design choices
- [ ] This PR makes no factual claims

## Project constraints

- [ ] Redis, Elasticsearch and Kafka remain individually optional (PostgreSQL stays the only hard dependency)
- [ ] No already-applied migration was edited (migrations are append-only)
- [ ] New runtime behaviour is instrumented with a Prometheus metric in `internal/metrics`
- [ ] Not applicable

## Developer Certificate of Origin

- [ ] **Every commit is signed off** (`git commit -s`) per [CONTRIBUTING.md](../CONTRIBUTING.md)
- [ ] I agree my contribution is licensed under the [MIT licence](../LICENSE)

<!--
Missing the sign-off? Fix the last commit with:
    git commit --amend -s --no-edit
Or a whole branch with:
    git rebase --signoff main
-->
