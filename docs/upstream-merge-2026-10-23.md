# Upstream merge and fork audit — 23 October 2026

## Provenance

| Item                        | Revision                                           |
| --------------------------- | -------------------------------------------------- |
| Original fork               | `3e7efdb6` (`origin/main`, fork tag `v13.1.0`)     |
| Preserved original branch   | `backup/pre-upstream-20261023`                     |
| Last shared upstream commit | `ece16559397f756513d7338f59168049dd6e2e51`         |
| Original upstream version   | `v3.6.0` plus 19 commits; embedded version `3.6.0` |
| Integrated upstream         | `775b2b0b26a699bb5ae361ce3c78b319ab790991`         |
| Upstream release line       | `v3.10.2` plus four commits on upstream `main`     |
| Integration branch          | `merge/upstream-20261023`                          |

The fork had 73 commits beyond the shared revision, with a net change across 138 files. Upstream had 504 commits after that revision. The integration preserves both histories through a merge, rather than replacing the fork with an upstream tree.

## What the fork added

| Modification                                                                                                   | Principal code                                                                                                      |
| -------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| Reseller roles, accounts, quotas, usage, passwords, manual disabling and quota locks                           | `internal/database/model/model.go`, `internal/web/service/panel/reseller.go`, `internal/web/controller/reseller.go` |
| Reseller inbound selection, read-only inbound views and client ownership                                       | `internal/web/middleware/rbac.go`, inbound/client controllers and services                                          |
| Admin reseller management, reseller overview, client owner column and owner filter                             | `frontend/src/pages/admins`, `frontend/src/pages/reseller`, `frontend/src/pages/clients`                            |
| Inbound and reseller traffic multipliers, management page and usage accounting                                 | `frontend/src/pages/multiply`, `internal/web/controller/multiply.go`, traffic services                              |
| Independent Telegram sales bots per owner; plans, payment methods, proof review and delivery                   | `internal/resellerbot`, `internal/web/controller/reseller_bot.go`, `internal/web/service/panel/reseller_bot*`       |
| Wallet ledger, discount/redeem codes, referrals, trials, subscriptions, credential revocation and auto-renewal | Sales-bot services, models and jobs                                                                                 |
| Shared subscription URL/host resolution and Jalali conversion                                                  | `internal/web/service/subscription_url.go`, `observed_host.go`, `internal/util/jalali`                              |
| Navigation, translations, generated API schemas and documentation for these features                           | Frontend routes/sidebar, all 13 locales, `tools/openapigen`, both OpenAPI files                                     |
| Fork installation/update URLs and badges; release version stamping; narrower release architectures             | READMEs, installation scripts, release workflow, panel update service                                               |
| Intentional removal of the Docker publication workflow                                                         | `.github/workflows/docker.yml` remains absent                                                                       |

These features remain present. Upstream dependency upgrades supersede the fork's earlier Go version bump. Generated API documentation was rebuilt from the combined source. Upstream fixes that already cover the fork's translated host-doc corrections were retained.

For the full original changes and commit list:

```sh
git log upstream/main..backup/pre-upstream-20261023
git diff ece16559 backup/pre-upstream-20261023
git diff upstream/main main
```

## Corrections made during integration

- **Quota preservation:** an edit with no `usageLimit` previously decoded as zero and made a 3 TiB quota unlimited. Omitted limits now preserve the stored value; negative or out-of-range supplied values are rejected. The UI rounds GB input to integer bytes.
- **Quota locks:** editing/resetting a manually disabled reseller no longer reconnects their locked clients. Quota-triggered locks can still be cleared by the existing reset/limit-increase flows.
- **Multiplier accounting:** local deltas and remote-node client usage match reseller accounting; reusing a caller's delta cannot multiply it again. Raw node baselines remain raw, and multiplier adjustments and reseller usage commit together.
- **Repeated master snapshots:** global traffic snapshots are display/enforcement inputs. Receiving the same cumulative snapshot no longer charges the reseller again on every poll.
- **Counter overflow:** scaled values and reseller totals saturate safely instead of becoming negative. Multiplier lookups are batched, database failures propagate, and reseller updates acquire locks in a stable order.
- **Multiplier descriptions:** UI text and rate labels describe multiplication rather than addition. A rate of 1 is unchanged usage.
- **AmneziaWG reseller visibility:** selected-inbound pickers share the admin projection, including AmneziaWG/TUIC server information, transport/security hints and disabled-flow policy. Server private keys remain redacted.
- **Sales bots default off:** `resellerBotEnable` defaults to false. The main admin controls it in Settings → Telegram. While off, saved bots do not start, sales endpoints return 503, maintenance and wallet auto-renew work are skipped, and the sales-bot menu is hidden. Switching off stops running bots and preserves their saved configuration.
- **Reseller deletion:** optional client cleanup uses the existing client deletion service so attachments and runtime configuration are removed too. Kept clients transfer to admin ownership. Deleted owners' saved bots are disabled and stopped.
- **Mobile menus:** reseller management uses cards on phones; other added menus constrain table scrolling and wrap controls. Content clears the mobile menu button. Verified at 320 and 390 CSS pixels.
- **Upstream API changes:** bot renewals and credential revocation pass the new hardware-ID parameters without resetting existing limits.
- **Tooling:** CI pins golangci-lint 2.15.0, which supports the merged Go 1.27 toolchain.

Regression tests were run against the broken behavior before applying the quota, snapshot billing, counter overflow, remote multiplier, reused delta, default-off bot, manual-disable and reseller-deletion fixes.

## Validation

- `make verify`: passed (schema freshness, Go/frontend lint, formatting, TypeScript, shuffled Go tests, frontend tests, production builds and Storybook).
- Frontend: 189 test files and 1,862 tests passed.
- `make race`: passed; the final reseller, controller and bot changes were also rerun with the race detector.
- `make node-e2e`: passed against two real local panel processes with both admin and node-sync tokens.
- PostgreSQL migration tests and deferred commit-failure tests: passed against an isolated local PostgreSQL 16 container.
- SQLite → PostgreSQL → SQLite smoke test: preserved a 3 TiB quota, 1 TiB usage, reseller role, multiplier, allowed inbound IDs and saved bot configuration.
- Both parser/TLS-pin fuzz targets: passed 30-second runs.
- `govulncheck`: zero vulnerabilities affecting called code; one advisory exists in a required module that is not called by this code.
- Mobile browser checks: admin, multiplier, reseller and Telegram-bot pages had document width equal to viewport width at both 320 px and 390 px; screenshots were inspected.
- Installation/update shell syntax and docs lint: passed.

These checks use local test databases and mocked browser data, except the separate read-only running-server inspection. They do not establish the historical cause of any production quota change.

## Running-server findings

Read-only inspection reported fork version `v13.0.0`. All 54 current reseller accounts had positive finite quota values, including multi-terabyte limits. The current database does not demonstrate historical quota loss; the omitted-field bug above is independently reproduced. Existing lost quota values cannot be inferred safely without an earlier database or a known intended limit.

No production database values, binaries, settings, services or deployment files were changed. No live quota was guessed or reset. Credentials and database contents are not included in this report.

## Operational notes

The merged source requires Go 1.27.3, Node 26 and CGo. Frontend assets must be built before compiling the Go binary. Sales bots require explicit global opt-in even when individual saved bot configurations are enabled. The original fork revision remains available on the backup branch for comparison or rollback; rolling back an actual database upgrade should use its corresponding pre-upgrade database backup.
