# GitHub Source Dependency Dogfood Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete source-observed GitHub dependency admission, deploy it with `hive-managed` enrollment and convergence enforcement, run the Actions/RCC L5 factory for six hours, then park it and record the result.

**Architecture:** GitHub enumeration carries issue bodies and current states into a bounded source snapshot. `pkg/worksource` parses only explicit hard-dependency fields and emits `convergence.Observation`; dashboard admission conjunctively composes it with bead observations, and existing queue/kick paths consume the shared evaluator. Operator scripts and a user-systemd timer enforce the experiment deadline without deleting durable state.

**Tech Stack:** Go, GitHub API, Docker Compose, shell, user systemd, Hive dashboard API, Codex through Headroom.

## Global Constraints

- Preserve the dirty dependency-observation work, local security commit, `/data`, GitHub App scope, Headroom project, Codex OAuth, model allocation, ACMM L5, hold labels, and disabled auto-merge.
- `pkg/convergence` remains source/runtime independent; no synthetic beads, second scheduler, LLM parsing, or arbitrary-link inference.
- Only `hive-managed` candidate issues contribute mutable source dependency declarations; targets are restricted to configured repositories.
- No Kubernetes mutation, public security disclosure, public push, replicas, auto-merge, or hold removal.

---

### Task 1: Parser and observation correctness

**Files:**
- Modify: `src/pkg/worksource/worksource_test.go`
- Modify: `src/pkg/worksource/dependencies.go`

**Interfaces:**
- Consumes: `DependencySnapshot`, `Ref`
- Produces: `ObserveDependencies(DependencySnapshot, Ref) convergence.Observation`

- [ ] Add RED cases for `hive-managed`, explicit GitHub issue URLs, ordinary Markdown wrappers, malformed fields, forced-False self/cycles, ignored metadata fields, reopening, missing/inaccessible targets, and deterministic deduplication.
- [ ] Run focused worksource tests and verify each new behavior fails for the intended reason.
- [ ] Implement the minimal parser/index changes, retaining strict line-anchored field semantics and configured-repository authority.
- [ ] Run focused tests, race tests, and package tests.

### Task 2: Shared admission and enumeration parity

**Files:**
- Modify: `src/pkg/dashboard/contribute_source_dependency_admission_test.go`
- Modify: `src/pkg/dashboard/contribute_admission_deps.go`
- Modify: `src/pkg/dashboard/convergence_kick.go`
- Modify: `src/pkg/github/client.go`
- Modify: `src/cmd/hive/convergence_kick.go`
- Modify: `src/cmd/hive/main.go`

**Interfaces:**
- Consumes: one-enumeration `IssueResult.SourceItems`
- Produces: identical ReadyQueue/selectTask/internal-kick decisions from composed bead and source observations

- [ ] Add RED tests for source-only, bead-only, union composition, ReadyQueue/selectTask parity, internal-kick parity, shadow/enforce reuse, restart reconstruction, and bounded source indexing/API behavior.
- [ ] Run focused dashboard/GitHub tests and verify RED failures.
- [ ] Complete the smallest implementation necessary; index each source issue and dependency identity once per observation generation.
- [ ] Run focused, package, race, and full source tests.

### Task 3: Build, deploy shadow, and reconcile live graph

**Files:**
- Modify: `/var/home/kdlocpanda/services/hive-local/hive.yaml`
- Preserve: `/var/home/kdlocpanda/services/hive-local/codex-allocation.yaml`

**Interfaces:**
- Consumes: locally built `hive-local:dogfood`
- Produces: healthy source-backed Hive running source observation in `shadow`

- [ ] Commit the dependency observer locally without pushing.
- [ ] Take a fresh cold backup, build from `/var/home/kdlocpanda/services/hive`, and recreate Hive/gateway without deleting volumes.
- [ ] Re-run published-gateway security regression and verify model/auth configuration.
- [ ] Revalidate every readiness-report issue and current open PR claim using read-only GitHub state.
- [ ] Compare shadow admission with READY/BLOCKED classifications and resolve every mismatch.

### Task 4: Migrate enrollment and enforce

**Files:**
- Modify: `/var/home/kdlocpanda/services/hive-local/hive.yaml`

**Interfaces:**
- Consumes: reconciled READY/BLOCKED executable set
- Produces: `hive-managed` enrollment with convergence `enforce`

- [ ] Ensure the `hive-managed` label exists in both configured repositories.
- [ ] Apply it only to freshly verified READY/BLOCKED executable issues; preserve all `hive-ready` labels.
- [ ] Change the required label to `hive-managed`, set convergence to `enforce`, recreate safely, and prove expected admitted/blocked/unknown counts.
- [ ] Prove every admitted issue has an enabled implementation-capable lane before launch.

### Task 5: Six-hour watchdog and factory launch

**Files:**
- Create: `/var/home/kdlocpanda/services/hive-local/park-dogfood.sh`
- Create: `/var/home/kdlocpanda/services/hive-local/abort-dogfood.sh`
- Create: user-systemd transient or persistent watchdog unit/timer

**Interfaces:**
- Consumes: owner-only dashboard pause API
- Produces: deadline park with container-stop fallback and an experiment-state receipt

- [ ] Write scripts that pause every scheduled agent without printing credentials, verify dispatch is disabled, and stop Compose services without volumes if pausing fails.
- [ ] Validate scripts without parking the pre-launch system; install and verify the user-systemd deadline.
- [ ] Record start/deadline/unit, baseline GitHub/Headroom/API metrics, then enable supervisor, scanner, ci-maintainer, quality, guide, and sec-check only.
- [ ] Monitor circuit breakers and natural dependency transitions until the deadline.

### Task 6: Park and dogfood receipt

**Files:**
- Create: `docs/dogfood/actions-rcc-factory-soak-2026-08.md`

**Interfaces:**
- Consumes: final Hive, GitHub, Headroom, security, and watchdog evidence
- Produces: durable answer to whether dependency-frontier advancement occurred automatically

- [ ] Confirm the watchdog parks every agent at or before six hours and no new dispatch can occur.
- [ ] Capture final admission counts, transitions, issues, PRs, CI, duplicate-work, latency, GitHub API, Headroom/model, security, and intervention evidence.
- [ ] Write and verify the receipt, including exact source SHA/status and explicit automatic-frontier verdict.
- [ ] Leave containers/dashboard available for inspection with all agents paused.
