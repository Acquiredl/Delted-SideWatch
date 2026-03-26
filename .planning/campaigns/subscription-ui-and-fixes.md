# Campaign: Subscription UI + Alertmanager Auth Fix + CI + CLAUDE.md

> Status: completed
> Created: 2026-03-26
> Architecture: .planning/architecture-subscription-ui-and-fixes.md
> Direction: Add subscription frontend UI, fix alertmanager webhook auth, add frontend tests to CI, update CLAUDE.md

## Phases

| # | Name | Type | Status | Dependencies |
|---|------|------|--------|-------------|
| 0 | Baseline | verify | completed | none |
| 1 | Alertmanager Auth Fix | build | completed | 0 |
| 2 | Subscription Types | build | completed | 0 |
| 3 | Subscription Components | build | completed | 2 |
| 4 | Subscribe Page + Nav | build | completed | 3 |
| 5 | Frontend Tests in CI | build | completed | 0 |
| 6 | CLAUDE.md Update | build | completed | 1-5 |

## Feature Ledger

1. `frontend/tsconfig.json` — exclude __tests__/ from tsc (fixes pre-existing jest-dom type errors)
2. `routes.go` — handleAlertWebhook now accepts Authorization: Bearer alongside X-Admin-Token
3. `alertmanager.yml` — http_config.authorization with credentials_file for webhook auth
4. `frontend/lib/api.ts` — SubscriptionStatus, PaymentAddress, SubPayment types + postJSON helper
5. `frontend/components/SubscriptionStatus.tsx` — tier badge, expiry, grace period, free-tier limits
6. `frontend/components/SubscriptionPayment.tsx` — payment subaddress (copyable), suggested amount, payment history table
7. `frontend/app/subscribe/page.tsx` — full subscription management page (address lookup, status, payment, API key generation)
8. `frontend/components/Navigation.tsx` — "Subscribe" nav link added
9. `frontend/app/miner/page.tsx` — subscription tier badge + upgrade CTA for free-tier users
10. `.github/workflows/security.yml` — frontend-tests job (npm test + tsc --noEmit) runs on PRs
11. `CLAUDE.md` — removed stale "tax export" from future work, documented subscription UI, updated test counts

## Decision Log

1. Alertmanager: Accept Authorization: Bearer alongside X-Admin-Token
2. Subscribe page: Standalone /subscribe route
3. CI: Add frontend-tests job to security.yml
4. Miner page: Add tier badge + CTA link to /subscribe

## Review Queue

(empty)
