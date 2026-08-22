# Soak Production Loop Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair the three production-loop failures exposed by the completed Actions/RCC soak and prove one convergence-admitted issue through the existing contributor path to a hold-gated PR.

**Architecture:** Keep scanner as a finding producer, add a server-side disclosure authorization check at the existing issue-request relay, and require an evidence receipt at the agent-facing bead terminal-transition boundary. Reuse ReadyQueue/selectTask, contributor leases, scoped GitHub App credentials, and PR verification for implementation delivery.

**Tech Stack:** Go, shell, embedded Markdown policies, Docker Compose, ClankeR contributor relay, GitHub App.

## Global Constraints

- Keep Hive and every scheduled agent parked until regressions and the one-issue canary pass.
- Do not edit the four already-public security issues.
- Do not change convergence, planning/adoption, enrollment, auto-merge, or hold policy.
- Do not introduce another scheduler or a generic Continuity refactor.
- Do not push the Hive fork publicly.

---

### Task 1: Private scanner security findings

**Files:**
- Modify: `src/pkg/github/issue_request_watcher.go`
- Modify: `src/pkg/agent/manager.go`
- Modify: `src/pkg/config/config.go`
- Modify: `bin/hive-open-issue.sh`
- Modify: `src/pkg/policies/defaults/scanner-holdgated.md`
- Test: `src/pkg/github/issue_request_watcher_test.go`
- Test: `src/pkg/agent/agent_capabilities_test.go`
- Test: `bin/test_hive_open_issue.sh`

- [ ] Add a failing watcher test proving sensitivity and finding identity reach authorization.
- [ ] Add a failing manager test proving a scanner security bead is denied without an operator-owned allowlist entry and admitted only with one.
- [ ] Add a failing wrapper test proving sensitivity and finding identity are serialized.
- [ ] Implement the request fields, authorization gate, and scanner policy instructions.
- [ ] Run the focused Go and shell tests.

### Task 2: Evidence-gated bead completion

**Files:**
- Modify: `src/pkg/beads/beads.go`
- Modify: `src/cmd/bd/main.go`
- Test: `src/pkg/beads/beads_test.go`
- Test: `src/cmd/bd/main_test.go`
- Modify: `src/pkg/policies/defaults/scanner-advisory.md`
- Modify: `src/pkg/policies/defaults/scanner-issues.md`
- Modify: `src/pkg/policies/defaults/scanner-holdgated.md`

- [ ] Add failing tests rejecting terminal transitions without an authoritative receipt and rejecting local-only evidence.
- [ ] Add a typed completion receipt and atomic close-with-receipt operation.
- [ ] Make `bd close` require a valid receipt and reject terminal status through `bd update`.
- [ ] Run focused bead and CLI tests.
- [ ] Reopen the two scanner beads prematurely closed during the soak.

### Task 3: Contributor implementation routing

**Files:**
- Modify only if a regression exposes a gap: `src/pkg/dashboard/contribute_ws.go`
- Test: `src/pkg/dashboard/contribute_dependency_admission_test.go`
- Test: `src/pkg/dashboard/contribute_ws_integration_test.go`

- [ ] Prove the existing ReadyQueue/selectTask path selects from the same convergence-admitted population.
- [ ] Prove a verified PR is required for durable shipped completion.
- [ ] Configure one local headless Codex contributor relay; do not enable scanner as an executor.

### Task 4: One-issue canary and receipts

**Files:**
- Modify: `docs/dogfood/actions-rcc-findings-2026-08.md`
- Create: `docs/dogfood/actions-rcc-factory-soak-2026-08.md`
- Create: `docs/dogfood/actions-rcc-canary-delivery-2026-08.md`

- [ ] Build and restart the source-backed stack with all scheduled agents paused.
- [ ] Run the published gateway regression.
- [ ] Start one contributor relay and let selectTask choose exactly one issue.
- [ ] Verify repository, tests, signed commit, remote branch, hold-gated App-authored PR, and server-verified completion receipt.
- [ ] Stop the contributor relay before it requests another task.
- [ ] Update the findings ledger and prior-soak receipt, then commit locally without pushing.
