# Code Review Findings

## High

1. Longest-prefix lookup returns routes for shorter prefixes too.
- Location: `internal/store/route.go:36`, `internal/store/route.go:44`, `internal/handler/routes.go:87`
- Details: `LPMLookup` selects all rows where `prefix >>= $2::inet`, sorts by mask length, and returns up to 100 rows. That includes less-specific prefixes after the longest match. The handler then sets `responsePrefix` to `routes[0].Prefix`, so the response can claim one matched prefix while `routes` contains paths from different prefixes.
- Impact: Incorrect route-lookup behavior for longest-match queries and inconsistent API payloads.

## Medium

1. `has_more` in route history can be incorrect.
- Location: `internal/store/history.go:23`, `internal/handler/history.go:107`
- Details: Query uses `LIMIT $5`, and `has_more` is set by `len(events) == limit`. If total rows are exactly `limit`, `has_more` is incorrectly `true`.
- Impact: Clients can show a phantom next page and loop unnecessarily.

2. Route history accepts invalid time ranges where `from > to`.
- Location: `internal/handler/history.go:63`
- Details: Validation only checks `to.Sub(from) > 7*24*time.Hour`; it never rejects reversed ranges.
- Impact: Invalid user input is silently accepted and can return confusing empty datasets.

3. Router existence check for history/lookup misses routers that only exist in historical events.
- Location: `internal/store/router.go:205`, `internal/store/router.go:207`
- Details: `GetRouterSummary` checks `routers`, `rib_sync_status`, and `current_routes`, but not `route_events`. A router with historical events but no current/live state can be reported as not found.
- Impact: False `404` on history queries for valid historical router IDs.

4. Frontend flattens AS_SET data and changes path semantics.
- Location: `web/src/api/client.ts:136`
- Details: `route.as_path.flat()` converts nested AS_SET entries into a flat list, losing set boundaries.
- Impact: Displayed AS paths can be semantically wrong versus backend data.

5. Best-path indication is dropped for all multi-path results.
- Location: `web/src/api/client.ts:131`
- Details: `best` is set to `api.routes.length === 1 && route.path_id === 0`. Any response with multiple paths marks every path as non-best, even when one should be preferred.
- Impact: The UI loses an important route-selection signal for exactly the scenarios where operators need it most.

6. Unknown ASN is coerced to `0` in target mapping.
- Location: `web/src/api/client.ts:113`, `web/src/components/TargetSelector.tsx:19`
- Details: `as_number ?? 0` makes missing ASN appear as `AS0` in UI labels.
- Impact: Misleading operator-facing data.

7. No test coverage remains for the refactored backend.
- Location: `cmd/api/main.go`, `internal/handler/routes.go`, `internal/store/route.go`
- Details: `go test ./...` reports `[no test files]` for all current Go packages after this refactor.
- Impact: Core API/store behavior changes have no automated regression protection.

## Low

1. `no rows` handling relies on string matching instead of sentinel errors.
- Location: `internal/store/router.go:148`, `internal/store/router.go:210`
- Details: Checks `err.Error() == "no rows in result set"`.
- Impact: Brittle error handling if error wrapping/message format changes.
