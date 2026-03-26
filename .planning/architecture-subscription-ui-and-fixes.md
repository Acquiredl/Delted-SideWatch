# Architecture: Subscription UI + Alertmanager Auth Fix + CI + CLAUDE.md Update

> Source: Gap analysis from prior session
> Date: 2026-03-26
> Mode: Feature (existing codebase)

## File Tree

```
~ frontend/lib/api.ts                                  ← add subscription types + fetcher helpers
+ frontend/app/subscribe/page.tsx                       ← subscription management page
+ frontend/components/SubscriptionStatus.tsx            ← tier badge, expiry, grace period display
+ frontend/components/SubscriptionPayment.tsx           ← subaddress + QR-style payment info
+ frontend/components/__tests__/SubscriptionStatus.test.tsx
+ frontend/components/__tests__/SubscriptionPayment.test.tsx
+ frontend/app/__tests__/subscribe.test.tsx             ← page-level test
~ frontend/components/Navigation.tsx                    ← add "Subscribe" nav link
~ frontend/app/miner/page.tsx                           ← add subscription tier badge + upsell CTA
~ config/alertmanager/alertmanager.yml                  ← add http_config.authorization for webhook
~ services/manager/cmd/manager/routes.go                ← accept Authorization: Bearer as alt to X-Admin-Token
~ .github/workflows/security.yml                        ← add frontend test + typecheck job
~ CLAUDE.md                                             ← update implementation status, remove stale future work
```

## Component Breakdown

### Feature: Subscription Frontend Page
- Files: `frontend/app/subscribe/page.tsx`, `frontend/components/SubscriptionStatus.tsx`, `frontend/components/SubscriptionPayment.tsx`, `frontend/lib/api.ts`, `frontend/components/Navigation.tsx`
- Dependencies: Backend subscription API endpoints (already exist)
- Complexity: **medium** — 3 new files, matches existing page patterns (miner/page.tsx), uses same SWR + Tailwind approach

### Feature: Miner Page Subscription Integration
- Files: `frontend/app/miner/page.tsx`
- Dependencies: Subscription status API, SubscriptionStatus component
- Complexity: **low** — add SWR call for subscription status, show tier badge, CTA to subscribe page

### Feature: Alertmanager Webhook Auth Fix
- Files: `config/alertmanager/alertmanager.yml`, `services/manager/cmd/manager/routes.go`
- Dependencies: none
- Complexity: **low** — change handler to also accept `Authorization: Bearer <token>`, add `http_config.authorization` to alertmanager.yml

### Feature: Frontend Tests in CI
- Files: `.github/workflows/security.yml`
- Dependencies: none
- Complexity: **low** — add one job (npm test + tsc --noEmit) to existing workflow

### Feature: CLAUDE.md Cleanup
- Files: `CLAUDE.md`
- Dependencies: none (should be last so it reflects final state)
- Complexity: **low** — text edits only

## Data Model

No new tables. All subscription data already exists (migration 003). Frontend consumes existing API responses:

### SubscriptionStatus (from GET /api/subscription/status/{address})
- Fields: miner_address (string), tier ("free"|"paid"), active (bool), expires_at (string|null), grace_until (string|null), has_api_key (bool)

### PaymentAddress (from GET /api/subscription/address/{address})
- Fields: miner_address (string), subaddress (string), suggested_amount_xmr (string), amount_usd (string)

### SubPayment (from GET /api/subscription/payments/{address})
- Fields: id (number), miner_address (string), tx_hash (string), amount (number), xmr_usd_price (number|null), confirmed (bool), main_height (number|null), created_at (string)

## Key Decisions

### Alertmanager Auth: Authorization header (not X-Admin-Token)
- **Chosen**: Change the webhook handler to accept BOTH `X-Admin-Token` AND `Authorization: Bearer <token>`. Alertmanager natively supports `http_config.authorization` with `type: Bearer` and `credentials_file`. This is the supported way to send auth headers in Alertmanager webhook configs.
- **Rejected**: Skip auth for internal network — weakens defense-in-depth. Any service on the Docker network could trigger fake alerts.
- **Rejected**: nginx proxy to inject header — adds unnecessary indirection for an internal service.

### Subscription Page: Standalone /subscribe route (not embedded in miner page)
- **Chosen**: Dedicated `/subscribe` page with address input, payment info, and status. Linked from miner page via CTA and from nav bar. This keeps the miner page focused on stats while giving subscription its own space for the payment flow.
- **Rejected**: Embed subscription UI in miner page — clutters the stats dashboard, mixes concerns. The payment subaddress display and API key generation need their own real estate.
- **Rejected**: Modal/dialog overlay — poor mobile experience, can't deep-link to subscription status.

### Frontend Tests in CI: Add to security.yml (not new workflow)
- **Chosen**: Add a `frontend-tests` job to the existing `security.yml` workflow. This workflow already triggers on PRs and pushes to main. Currently PRs only get security checks but no frontend tests (deploy.yml only runs on push to main). Adding frontend tests here means PRs get both security + test coverage.
- **Rejected**: New separate `ci.yml` workflow — more files to maintain, the infrastructure (node setup, npm ci) already exists in security.yml's npm-audit job.
- **Rejected**: Add PR trigger to deploy.yml — risky because build-images and deploy jobs would also trigger (even with environment protection, it's noisy).

## Build Phases

### Phase 0: Baseline
- **Goal**: Verify current builds compile and tests pass before changes
- **Files**: none (read-only)
- **Dependencies**: none
- **End Conditions**:
  - [ ] `cd services/manager && go build ./cmd/manager/` compiles
  - [ ] `cd services/gateway && go build ./cmd/gateway/` compiles
  - [ ] `cd frontend && npx tsc --noEmit` passes
  - [ ] Existing Go tests pass
  - [ ] Existing frontend tests pass

### Phase 1: Alertmanager Auth Fix
- **Goal**: Fix the unauthenticated webhook — handler accepts Bearer token, alertmanager.yml sends it
- **Files**:
  - `~ services/manager/cmd/manager/routes.go` — handleAlertWebhook accepts `Authorization: Bearer <token>` in addition to `X-Admin-Token`
  - `~ config/alertmanager/alertmanager.yml` — add `http_config.authorization` with `credentials_file` pointing to Docker secret
- **Dependencies**: Phase 0
- **End Conditions**:
  - [ ] `cd services/manager && go build ./cmd/manager/` compiles
  - [ ] `cd services/manager && go test ./...` passes
  - [ ] handleAlertWebhook accepts both X-Admin-Token header and Authorization: Bearer header
  - [ ] alertmanager.yml has http_config.authorization configured for the webhook
  - [ ] No regressions in existing tests

### Phase 2: Frontend Subscription Types + API Client
- **Goal**: Add TypeScript types and API helpers for the subscription endpoints
- **Files**:
  - `~ frontend/lib/api.ts` — add SubscriptionStatus, PaymentAddress, SubPayment interfaces + poster/fetcher helpers
- **Dependencies**: Phase 0
- **End Conditions**:
  - [ ] `cd frontend && npx tsc --noEmit` passes
  - [ ] SubscriptionStatus, PaymentAddress, SubPayment types defined
  - [ ] Helper for POST /api/subscription/api-key/{address} exists
  - [ ] Existing frontend tests pass

### Phase 3: Subscription Components
- **Goal**: Build the two core subscription UI components
- **Files**:
  - `+ frontend/components/SubscriptionStatus.tsx` — shows tier badge (Free/Paid), expiry date, grace period countdown, API key status
  - `+ frontend/components/SubscriptionPayment.tsx` — shows payment subaddress (monospace, copyable), suggested XMR amount, USD equivalent, payment history table
  - `+ frontend/components/__tests__/SubscriptionStatus.test.tsx`
  - `+ frontend/components/__tests__/SubscriptionPayment.test.tsx`
- **Dependencies**: Phase 2 (types must exist)
- **End Conditions**:
  - [ ] `cd frontend && npx tsc --noEmit` passes
  - [ ] SubscriptionStatus renders "Free" badge when tier is free
  - [ ] SubscriptionStatus renders "Paid" badge with expiry when tier is paid
  - [ ] SubscriptionPayment renders subaddress in monospace
  - [ ] SubscriptionPayment renders payment history table (reuses data-table pattern)
  - [ ] Component tests pass
  - [ ] Existing frontend tests pass

### Phase 4: Subscription Page + Navigation
- **Goal**: Wire the subscription page, add nav link, add tier badge to miner page
- **Files**:
  - `+ frontend/app/subscribe/page.tsx` — address input form, SWR calls to /api/subscription/*, renders SubscriptionStatus + SubscriptionPayment + API key generation
  - `+ frontend/app/__tests__/subscribe.test.tsx`
  - `~ frontend/components/Navigation.tsx` — add "Subscribe" to navLinks
  - `~ frontend/app/miner/page.tsx` — add SWR call for subscription status, show tier badge, link to /subscribe
- **Dependencies**: Phase 3 (components must exist)
- **End Conditions**:
  - [ ] `cd frontend && npx tsc --noEmit` passes
  - [ ] `cd frontend && npm test` passes (all existing + new tests)
  - [ ] `/subscribe` page renders address input form
  - [ ] After address lookup, page shows subscription status + payment info
  - [ ] Navigation component includes "Subscribe" link
  - [ ] Miner page shows subscription tier badge when address is active
  - [ ] No regressions in existing page/component tests

### Phase 5: Frontend Tests in CI
- **Goal**: Add frontend test + typecheck job to security.yml so PRs get coverage
- **Files**:
  - `~ .github/workflows/security.yml` — add `frontend-tests` job: setup node, npm ci, npm test, tsc --noEmit
- **Dependencies**: Phase 0
- **End Conditions**:
  - [ ] security.yml is valid YAML
  - [ ] security.yml has a `frontend-tests` job
  - [ ] Job runs npm test and npx tsc --noEmit in frontend/
  - [ ] No changes to deploy.yml gates (build-images still needs [test-go, test-frontend, security])

### Phase 6: CLAUDE.md Update
- **Goal**: Remove stale "future work" items, document subscription UI, reflect current state
- **Files**:
  - `~ CLAUDE.md` — update Implementation Status section
- **Dependencies**: Phases 1-5 (must reflect final state)
- **End Conditions**:
  - [ ] "Tax export endpoint" removed from "Potential future work" (it's implemented)
  - [ ] Subscription frontend documented in Implementation Status
  - [ ] "XMR subscription payment verification" updated to note frontend exists
  - [ ] Alertmanager auth fix documented
  - [ ] Frontend tests in CI documented
  - [ ] No broken markdown formatting

## Phase Dependency Graph

```
Phase 0 (Baseline) → Phase 1 (Alertmanager fix)
Phase 0 (Baseline) → Phase 2 (Types) → Phase 3 (Components) → Phase 4 (Page + Nav)
Phase 0 (Baseline) → Phase 5 (CI)
Phases 1-5 → Phase 6 (CLAUDE.md)
```

- Phase 1, Phase 2, and Phase 5 can run **in parallel** after Phase 0
- Phases 2→3→4 are strictly sequential (type deps)
- Phase 6 runs last (documents final state)

## Risk Register

1. **Subscription API may return unexpected shapes**: The subscription endpoints have never been tested with a real frontend. Response shapes in `types.go` may not match what the frontend expects (e.g., null vs omitted fields, atomic units vs XMR). **Mitigation**: Types are derived directly from Go struct JSON tags in `types.go`. Use defensive rendering (null checks, fallback values). mocknode doesn't serve subscription endpoints, so tests use mocked SWR responses.

2. **Alertmanager credentials_file requires Docker secret to exist**: The auth fix uses `credentials_file: /run/secrets/admin_token`. If the secret doesn't exist on the VPS, Alertmanager won't start. **Mitigation**: Document the required secret in alertmanager.yml comments. Alertmanager already runs in Docker with access to secrets. Existing `ADMIN_TOKEN` env var provides the same value — just needs to be mounted as a file.

3. **Navigation crowding on mobile**: Adding "Subscribe" as a 5th nav link may overflow on narrow screens. **Mitigation**: Existing nav uses `flex items-center gap-1` with `px-3 py-2` links. 5 items is still manageable. If needed, truncate to icon-only on small screens, but this is cosmetic — not a blocker.

4. **security.yml frontend-tests job may fail on npm ci**: The npm-audit job already does npm ci + npm audit. Adding another job that also does npm ci is slightly redundant but ensures test isolation. **Mitigation**: Cache npm dependencies using `cache: npm` in setup-node (same pattern as npm-audit job). If npm ci fails, it would also fail in npm-audit — so the failure surface doesn't change.

## Deployment Strategy

- **Platform**: Same Docker Compose stack on DigitalOcean VPS
- **No new services**: All changes are frontend files, config edits, and CI workflow changes
- **New requirement**: Alertmanager needs `admin_token` Docker secret mounted as a file (for `credentials_file`)
- **Pre-deploy**: `go build`, `go test`, `npm test`, `tsc --noEmit` all pass
- **Rollback**: Revert the commit. No schema changes, no state to clean up.
