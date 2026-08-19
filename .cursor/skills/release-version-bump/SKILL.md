---
name: release-version-bump
description: >-
  End-to-end release bump for vault-plugin-secrets-artifactory: update Go version
  and Go module dependencies, run local build/tests against the existing Artifactory
  image, then create a PR only after asking permission. Use when the user asks to
  bump Go, refresh go.mod dependencies, prepare a plugin release, or ship a version
  bump PR. Do not bump the Artifactory image version. Ask the user only twice:
  before starting Artifactory/integration tests, and before commit/push/PR.
---

# Release Version Bump (Vault Artifactory Secrets Plugin)

Complete workflow for this repo only. Follow phases in order. Track progress with the checklist.

```
Progress:
- [ ] 1. Detect current versions + target versions
- [ ] 2. Update Go / dependencies / CHANGELOG / CI
- [ ] 3. Build + unit tests
- [ ] 4. ASK → then local integration smoke tests vs Artifactory
- [ ] 5. Report go/no-go
- [ ] 6. ASK → then commit / branch / PR (only if approved)
```

## User checkpoints (ask only these)

Ask the user **only** at these two points. Do **not** pause for confirmation on version targets, file edits, dependency upgrades, CHANGELOG text, unit tests, or intermediate choices.

1. **Before Phase 4** — after unit tests/build succeed (or if there is nothing to change but user still wants smoke tests), ask:

   > Unit tests/build are done. OK to start Artifactory and run local integration smoke tests?

   Proceed with Phase 4 only if the user says yes. If they say no, skip to Phase 5 with integration checks marked SKIPPED and still report a verdict.

2. **Before Phase 6** — after the go/no-go verdict, ask:

   > Tests are complete (GO/NO-GO). Do you want me to create a branch, commit, push, and open a PR to `master`?

   Proceed with commit/push/PR only if the user says yes.

Between checkpoints: run autonomously, choose sensible defaults, briefly report progress, and keep going.

## Hard rules

1. **Do not open a PR, push, or create a remote branch until the user explicitly approves** at checkpoint 2.
2. **Do not start Artifactory or run integration smoke tests until the user explicitly approves** at checkpoint 1.
3. Never commit temporary helpers (e.g. `scripts/get-token-inside.sh`), credentials, tokens, or `.env` files.
4. Prefer one focused commit on a new branch from latest `origin/master`.
5. Commit subject must match org style: `INST-XXXXX - Short-description` (derive ticket from branch/user input).
6. Do not force-push unless the user explicitly asks.
7. **Do not update the Artifactory image version** (`scripts/Dockerfile` stays unchanged unless the user explicitly asks outside this skill).

## Phase 1 — Detect versions

Do **not** ask the user to confirm targets. Pick latest Go + available dep upgrades automatically; note Artifactory tag for smoke tests only.

1. Read current state:
   - `go.mod` → `go` directive
   - `scripts/Dockerfile` → note current Artifactory image tag for smoke tests only (do not change it)
   - `.github/workflows/acceptance-tests.yml` and `release.yml` → `go-version`
   - `CHANGELOG.md` top entry
2. Resolve targets:
   - **Go**: latest stable (`go version` locally, or https://go.dev/dl/). Prefer full patch in `go.mod` (e.g. `1.26.6`); CI may use minor (`1.26`).
   - **Artifactory**: leave as-is in `scripts/Dockerfile`. Use that existing tag for local smoke tests.
   - **Deps**: update after Go bump (skill `go get` list).
3. If already on latest Go and no dep updates exist, skip Phase 2 edits, still run Phase 3, then hit checkpoint 1 for optional smoke tests.

## Phase 2 — Make version / dependency changes

Do **not** ask before editing. Apply changes immediately when targets differ from current.

Update these files as needed:

| File | Change |
|------|--------|
| `go.mod` | `go X.Y.Z` |
| `.github/workflows/acceptance-tests.yml` | `go-version: X.Y` |
| `.github/workflows/release.yml` | `go-version: X.Y` |
| `CHANGELOG.md` | New/top NOTES for this release |

**Do not edit** `scripts/Dockerfile` (Artifactory image tag).

Dependency update (after Go bump):

```bash
# Direct deps
go get github.com/golang-jwt/jwt/v4@latest \
  github.com/hashicorp/go-hclog@latest \
  github.com/hashicorp/go-version@latest \
  github.com/hashicorp/vault/api@latest \
  github.com/hashicorp/vault/sdk@latest \
  github.com/jarcoal/httpmock@latest \
  github.com/samber/lo@latest \
  github.com/stretchr/testify@latest

# Safe common upgrades (avoid `go get -u all` — breaks on armón/go-metrics rename)
go get golang.org/x/crypto@latest golang.org/x/net@latest golang.org/x/sys@latest \
  golang.org/x/text@latest golang.org/x/sync@latest golang.org/x/oauth2@latest \
  google.golang.org/grpc@latest google.golang.org/protobuf@latest \
  github.com/hashicorp/go-plugin@latest github.com/hashicorp/go-metrics@latest \
  github.com/hashicorp/go-kms-wrapping/v2@latest \
  go.opentelemetry.io/otel@latest \
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@latest

go mod tidy
```

CHANGELOG NOTES example:

```markdown
## X.Y.Z (Month Day, Year)

NOTES:

* Update Go minimum version to A.B.C.
* Update Go module dependencies to latest available versions.
```

## Phase 3 — Build and unit tests

Do **not** ask. Run immediately after Phase 2 (or Phase 1 if no edits).

```bash
export PATH="$HOME/bin:$PATH"   # if vault CLI lives in ~/bin
rm -rf dist
go test -count=1 -timeout 5m ./...
make build
go version -m dist/*/artifactory-secrets-plugin | head -3
```

Stop and fix failures before asking checkpoint 1. After success → **checkpoint 1** (Artifactory / smoke tests).

## Phase 4 — Local integration smoke tests

**Requires checkpoint 1 approval.** Do not start Docker Artifactory or Vault integration flows before that.

Prereqs: Docker, Vault CLI, GoReleaser.

Use the **existing** Artifactory image from `scripts/Dockerfile` (do not bump it).

Tear down leftovers first:

```bash
pkill -f 'vault server -dev' 2>/dev/null || true
make stop_artifactory 2>/dev/null || true
docker ps --format '{{.ID}} {{.Image}} {{.Ports}}' | awk '/artifactory|:8082->/ {print $1}' | xargs -r docker stop
rm -f vault/artifactory.env
```

Then:

1. Start Artifactory: `make artifactory` (waits until ping OK; Apple Silicon uses OSS via `run-artifactory-container.sh`).
2. Start Vault in background with plugin dir from `make build` (`dist/vault-plugin-secrets-artifactory_<os>_<arch>_*/`).
3. Register + enable plugin (`make setup` or explicit `vault plugin register` / `secrets enable` with binary-reported version).
4. Configure admin token and run:
   - `make admin` **or** equivalent write/read `config/admin` + rotate
   - `make usertoken` / `user_token/<existing-user>`
   - `make testrole` / `roles/test` + `token/test`

### Known gotchas (do not mis-report as product bugs)

1. **Host UI login to Artifactory may hang** on localhost; if `scripts/getArtifactoryAdminToken.sh` times out, obtain an Access token via UI login **inside** the Artifactory container (temporary script OK locally — **never commit it**).
2. **`user_token/<username>` requires a real Artifactory user**. `user_token/test` fails if user `test` does not exist; use `admin` or create the user. Role path `token/test` creates a transient `v-test-*` user and does not need a pre-existing user.
3. Do not treat empty/wrong-audience legacy tokens as plugin failures; plugin Access APIs need Access-audience JWTs.

Detailed commands: [testing.md](testing.md).

## Phase 5 — Verdict

Do **not** ask. Report immediately after Phase 4 (or after skip).

| Check | Result |
|-------|--------|
| Unit tests | PASS/FAIL |
| Build / Go toolchain in binary | PASS/FAIL + version |
| Plugin register/enable | PASS/FAIL / SKIPPED |
| config/admin + rotate | PASS/FAIL / SKIPPED |
| roles/test + token/test | PASS/FAIL / SKIPPED |
| user_token (existing user) | PASS/FAIL / SKIPPED |

End with **GO** or **NO-GO** and short rationale. Then → **checkpoint 2** (PR).

## Phase 6 — PR (permission required)

**Requires checkpoint 2 approval.**

Only if the user says yes:

1. `git fetch origin master`
2. Create branch: `INST-XXXXX-<short-slug>` from `origin/master` (or user-provided name)
3. Stage only intended files (`go.mod`, `go.sum`, workflows, `CHANGELOG.md`) — **not** `scripts/Dockerfile`
4. Commit: `INST-XXXXX - Update go version and dependencies` (adjust to match changes)
5. `git push -u origin HEAD`
6. `gh pr create --base master` with Summary + Test plan
7. Return the PR URL

If user declines, leave changes local and summarize next commands.

## Do not

- Ask for confirmation except at the two user checkpoints above
- Update `scripts/Dockerfile` / Artifactory image version as part of this skill
- Commit `scripts/get-token-inside.sh` or similar ad-hoc scripts
- Run `go get -u all` as the primary upgrade path
- Call OSS-only UI failures or missing-user `user_token/test` a dependency regression without rechecking with an existing user
- Start Artifactory/integration tests or push/PR without the matching checkpoint approval
