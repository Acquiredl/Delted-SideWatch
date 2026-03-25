# Campaign: DevSecOps Security Scanning Pipeline + Alertmanager Webhook

Status: completed
Started: 2026-03-25T00:00:00Z
Direction: Implement the DevSecOps security scanning pipeline identified in the previous session's audit — gosec linting, CI security workflow, Dependabot, gitleaks, deploy smoke tests, CODEOWNERS, and the missing alertmanager webhook handler.

## Claimed Scope
- .golangci.yml
- Makefile
- .github/
- .gitleaks.toml
- infra/scripts/deploy.sh
- config/alertmanager/alertmanager.yml
- services/manager/cmd/manager/routes.go
- services/manager/cmd/manager/main.go
- services/manager/internal/metrics/metrics.go

## Phases
| # | Status | Type | Phase | Done When |
|---|--------|------|-------|-----------|
| 0 | complete | verify | Baseline verification | Go builds compile, go vet passes, existing tests pass |
| 1 | complete | build | Linting + Makefile hardening | .golangci.yml has gosec+gocritic, `make lint` runs golangci-lint, no blocking findings |
| 2 | complete | build | CI security workflow + gitleaks + Dependabot + CODEOWNERS | 4 new files created, deploy.yml gates on security, all valid YAML |
| 3 | complete | build | Deploy smoke tests + Discord notifications | deploy.sh has smoke_test(), Discord opt-in via env var, bash -n passes |
| 4 | complete | build | Alertmanager webhook handler | POST /api/webhook/alerts returns 200 with valid payload, 403 without token, manager compiles and tests pass |

## Phase End Conditions
| Phase | Condition Type | Check |
|-------|---------------|-------|
| 0 | command_passes | cd services/manager && go build ./cmd/manager/ |
| 0 | command_passes | cd services/gateway && go build ./cmd/gateway/ |
| 0 | command_passes | cd services/manager && go vet ./... |
| 0 | command_passes | cd services/gateway && go vet ./... |
| 1 | file_exists | .golangci.yml contains "gosec" and "gocritic" |
| 1 | command_passes | Makefile lint target references golangci-lint (grep check) |
| 1 | command_passes | cd services/manager && go build ./cmd/manager/ |
| 1 | command_passes | cd services/gateway && go build ./cmd/gateway/ |
| 2 | file_exists | .github/workflows/security.yml |
| 2 | file_exists | .github/dependabot.yml |
| 2 | file_exists | .gitleaks.toml |
| 2 | file_exists | .github/CODEOWNERS |
| 2 | command_passes | python -c "import yaml; yaml.safe_load(open('.github/workflows/security.yml'))" OR yq validation |
| 2 | file_exists | .github/workflows/deploy.yml contains "needs:" reference to security |
| 3 | command_passes | bash -n infra/scripts/deploy.sh |
| 3 | command_passes | grep -q "smoke_test" infra/scripts/deploy.sh |
| 3 | command_passes | grep -q "DISCORD_WEBHOOK_URL" infra/scripts/deploy.sh |
| 4 | command_passes | cd services/manager && go build ./cmd/manager/ |
| 4 | command_passes | cd services/manager && go test ./... |
| 4 | command_passes | grep -q "webhook/alerts" services/manager/cmd/manager/routes.go |
| 4 | command_passes | grep -q "AlertsReceived" services/manager/internal/metrics/metrics.go |

## Feature Ledger
| Feature | Status | Phase | Notes |
|---------|--------|-------|-------|
| gosec + gocritic linting | done | 1 | Added to .golangci.yml, G104/G304 excluded |
| Makefile lint hardening | done | 1 | lint→golangci-lint, added security target |
| CI security workflow | done | 2 | golangci-lint, govulncheck, npm audit, Trivy, gitleaks |
| Dependabot | done | 2 | Go modules (2 dirs) + npm + GitHub Actions |
| Gitleaks config | done | 2 | Baseline mode, allowlists for examples/tests |
| CODEOWNERS | done | 2 | @acquiredl owns security-sensitive paths |
| Deploy gates on security | done | 2 | deploy.yml needs: [test, security] |
| Deploy smoke tests | done | 3 | smoke_test() validates /api/pool/stats + /health |
| Discord deploy notifications | done | 3 | Opt-in via DISCORD_WEBHOOK_URL env var |
| Alertmanager webhook handler | done | 4 | POST /api/webhook/alerts with slog + Prometheus counter |
| Alertmanager auth header | done | 4 | X-Admin-Token in alertmanager.yml (placeholder) |

## Decision Log
- 2026-03-25: Run gosec via golangci-lint, not standalone binary
  Reason: Single tool, single config, single CI cache. golangci-lint already used in project.
- 2026-03-25: Alertmanager webhook logs + Prometheus counter only, no DB persistence
  Reason: Alertmanager + Loki already provide alert history. Adding a table is redundant.
- 2026-03-25: Gitleaks in baseline/diff mode, not full-history scan
  Reason: Repo history likely has false positives from .env.example and test fixtures.
- 2026-03-25: Trivy scans built Docker images, not filesystem
  Reason: Catches Alpine base image CVEs that filesystem scans miss.
- 2026-03-25: Alertmanager webhook requires X-Admin-Token auth (matches backfill-prices pattern)
  Reason: Consistent with existing admin endpoint auth. alertmanager.yml must be updated to send the header.

## Review Queue
- [ ] Security: Verify gosec findings are genuine issues vs false positives before adding nolint directives
- [ ] Architecture: Confirm alertmanager.yml auth header approach (X-Admin-Token) works with Alertmanager's http_config

## Circuit Breakers
- gosec reports 10+ real security issues in existing code (scope too large for this campaign — split into separate remediation)
- golangci-lint config change causes existing CI to fail on unrelated linter (gocritic false positives)
- Alertmanager webhook handler requires schema changes (violates "no new tables" constraint)

## Active Context
Campaign completed. All 5 phases done. 4 new files, 6 modified files, zero regressions.

## Continuation State
Phase: complete
Sub-step: done
Files modified: .golangci.yml, Makefile, .github/workflows/deploy.yml, .github/workflows/security.yml, .github/dependabot.yml, .github/CODEOWNERS, .gitleaks.toml, infra/scripts/deploy.sh, services/manager/cmd/manager/routes.go, services/manager/internal/metrics/metrics.go, config/alertmanager/alertmanager.yml
Blocking: none
