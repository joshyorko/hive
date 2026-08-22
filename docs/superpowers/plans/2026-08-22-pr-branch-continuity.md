# PR and Branch Continuity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an owner explicitly adopt an existing GitHub pull request, preserve its branch and history, suppress replacement work for its owned slice, and expose safe continuation through Hive's existing contributor path.

**Architecture:** Add a small source-neutral `pkg/continuity` authority ledger keyed by canonical pull-request identity. A GitHub observer produces current PR, branch, topology, relationship, verification, and write-capability evidence; an owner-only dashboard operation validates that evidence before persisting adoption. The existing GitHub claim guard consumes active adoption as an authoritative claim, while the existing actionable/status/contributor path receives eligible continuation envelopes; `pkg/convergence` remains unchanged.

**Tech Stack:** Go, `go-github/v72`, atomic JSON persistence under `/data`, existing dashboard owner authentication, existing worksource identity and contributor protocol.

## Global Constraints

- Root all work at freshly fetched `upstream/v4`; reapply only reviewed ACMM deltas.
- Preserve the running ACMM reconciler and private security work.
- Discovery is not authority; GitHub labels alone never authorize adoption.
- No replacement branch, replacement PR, force-push, rebase, amend, squash, or author rewriting.
- Adoption is distinct from hold/review/merge authorization.
- Actions/RCC are read-only during the dogfood dry-run.
- Do not modify `pkg/convergence` with GitHub- or PR-specific semantics.

---

### Task 1: Durable adoption authority

**Files:**
- Create: `src/pkg/continuity/types.go`
- Create: `src/pkg/continuity/ledger.go`
- Test: `src/pkg/continuity/ledger_test.go`

**Interfaces:**
- Produces: `PRRef`, `Record`, `Observation`, `State`, `Ledger.Adopt`, `Ledger.Refresh`, `Ledger.Revoke`, `Ledger.LookupWork`, and `Ledger.List`.
- Persists: `/data/continuity-pr-adoptions.json` with schema version, monotonic generation, observed head/base identity, actor, provenance, timestamps, relationships, acceptance slices, and lifecycle history.

- [ ] Write failing tests for authorized adoption, idempotency, restart reconstruction, active work lookup, revocation, and unexpected-head fail-closed refresh.
- [ ] Run `go test ./pkg/continuity` and confirm failures are caused by missing implementation.
- [ ] Implement the minimal atomic ledger and immutable read snapshots.
- [ ] Re-run `go test ./pkg/continuity` and `go test -race ./pkg/continuity`.

### Task 2: GitHub PR observation and bounded delivery graph

**Files:**
- Create: `src/pkg/github/continuity.go`
- Test: `src/pkg/github/continuity_test.go`

**Interfaces:**
- Consumes: `continuity.PRRef`.
- Produces: one `continuity.Observation` from authoritative PR, repository, comparison, files, reviews/checks, and linked-issue evidence.

- [ ] Write failing fixtures for human drafts, exact head/base identity, fork/unwritable branches, closing/reference semantics, stack ancestry, file overlap, and unrelated PR isolation.
- [ ] Confirm RED with `go test ./pkg/github -run Continuity`.
- [ ] Implement bounded paginated observation without mutation or label authority.
- [ ] Re-run focused and race tests.

### Task 3: Owner-gated adoption API

**Files:**
- Create: `src/pkg/dashboard/api_continuity.go`
- Create: `src/pkg/dashboard/api_continuity_test.go`
- Modify: `src/pkg/dashboard/api.go`
- Modify: `src/pkg/dashboard/deps.go`
- Modify: `src/pkg/dashboard/server.go`
- Modify: `src/pkg/dashboard/owner_only_handlers_deny_test.go`

**Interfaces:**
- Produces: `GET /api/continuity/pr-adoptions`, `POST /api/continuity/pr-adoptions`, and dry-run refresh/adopt/revoke actions.
- Authority: `requireOwnerRole` plus server-derived `requestUser`; request body cannot nominate the principal.

- [ ] Write failing tests proving anonymous/viewer/agent-label adoption is refused, owner adoption persists provenance, repeated adoption is idempotent, and revocation removes continuation authority.
- [ ] Confirm RED with focused dashboard tests.
- [ ] Implement handlers over the continuity ledger and GitHub observer.
- [ ] Re-run focused dashboard tests and owner-route denial coverage.

### Task 4: Duplicate-work suppression and contributor continuation projection

**Files:**
- Modify: `src/pkg/github/prclaims.go`
- Modify: `src/pkg/github/prclaims_test.go`
- Modify: `src/pkg/github/client.go`
- Modify: `src/pkg/dashboard/status_builder.go`
- Modify: `src/pkg/dashboard/contribute_ws.go`
- Modify: `src/pkg/dashboard/contribute_protocol.go`
- Add focused tests under `src/pkg/dashboard/`.

**Interfaces:**
- Consumes: active continuity adoption snapshot.
- Produces: adopted claims honored by `FilterClaimedIssues`, and continuation envelopes keyed as `owner/repo!pr-N` in the existing contributor selection path.

- [ ] Write failing tests showing ordinary human drafts remain ignored by the implementation queue, active adoption suppresses the linked owned slice, separate slices on one issue remain distinct, and revoked adoption releases suppression.
- [ ] Write failing contributor tests showing the existing head branch/base/head SHA are delivered, unexpected movement withholds assignment, and no unrelated human PR is surfaced.
- [ ] Confirm RED.
- [ ] Implement the smallest additive claim and continuation projection without adding a scheduler.
- [ ] Re-run ReadyQueue/selectTask parity and race tests.

### Task 5: Dogfood dry-run and ACMM correlation packet

**Files:**
- Create: `docs/continuity.md`
- Create: `docs/dogfood/actions-rcc-pr-continuity-2026-08-22.md`
- Create: `docs/dogfood/actions-rcc-acmm-correlation-2026-08-22.json`
- Update: existing shareable dogfood ledger where applicable.

**Interfaces:**
- Consumes: read-only live GitHub state for Actions PRs 109, 112, 115, 119, and 128 plus open generated ACMM issues.
- Produces: deterministic JSON/Markdown observations and proposed dispositions only; no Actions/RCC mutation.

- [ ] Refresh each PR's head/base, linked issues, topology, files, checks, acceptance delta, closing semantics, and write capability.
- [ ] Run adoption evaluation in dry-run mode and record proposed state/action.
- [ ] Correlate all remaining generated ACMM issues into existing/adopted coverage, coherent outcomes, genuine gaps, evaluator gaps, or human-policy decisions.
- [ ] Verify the packet does not contain secrets or private security details.

### Task 6: Final verification and publication

**Files:** all touched files.

- [ ] Run `gofmt`, `git diff --check`, focused tests, race tests, `go vet` on changed packages, and `go build ./...`.
- [ ] Run broader Go and shell suites as practical; isolate proven host-only tmux/iptables failures.
- [ ] Re-fetch `upstream/v4`, compare touched paths, and reconcile material drift.
- [ ] Commit shareable source/tests/docs with preserved ACMM history.
- [ ] Push `dogfood/pr-branch-continuity-2026-08-22` to `joshyorko/hive` without opening a PR.
