# Contributing to Adaptive Search

Thanks for taking an interest. This is a **research prototype**, so the most
valuable contributions are usually not features — they are measurements,
counter-evidence, and corrections.

## What is most useful

In rough priority order:

1. **The missing tiering benchmark.** The README states plainly that this project
   never measured whether adaptive tier selection actually beats a static choice.
   A reproducible benchmark comparing tiers at a given corpus size would be the
   single most valuable addition to the repository.
2. **Evidence that a stated conclusion is wrong.** If the hysteresis still
   oscillates, or the stability score behaves differently than described, please
   show it. Correcting the write-up matters as much as correcting the code.
3. **Filling a documented gap** — see "What this is not" in the README. The
   `performBTreeBuild` stub, the unwired B-Tree index, and the per-process
   hysteresis counters are all known and all fair game.
4. **Reproducibility fixes** — anything that makes the prototype easier to run
   and verify, including the gaps in `config/config.yaml.example`.
5. Bug fixes and documentation improvements.

If you are proposing a substantial change, please open an issue first so we can
agree on the approach before you spend time on it.

## Licence and sign-off

Contributions are accepted under the [MIT licence](LICENSE), the same licence as
the project.

This project uses a **Developer Certificate of Origin (DCO)**, not a CLA. You
keep copyright in your contribution; you simply certify that you have the right
to submit it. Read the full text at <https://developercertificate.org/>.

Certify by signing off every commit:

```bash
git commit -s -m "your message"
```

That appends a trailer using your configured git identity:

```
Signed-off-by: Your Name <you@example.com>
```

Use a real name and a working email address. If you forget on your last commit:

```bash
git commit --amend -s --no-edit
```

For a whole branch:

```bash
git rebase --signoff main
```

## Development setup

You need Go 1.25 (the version is pinned in `search-system/go.mod`), Node.js 20
for the dashboard, and Docker for the local stack.

```bash
git clone https://github.com/iampopye/IMI_Hackathon.git
cd IMI_Hackathon/search-system

cp config/config.yaml.example config/config.yaml
# replace the CHANGE_ME placeholders with searchuser / searchpass

docker compose up -d postgres redis   # minimum viable stack
go run ./cmd/server
```

Only PostgreSQL is strictly required — Redis, Elasticsearch and Kafka are each
optional and the service degrades gracefully without them. That makes for a much
lighter dev loop than the full Compose stack.

## Before you open a pull request

CI runs exactly these, so run them locally first:

```bash
cd search-system

gofmt -l .          # must print nothing
go build ./...
go vet ./...
go test ./... -race
```

For the dashboard:

```bash
cd search-ui
npm ci
npm run build
```

### Running the database-backed tests

`go test ./...` **silently skips** every integration test unless
`TEST_POSTGRES_DSN` is set. A green local run therefore does not mean they
passed. If you touch anything in `internal/db`, `internal/upsert`,
`internal/outbox` or `internal/dataset`, run them properly:

```bash
docker compose up -d postgres
docker compose exec postgres createdb -U searchuser searchdb_test

TEST_POSTGRES_DSN="host=localhost port=5432 user=searchuser password=searchpass dbname=searchdb_test sslmode=disable" \
  go test ./internal/db/... ./internal/integration/... -race -v
```

CI runs these in a dedicated job against a PostgreSQL 16 service container.

### Chaos tests

These need no database and are meaningless without the race detector:

```bash
go test ./... -run Chaos -race
```

## Conventions

- **Formatting is enforced.** `gofmt -l .` must be empty; CI fails otherwise.
  Note that `gofmt` sorts imports within each contiguous block — keep
  `github.com/iampopye/IMI_Hackathon/...` imports in correct alphabetical order
  alongside third-party ones.
- **Follow the existing structure.** Adaptation logic belongs in
  `internal/dataset`; query routing in `internal/search`; anything crossing a
  process boundary goes through `internal/outbox` or `internal/pipeline`.
- **Instrument new behaviour.** If you add a mechanism that can fire at runtime,
  add a Prometheus metric for it in `internal/metrics`. Being able to observe the
  adaptation is the point of the project.
- **Keep external dependencies optional.** Redis, Elasticsearch and Kafka are all
  individually optional today. Do not introduce a hard dependency on any of them
  without discussion.
- **Migrations are append-only.** Add `internal/db/migrations/00N_name.sql` with
  the next number; never edit an applied migration. They are embedded via
  `go:embed` and run automatically at startup.
- Commit messages: a concise imperative subject line. Conventional-commit
  prefixes (`feat:`, `fix:`, `docs:`, `chore:`) are welcome but not required.

## Claims and evidence

Because this repository is a research artifact, its credibility depends on not
overclaiming. If your change adds or modifies a factual statement in the README
or on the landing page:

- State how it was measured, and on what hardware or corpus.
- If it is a design choice rather than a measured result, say so — the tier
  thresholds are labelled as design parameters for exactly this reason.
- Do not quote performance numbers that cannot be reproduced from what is in the
  repository.

## Reporting bugs and requesting features

Use the [issue templates](https://github.com/iampopye/IMI_Hackathon/issues/new/choose).
For anything security-related, do **not** open a public issue — follow
[SECURITY.md](SECURITY.md).

## Code of conduct

Be decent to people. Assume good faith, critique the work rather than the person,
and keep disagreement technical. Behaviour that makes the project worse to
participate in is not welcome regardless of the quality of the patch.
