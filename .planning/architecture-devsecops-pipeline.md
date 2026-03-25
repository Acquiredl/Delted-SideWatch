# Architecture: DevSecOps Security Scanning Pipeline + Alertmanager Webhook

> Source: Previous session DevSecOps audit handoff
> Date: 2026-03-25
> Mode: Feature (existing codebase)

## File Tree

```
+ .github/dependabot.yml                        ← Go module + npm dependency updates
+ .github/workflows/security.yml                ← security scanning CI workflow
+ .github/CODEOWNERS                             ← code review ownership rules
+ .gitleaks.toml                                 ← secret detection config
~ .golangci.yml                                  ← add gosec, gocritic linters
~ Makefile                                       ← lint target → golangci-lint, add security targets
~ .github/workflows/deploy.yml                   ← add security job gate before deploy
~ infra/scripts/deploy.sh                        ← post-deploy smoke tests + Discord webhook
~ services/manager/cmd/manager/routes.go         ← add POST /api/webhook/alerts handler
~ services/manager/cmd/manager/main.go           ← wire alert handler
~ services/manager/internal/metrics/metrics.go   ← add alert-received counter
```

## Component Breakdown

### Feature: Static Analysis + Security Linting
- Files: `.golangci.yml`, `Makefile`
- Dependencies: none
- Complexity: **low** — config-only, no Go code

### Feature: Dependency Scanning
- Files: `.github/dependabot.yml`
- Dependencies: none
- Complexity: **low** — single YAML file

### Feature: CI Security Workflow
- Files: `.github/workflows/security.yml`, modified `.github/workflows/deploy.yml`
- Dependencies: `.golangci.yml` (must have gosec enabled first)
- Complexity: **medium** — multi-job workflow with golangci-lint, govulncheck, npm audit, Trivy, gitleaks

### Feature: Secret Detection
- Files: `.gitleaks.toml`
- Dependencies: none
- Complexity: **low** — config file with allowlist for false positives

### Feature: Deploy Smoke Tests + Notifications
- Files: modified `infra/scripts/deploy.sh`
- Dependencies: running services (existing)
- Complexity: **low** — shell additions to existing script

### Feature: Code Ownership + Branch Protection
- Files: `.github/CODEOWNERS`
- Dependencies: none (branch protection is GitHub settings, not code)
- Complexity: **low** — single file

### Feature: Alertmanager Webhook Handler
- Files: modified `routes.go`, modified `main.go`, modified `metrics.go`
- Dependencies: existing route registration pattern
- Complexity: **medium** — must parse Alertmanager's webhook payload, log structured alerts, emit Prometheus counter, admin-token auth

## Data Model

No new database tables. The alertmanager webhook handler is stateless — it receives alert payloads, logs them via slog, and increments a Prometheus counter. No persistence beyond what Alertmanager itself handles.

## Key Decisions

### Alertmanager Webhook: Log + Metric (no database)
- **Chosen**: Handler receives Alertmanager's POST payload, logs each alert at appropriate level (slog.Warn for firing, slog.Info for resolved), increments `p2pool_alerts_received_total` counter with labels `{alertname, status}`. No database writes. Admin-token auth (matching existing `handleBackfillPrices` pattern).
  - Alertmanager already handles retry/escalation — the webhook just needs to acknowledge
  - Structured slog output flows into Loki via existing promtail config → searchable
  - Prometheus counter enables "alerts per hour" dashboards and meta-alerting
  - Zero new dependencies, zero schema changes
- **Rejected**: Store alerts in Postgres — adds table, migration, cleanup job. Alertmanager + Loki already provide full alert history. DB storage would be redundant.
- **Rejected**: Forward to Discord directly from manager — Alertmanager already supports Discord webhooks natively (it's in alertmanager.yml commented out). Don't rebuild what Alertmanager does.

### CI Security Tool: golangci-lint (not standalone gosec)
- **Chosen**: Run `gosec` via `golangci-lint` — it's already the project's lint runner (referenced in `.golangci.yml`). Adding `gosec` and `gocritic` as additional enabled linters is one line each. The CI workflow runs `golangci-lint run` which catches everything.
  - Single tool, single cache, single config file
  - golangci-lint handles version pinning and caching in CI natively via `golangci/golangci-lint-action`
- **Rejected**: Standalone `gosec` binary — separate install, separate config, separate CI step. Redundant with golangci-lint's gosec integration.

### Trivy Scan Target: Built Docker images (not filesystem)
- **Chosen**: Scan the actual Docker images after `docker compose build`. This catches both application vulnerabilities and base image (Alpine) CVEs in one pass.
  - Matches what actually runs in production
  - Catches Alpine package vulnerabilities that filesystem scans miss
- **Rejected**: Filesystem-only scan — misses base image CVEs, container config issues.

### Gitleaks: Baseline mode (not block-all-history)
- **Chosen**: Run gitleaks in `--baseline-path` mode against the diff only (new commits). Existing repo history may have false positives from test fixtures, `.env.example`, etc. Blocking on full-history scan would require extensive allowlisting before the workflow can go green.
- **Rejected**: Full-history scan — would require auditing every commit, likely many false positives from example configs and test data.

## Build Phases

### Phase 0: Baseline
- **Goal**: Verify existing CI and codebase compile before making changes
- **Files**: none (read-only)
- **Dependencies**: none
- **End Conditions**:
  - [ ] `cd services/manager && go build ./cmd/manager/` compiles
  - [ ] `cd services/gateway && go build ./cmd/gateway/` compiles
  - [ ] `cd services/manager && go vet ./...` passes
  - [ ] `cd services/gateway && go vet ./...` passes
  - [ ] Existing tests pass

### Phase 1: Linting + Makefile Hardening
- **Goal**: Add gosec/gocritic to golangci-lint config, update Makefile lint target to run golangci-lint instead of bare go vet, add convenience security targets
- **Files**:
  - `~ .golangci.yml` — add `gosec`, `gocritic` to enabled linters; add `gosec` settings section
  - `~ Makefile` — change `lint` to run `golangci-lint run`, add `security` target for local govulncheck + npm audit
- **Dependencies**: Phase 0 (baseline verified)
- **End Conditions**:
  - [ ] `.golangci.yml` includes `gosec` and `gocritic` in enabled linters
  - [ ] `make lint` invokes `golangci-lint run` (not bare `go vet`)
  - [ ] `make lint` succeeds on the current codebase (no new findings that block CI)
  - [ ] If gosec reports real issues, fix them in this phase
  - [ ] Existing tests still pass
  - [ ] No new typecheck errors

### Phase 2: CI Security Workflow + Gitleaks + Dependabot
- **Goal**: Create the security scanning GitHub Actions workflow, gitleaks config, and Dependabot config
- **Files**:
  - `+ .github/workflows/security.yml` — jobs: golangci-lint, govulncheck, npm-audit, trivy, gitleaks
  - `+ .gitleaks.toml` — baseline config with allowlists for .env.example, test fixtures
  - `+ .github/dependabot.yml` — Go modules (services/manager, services/gateway) + npm (frontend)
  - `+ .github/CODEOWNERS` — assign reviewers for security-sensitive paths
  - `~ .github/workflows/deploy.yml` — add `needs: security` gate (security workflow must pass before deploy)
- **Dependencies**: Phase 1 (golangci-lint config must be correct first)
- **End Conditions**:
  - [ ] `security.yml` workflow is valid YAML (parseable by `yq` or GitHub Actions linter)
  - [ ] Workflow triggers on push to main + all PRs
  - [ ] golangci-lint job uses `golangci/golangci-lint-action` with the repo's `.golangci.yml`
  - [ ] govulncheck job runs `govulncheck ./...` for both manager and gateway modules
  - [ ] npm-audit job runs `npm audit --audit-level=high` in frontend/
  - [ ] Trivy job scans at least the manager and gateway Docker images
  - [ ] gitleaks job runs in baseline mode (diff-only, not full history)
  - [ ] `.gitleaks.toml` allowlists `.env.example` and test fixture paths
  - [ ] `dependabot.yml` covers 3 ecosystems: gomod (manager), gomod (gateway), npm (frontend)
  - [ ] `CODEOWNERS` assigns review ownership for `.github/`, `infra/`, `services/`
  - [ ] `deploy.yml` gates on security workflow passing
  - [ ] No regressions in existing CI

### Phase 3: Deploy Enhancements
- **Goal**: Add post-deploy smoke tests and optional Discord deploy notifications to deploy.sh
- **Files**:
  - `~ infra/scripts/deploy.sh` — add `smoke_test()` function (validates `/api/pool/stats` returns JSON), add optional Discord webhook notification on success/failure
- **Dependencies**: Phase 0 (existing deploy.sh must be understood)
- **End Conditions**:
  - [ ] `smoke_test()` function calls `GET /api/pool/stats` and validates response contains `"total_hashrate"`
  - [ ] `smoke_test()` function calls `GET /health` on both gateway and manager and checks for `"status":"ok"`
  - [ ] Smoke test failure triggers rollback (same as existing healthcheck failure)
  - [ ] Discord notification is opt-in via `DISCORD_WEBHOOK_URL` env var (no-op if unset)
  - [ ] Notification includes: commit hash, branch, deploy duration, success/failure status
  - [ ] `deploy.sh` still works correctly without Discord webhook configured
  - [ ] `bash -n infra/scripts/deploy.sh` passes (no syntax errors)

### Phase 4: Alertmanager Webhook Handler
- **Goal**: Implement the missing POST /api/webhook/alerts endpoint that Alertmanager is configured to call, wired into the manager's existing routing
- **Files**:
  - `~ services/manager/internal/metrics/metrics.go` — add `AlertsReceived` counter vec with labels `{alertname, status}`
  - `~ services/manager/cmd/manager/routes.go` — add `handleAlertWebhook()` handler
  - `~ services/manager/cmd/manager/main.go` — no change needed (route already uses mux, adminToken already wired)
- **Dependencies**: Phase 0 (baseline)
- **End Conditions**:
  - [ ] `POST /api/webhook/alerts` accepts Alertmanager v4 webhook payload
  - [ ] Handler validates admin token via `X-Admin-Token` header (same pattern as `handleBackfillPrices`)
  - [ ] Each alert in the payload is logged via slog: alertname, status (firing/resolved), severity, summary
  - [ ] `p2pool_alerts_received_total{alertname, status}` Prometheus counter incremented per alert
  - [ ] Returns 200 on success (Alertmanager expects this)
  - [ ] Returns 403 on invalid/missing admin token
  - [ ] Returns 400 on unparseable JSON body
  - [ ] `cd services/manager && go build ./cmd/manager/` compiles
  - [ ] `cd services/manager && go test ./...` passes
  - [ ] No regressions in existing functionality

## Phase Dependency Graph

```
Phase 0 (Baseline) → Phase 1 (Linting)
                          → Phase 2 (CI + Gitleaks + Dependabot)
Phase 0 (Baseline) → Phase 3 (Deploy Enhancements)
Phase 0 (Baseline) → Phase 4 (Alertmanager Webhook)
```

- Phase 1 → Phase 2 is sequential (CI workflow depends on corrected golangci-lint config)
- Phases 3 and 4 are **independent** of each other and of Phases 1-2
- Maximum parallelism: Phase 1 + Phase 3 + Phase 4 can start simultaneously after Phase 0

## Risk Register

1. **gosec false positives blocking CI**: gosec may flag existing code (e.g., `math/rand` usage in non-crypto contexts, hardcoded test values). **Mitigation**: Run gosec locally in Phase 1 and fix real issues or add `//nolint:gosec` with justification comment. The CI workflow should not be green-path-blocked by pre-existing issues.

2. **Trivy scan latency slowing PRs**: Docker image scanning can take 2-5 minutes per image. With manager + gateway images, that's up to 10 minutes added to PR checks. **Mitigation**: Run Trivy in a parallel job (not blocking other checks). Cache Trivy DB between runs. Only fail on HIGH/CRITICAL severity — don't block on LOW/MEDIUM.

3. **Alertmanager webhook auth mismatch**: The existing `alertmanager.yml` calls `http://manager:8081/api/webhook/alerts` with no auth headers. The handler requires `X-Admin-Token`. **Mitigation**: Either (a) add `http_config.headers` to alertmanager.yml to send the token, or (b) make the webhook handler skip auth when the request comes from the Docker internal network. Option (a) is cleaner — document the required config in alertmanager.yml.

4. **Deploy.sh Discord webhook leaking sensitive info**: Deploy notifications might include commit messages that reference security issues or internal details. **Mitigation**: Notification includes only commit hash, branch name, duration, and status. No commit messages, no diffs, no env var values.

## Deployment Strategy

- **Platform**: Same Docker Compose stack, same VPS
- **No new services**: All changes are config files, CI workflows, and minor Go handler additions
- **Migration**: None — no new database tables
- **Rollback**: Revert the commit. No state to clean up.
- **Pre-deploy**: After merging, the new `security.yml` workflow gates deploy. First deploy after merge will be the first time the full security pipeline runs in CI.
- **Post-deploy verification**: The enhanced `deploy.sh` smoke tests will validate the new alertmanager webhook endpoint is responding.
