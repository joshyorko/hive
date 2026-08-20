# Convergence implementation packet

**Status:** working execution contract for decomposing [#3845](https://github.com/kubestellar/hive/issues/3845) into current-tree, agent-ready issues  
**Reconciled:** 2026-08-20  
**Source baseline:** `v4@380a4ad80cfe6ab65e85b63e84745dfeae545463`  
**Landed convergence slices:** [#3857](https://github.com/kubestellar/hive/pull/3857), [#3904](https://github.com/kubestellar/hive/pull/3904)

This packet does **not** replace #3845 as the upstream design contract. It reconciles that contract and its supporting research against the implementation that now exists. For implementation facts, the pinned source tree and merged code win. For architectural intent, #3845 wins unless implementation evidence demonstrated that an assumption was unsafe, unreachable, or unnecessarily broad.

The two linked reports in #3845 remain useful design evidence, but they are point-in-time, non-normative snapshots. Agents must not load them wholesale or treat their old first-implementation recommendations as current instructions.

---

## 1. Governing objective

Hive should continuously judge declared repository outcomes against current authoritative observations and admit only safe, authorized transitions into its existing routing and execution machinery.

```text
desired intent
    ↓
authoritative observations
    ↓
proof / condition evaluation
    ↓
known drift / uncertainty
    ↓
admissible transitions
    ↓
existing Hive policy, scheduling, routing, and execution
    ↓
observe again
```

The convergence layer is not a second planner or scheduler.

- Planning decides what work ought to exist or be revised.
- Convergence judges what is currently true and which transitions are admissible.
- Hive keeps ownership of policy, prioritization, cadence, lane/role routing, contributor lifecycle, and execution.

The durable target is not merely a closed task. It is an **outcome under an authoritative acceptance generation**, supported by evidence for exact subjects and relevant assumptions.

---

## 2. Source priority and reading policy

When sources disagree, use this order:

1. Current `v4` source at the exact baseline SHA above.
2. Final merged state and review history of #3857 and #3904.
3. Canonical issue body of #3845.
4. Supporting research reports linked by #3845.
5. Historical #3828 discussion.

Minimum source set for every child-issue transaction:

- #3845.
- This packet.
- Current files named by that issue pack at the then-current `v4` SHA.
- Duplicate search across open and recently closed issues/PRs.

Load the old research only when a pack names a specific question. Do not spend context loading both full PDFs into every worker.

---

## 3. Status vocabulary

- **LANDED** — the production path and load-bearing tests now establish the property.
- **PARTIALLY LANDED** — a forward-compatible seam or adjacent primitive exists, but the #3845 property is not established end to end.
- **SUPERSEDED BY IMPLEMENTATION EVIDENCE** — the original intent remains useful, but the proposed mechanism or universal rule was disproved or narrowed by implementation.
- **STILL OPEN** — the current tree does not establish the property.
- **REQUIRES FURTHER RESEARCH / DECISION** — an implementation issue would currently encode an arbitrary policy choice or under-specified safety contract.
- **DEFERRED** — intentionally not justified by the current vertical.

---

## 4. As-built architecture

### 4.1 Shared contributor admission — LANDED

[#3857](https://github.com/kubestellar/hive/pull/3857) extracted one deterministic contributor-neutral admission decision used by both:

- `ReadyQueue` offerability; and
- live `selectTask` assignment.

Open-PR claim suppression can no longer advertise work that live assignment refuses. Contributor-specific cooldown, role, trust, rate, ranking, and routing remain downstream.

Current seam:

- [`src/pkg/dashboard/contribute_admission.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/dashboard/contribute_admission.go)
- [`src/pkg/dashboard/contribute_sse.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/dashboard/contribute_sse.go)
- [`src/pkg/dashboard/contribute_ws.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/dashboard/contribute_ws.go)

### 4.2 Pure convergence evaluator — LANDED for admission

[#3904](https://github.com/kubestellar/hive/pull/3904) added [`src/pkg/convergence`](https://github.com/kubestellar/hive/tree/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/convergence):

- pure `Evaluate(Observation) Decision`;
- no GitHub types, bead types, HTTP, I/O, clock, package state, CRDs, or scheduler behavior;
- Kubernetes-style `True`, `False`, and `Unknown` condition status;
- current condition types limited to `Observed` and `Ready` because richer conditions are not yet computed;
- decision carries admission, reason, blockers, observed record, observed generation, and conditions.

This package currently answers **admission**, not full repository convergence.

### 4.3 Legacy dependency observer — LANDED

The first observer in [`contribute_admission_deps.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/dashboard/contribute_admission_deps.go):

- resolves GitHub issue candidates into bead records;
- projects `DependsOn` as the narrow synthetic condition `LegacyWorkCompleted(beadID)`;
- treats open dependencies as unsatisfied;
- treats unresolved, non-retired dependencies as unknown;
- treats terminal dependencies and IDs retired only after terminal state as satisfied;
- recomputes from current ledger state on every sweep, so readiness is reversible;
- blocks only the affected candidate;
- shares the decision between queue projection and live assignment;
- reconstructs after restart from durable bead and retirement archive state.

`Store.Ready()` deliberately remains a planning-local query and still does not evaluate `DependsOn`.

### 4.4 Snapshot coherence and cost guard — LANDED

Implementation review found that `Store.List` returned live bead pointers after releasing its lock. #3904 added `Store.ReadEach` so projection occurs under the store read lock and only copied values escape. A race test exercises concurrent status/dependency writers against the production admission path.

The final sweep was also reduced from roughly 12.3 ms / 15.8 MB / 57,867 allocations to roughly 5.0 ms / 4.8 MB / 17,739 allocations for eight stores × 5,000 beads with 150 candidates. `BenchmarkAdmissionSweep` is the current cost guard.

### 4.5 Contributor assignment fencing — LANDED as adjacent prior art

The contributor protocol now has server-owned renewable task leases and task generations. Progress renews the lease; stale-generation completion/failure is rejected after revocation or reassignment.

This is useful prior art, not the repository-wide resource-claim or mutation-ownership model requested by #3845. It remains tied to contributor assignment identity and protocol behavior.

### 4.6 Work-source abstraction — PARTIALLY LANDED after the original research

The current tree now includes [`pkg/worksource`](https://github.com/kubestellar/hive/tree/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/worksource):

- source-neutral `Issue` with `SourceType`, `ExternalID`, repository target, and other fields;
- `TaskKey` preserving GitHub `repo#number` and providing string-key identities for Linear/Jira;
- GitHub Issues, GitHub Projects, Linear, and Jira read adapters;
- governor configuration and Phase-1 enumeration wiring.

However, this is not yet source-neutral end to end:

- `ToGitHubIssues` drops `SourceType`, `ExternalID`, source state, and priority;
- non-GitHub issues enter the legacy `github.Issue` shape with `Number == 0`;
- `convergence.Subject` remains `Repo + Number`;
- the legacy bead observer is GitHub-issue specific;
- the completed foundation issue #4185 said all identity-key sites would migrate, but merged #4189 changed only the new package/adapter files. Current production use of `TaskKey` is not the promised identity migration.

This is a current-tree prerequisite for source-neutral convergence and contributor behavior.

---

## 5. Reconciliation matrix

| Architectural area | Status after #3857/#3904 | Current evidence / consequence |
|---|---|---|
| One contributor-neutral admission contract | **LANDED** | #3857; `ReadyQueue` and `selectTask` call the same decision seam. |
| Open-PR claim parity | **LANDED** | Claimed issues are neither advertised nor assigned; unrelated work remains. |
| Runtime-independent evaluator | **LANDED** | `pkg/convergence` is pure and minimal. |
| Tri-state conditions | **LANDED for `Observed`/`Ready`** | `True`/`False`/`Unknown`; richer types intentionally absent. |
| Legacy `DependsOn` admission | **LANDED** | #3904 observer and production-path tests. |
| Reversible readiness | **LANDED for legacy dependencies** | Current ledger state is recomputed; no latch or queue surgery. |
| Race-safe coherent ledger projection | **LANDED** | `ReadEach`, copied records, race test. |
| Restart reconstruction | **LANDED for legacy observer** | Bead store + persisted retired IDs. |
| Candidate-local uncertainty | **LANDED** | One unknown dependency does not serialize unrelated work. |
| Desired / acceptance generation | **PARTIALLY LANDED** | `Generation` is currently an `UpdatedAt`-derived reported value and is never compared. |
| Repository/outcome conditions and status | **PARTIALLY LANDED** | Admission conditions exist; no durable outcome identity, `Converged`, `Degraded`, human/external blocking, or quiescence status. |
| Exact-subject proof | **STILL OPEN** | No generic proof bound to candidate SHA, base/input assumptions, producer/authority, freshness, and acceptance generation. |
| Non-monotonic condition dependencies | **PARTIALLY LANDED** | Legacy dependency satisfaction can flip while a live bead exists; historical terminal retirement remains monotonic by design. |
| Selective invalidation | **STILL OPEN** | No assumption graph or proof invalidation by changed SHA/base/policy/harness/scanner input. |
| Partial-observation authority | **REQUIRES FURTHER RESEARCH / DECISION** | Current partial ledger policy preserves fleet availability by admitting records outside the readable subset; it cannot prove an unseen candidate had no declaration in a failed source. |
| Operator-visible blocked/unknown status | **STILL OPEN** | Non-admitted candidates disappear from offerable `ReadyQueue`; reduced coverage is logged but not a first-class convergence projection. |
| Internal-agent dispatch parity | **STILL OPEN** | Scheduler/governor consumes actionable issue sets and does not consume `pkg/convergence.Decision`. |
| Source-neutral work identity | **PARTIALLY LANDED** | `worksource.Issue`/`TaskKey` exist; convergence and many identity maps still use GitHub numeric shape. |
| Resource claims and bounded writer capacity | **STILL OPEN** | No repository resource/write-set claim model. |
| Mutation leases/fencing | **PARTIALLY LANDED as contributor prior art** | Task generation and lease exist for contributor assignments, not generic repository mutations. |
| Idempotent side-effect replay | **STILL OPEN** | No generic operation journal for crash-after-effect-before-ack. See #4002 prior art. |
| Authority / anti-self-certification | **PARTIALLY LANDED as separate intent prior art** | `pkg/intent` gates PR tiers using linked issue, approved plan, human approval, and alignment; it is not convergence outcome/proof authority. |
| Discovery / objective promotion | **STILL OPEN** | No explicit promotion from discovered observation into declared mandatory intent. |
| Additional evidence providers | **STILL OPEN** | Work sources enumerate tasks; they are not the proof/evidence-provider model. |
| Project/cross-repository aggregation | **STILL OPEN** | No cross-repository outcome/condition identity, cycle handling, or aggregation. |
| Kubernetes API / CRD adapter | **DEFERRED** | Still unjustified; runtime-independent core is the correct current seam. |

---

## 6. Assumptions superseded by implementation evidence

### 6.1 `Store.Ready()` is not the first production seam

The early “teach `Store.Ready()` about `DependsOn` first” path is superseded. `Store.Ready()` is not the live contributor admission authority. #3857/#3904 correctly established and extended the shared live admission contract instead.

Do not create a child issue whose acceptance is merely “`Store.Ready()` filters dependencies.” That would create a second gate with independent semantics.

### 6.2 Mandatory co-publication with `runEvalCycle` was not required for the first vertical

The reports suggested co-publishing an immutable admission snapshot with the status generation. The implemented per-pass sweep established queue/assignment parity without new persisted/cache state. Each call gets one coherent bead projection and uses it for that pass.

A future issue may prove that cross-subsystem epoch consistency is required, but it must demonstrate an actual stale-world invariant rather than reinstating the old mechanism by assumption.

### 6.3 Global fail-closed behavior on partial legacy-ledger coverage is unsafe

The original rule “a degraded read authorizes nothing” remains correct **when the affected candidate is known to depend on that authority**.

It is not safe as a global rule over today’s sparse legacy bead population. Once startup load failures became observable, withholding every candidate not found in the readable subset would empty most of the contributor queue because most actionable issues have no bead record.

The final #3904 policy therefore:

- gates records it can see;
- admits candidates it cannot map in the truncated legacy view; and
- logs reduced coverage.

This is an availability-preserving compromise, not proof of safe absence. The long-term fix is failure-domain-aware authority: identify which source can authoritatively own a candidate’s intent before deciding whether an outage makes its absence trustworthy.

### 6.4 Missing dependency IDs are not uniformly unknown

A missing ID that was durably retired only after reaching terminal state is satisfied. A missing ID with no such evidence remains unknown.

Any future observer must preserve the distinction between:

- absent because it never existed / cannot be read;
- absent because authoritative terminal evidence was archived; and
- absent because the candidate is intentionally unmanaged.

### 6.5 Broad ontology persistence is not required before a vertical

The minimal pure types were sufficient for the first production slice. Do not create persisted Go types or CRDs for every conceptual noun in #3845 before the system computes them.

Add a type when a production vertical establishes its source, authority, lifecycle, and tests.

### 6.6 Automatic model-driven investigation is not required for every `Unknown`

The first slice safely withholds the affected candidate and re-observes. Model-driven investigation may later reduce uncertainty, but it must be an admitted read-only transition with bounded cost and explicit evidence—not the default implementation of tri-state conditions.

### 6.7 Full-record scans under loose locking are not acceptable

Implementation evidence made snapshot discipline and cost part of the contract. New observers on an assignment hot path must:

- project under the authoritative source’s lock/transaction boundary;
- avoid retaining mutable source objects;
- provide race coverage; and
- include a representative cost guard when work scales with ledger or fleet size.

### 6.8 The original #3904 PR-body partial-read statement is historical

The initial PR body said partial reads withhold unfound candidates. Final merged code deliberately reversed that after proving it would create a fleet-wide stall. Current source and the final review record are authoritative.

### 6.9 The source-neutral identity foundation is not complete merely because #4185 closed

#4185 explicitly required replacing the production identity-key sites. #4189 added the helper and default adapter but changed only three files. Treat the identity migration as open implementation work; do not rely on the closed issue’s state as proof.

---

## 7. Remaining non-negotiable invariants

1. **Reconstructability:** no authoritative judgment depends only on an agent’s remembered context or a process-local event history.
2. **One admission meaning:** every mutating execution path must consume the same normalized judgment or prove an equivalent boundary.
3. **Level-triggered truth:** events trigger reevaluation; current authoritative state determines the result.
4. **Desired generation:** changing an acceptance contract creates a new generation.
5. **Exact-subject proof:** evidence is valid only for the exact subject and declared input assumptions it evaluated.
6. **Selective invalidation:** changed assumptions invalidate only dependent judgments.
7. **Candidate-local uncertainty:** one unknown does not globally serialize unrelated work.
8. **Failure-domain locality:** an unavailable authority blocks the outcomes whose truth depends on it, not every candidate in the fleet.
9. **No waiting-capacity leak:** CI, review, publication, external dependency, and human-decision waits do not consume mutation-writer capacity.
10. **Fenced ownership:** stale owners cannot assert authoritative mutation completion after reassignment.
11. **Idempotent effects:** a crash between external effect and acknowledgment cannot produce duplicate authoritative effects.
12. **Authority separation:** implementation cannot weaken acceptance and certify itself unless policy explicitly grants both authorities.
13. **Auditable decisions:** acceptance, rejection, supersession, retry, and authority decisions leave durable receipts.
14. **No second scheduler:** convergence admits; existing Hive machinery selects and routes.
15. **No CRD-first design:** an adapter follows a proven runtime need.

---

## 8. Implementation dependency graph

```text
#3857 shared contributor admission
        ↓
#3904 legacy dependency/condition admission
        ├──────────────┐
        ↓              ↓
A1 source-aware     A2 admission diagnostics
WorkRef identity       / status projection
        │              │
        └──────┬───────┘
               ↓
        A3 internal-agent
        dispatch admission parity
               ↓
        A4 narrow shadow validation

Decision Gate G1: canonical outcome/intent owner
               ↓
B1 outcome identity + desired/observed generation status
               ↓
Research Gate G2: exact-subject GitHub proof fingerprint
               ↓
B2 exact-subject CI/review proof vertical
               ↓
B3 non-monotonic conditions + selective invalidation

Research/Decision Gate G3: failure-domain-aware observer authority
               └───────────── feeds B1/B2/B3 and replaces the legacy partial-view compromise

B1/B2/B3 + existing contributor lease prior art
               ↓
C1 resource claims + bounded mutation capacity
               ↓
C2 repository mutation lease/fencing + idempotent operation journal
               ↓
C3 authority / anti-self-certification
               ↓
D1 discovery, additional providers, project/cross-repo aggregation
               ↓
D2 optional Kubernetes API/CRD adapter only if deployment evidence justifies it
```

A-series issues are current-tree seam work. B/C/D issues must not be opened as implementation-ready merely because they appear in the roadmap; their named gate must first be closed.

---

## 9. Agent-ready issue packs

These are the only packs approved for immediate source verification and issue authoring. Codex must still duplicate-search and re-read current `v4` before opening each issue.

### A1 — Finish source-aware work identity through admission and contributor state

**Provisional title:** `feat(convergence): carry source-aware WorkRef identity through admission and contributor state`

**Invariant**

Distinct work-source items never collapse onto the same task/admission/hold/cooldown/claim identity, while GitHub’s existing `repo#number` keys and persisted state remain backward-compatible.

**Current evidence**

- `pkg/worksource.Issue` and `TaskKey` exist.
- #4185 required identity-site migration, but #4189 only added the package/adapter.
- `ToGitHubIssues` drops `SourceType` and `ExternalID`; non-GitHub items become `Number == 0`.
- `convergence.Subject` and the legacy dependency observer remain GitHub numeric.
- Contributor state historically keys many maps and task IDs by repository plus integer number.

**Smallest vertical**

Carry one canonical source-aware work reference far enough that:

1. non-GitHub issues retain source identity through governor/scheduler and contributor candidate construction;
2. task/admission/active/hold/cooldown/claim identities use one helper;
3. GitHub keys remain byte-identical and existing persisted GitHub state remains readable;
4. the legacy bead observer explicitly handles only GitHub issue references rather than guessing that a string-keyed source is a GitHub `#0` issue.

The implementer must choose the smallest current-tree representation after tracing all call sites. Do not create a second parallel task identity type unless existing `worksource.Issue` cannot safely carry the contract.

**Strict RED matrix**

- Two Linear/Jira items with `Number == 0` and different `ExternalID` never share admission, cooldown, hold, claim, or active-task state.
- The same external key mapped to different repositories remains distinct.
- GitHub issue keys remain exactly `repo#number`.
- Existing persisted GitHub claim/cooldown/assignment state still loads and behaves identically.
- A non-GitHub item appears in an agent/contributor prompt with its native external key, never `#0`.
- The GitHub-only bead observer neither matches nor invents a bead identity for an unsupported source.
- Production-path tests cover at least one real assignment/offer path, not only `TaskKey` unit tests.

**Non-goals**

- No Linear/Jira write-back.
- No new work-source adapter.
- No generic evidence provider.
- No cross-repository outcome aggregation.
- No change to convergence semantics beyond identity preservation.

**Dependency:** #3904 and landed `pkg/worksource` Phase 1.

---

### A2 — Surface convergence admission diagnostics without making blocked work offerable

**Provisional title:** `feat(convergence): expose blocked, unknown, and degraded admission status from the shared evaluator`

**Invariant**

Operators can see why a candidate is not admitted, using the same decision that gates queue/assignment, without representing blocked work as ready or introducing a second evaluator.

**Current evidence**

- `Decision` already carries reason, blockers, observed record/generation, and conditions.
- `ReadyQueue` drops non-admitted candidates.
- Partial coverage currently produces logs, not durable/readable per-candidate status.
- `Converged`, human/external blockers, and quiescence are not yet computed and must not be fabricated.

**Smallest vertical**

Add a read-only projection or endpoint/status block for current candidate admission decisions. Its mechanism is open, but it must consume the same per-pass evaluator and distinguish:

- admitted;
- blocked (`Ready=False`);
- unknown (`Ready=Unknown`);
- reduced observer coverage / current legacy partial-view policy.

Expose only conditions the system actually computes today.

**Strict RED matrix**

- Unsatisfied dependency: not offerable/not assignable, diagnostic says blocked and names the dependency.
- Unresolved dependency: not offerable/not assignable, diagnostic says unknown rather than false.
- Unrelated candidate remains offerable/selectable and is not marked blocked.
- Partial legacy-ledger coverage is visible and does not claim full observation.
- Diagnostic and assignment cannot disagree for the same captured pass.
- Removing the shared evaluator call—or reimplementing its logic separately—fails a production-path parity test.
- No `Converged=True`, `HumanDecisionRequired`, or other uncomputed status is emitted.

**Non-goals**

- Do not put blocked candidates back into the offerable `ReadyQueue`.
- No dashboard redesign requirement.
- No durable outcome model.
- No exact-subject proof.
- No new planner/scheduler behavior.

**Dependency:** #3904. Can proceed independently of A1 if it remains GitHub-only; must compose cleanly with A1.

---

### A3 — Apply the shared admission judgment to internal agent dispatch

**Provisional title:** `feat(convergence): prevent dependency-blocked work from reaching internal agent kicks`

**Invariant**

A candidate withheld by convergence admission cannot be offered to a contributor while simultaneously being injected into an internal Hive agent kick.

**Current evidence**

- Contributor `ReadyQueue`/`selectTask` consume the shared evaluator.
- The governor/scheduler caches and formats `github.ActionableResult` independently.
- #3904 explicitly left agent dispatch at its own boundary.
- Work-source Phase 1 now feeds non-default source items through a legacy adapter.

**Smallest vertical**

Trace current enumeration → governor evaluation → `Scheduler.SetLastActionable` / `BuildKickMessages` and establish one contributor-neutral admission projection for internal issue lists. Preserve governor mode, cadence, role/lane routing, classification, and prioritization.

The implementation may share a normalized admitted candidate set or call the pure evaluator at the agent boundary. It must not create a second, semantically independent dependency gate.

**Strict RED matrix**

- A depends on unsatisfied B: A is absent from internal agent issue payloads and contributor offers/assignments.
- B becomes satisfied: A appears on the next bounded refresh/evaluation without restart.
- B becomes unsatisfied again: A disappears again.
- Unrelated C remains available to both execution families.
- Unknown dependency does not reach a mutating agent kick.
- No change to governor mode/cadence/routing for the same admitted population.
- Manual kicks using cached actionable state cannot bypass the gate.
- Source-aware items from A1 do not collapse to GitHub `#0` identities.

**Non-goals**

- No second scheduler.
- No new agent role or queue.
- No prompt redesign.
- No exact-subject proof or mutation lease.

**Dependency:** A1 unless current-tree tracing proves a GitHub-only vertical can land without creating a second identity migration. A2 is recommended for observability but not a hard dependency.

---

### A4 — Add a narrow, read-only shadow comparison harness

**Provisional title:** `test(convergence): shadow-compare Hive admission against Project Bluefin Review for one bounded work class`

**Invariant**

Hive can compare its current admission judgments with a known external operational reference without mutating, claiming, or double-dispatching work.

**Readiness rule**

Open this issue only after A2 and A3 land or after Sol proves the chosen shadow class does not require them. This is a validation harness, not a Review-specific runtime dependency.

**Smallest vertical**

Choose one narrow behavior already implemented by both systems—such as mandatory blocker suppression and lane-local unrelated readiness. Emit comparison records with exact source generations and disagreement reasons. Hive remains non-authoritative in shadow mode.

**Strict RED matrix**

- Shadow evaluation performs no GitHub mutation, task claim, or dispatch.
- Same observed snapshot produces deterministic comparison output.
- Disagreement preserves both judgments and evidence; it does not silently choose one.
- One blocked item does not hide unrelated comparison results.
- Restart reconstructs or recomputes from authoritative state; no in-memory-only verdict becomes truth.
- Review-specific logic stays outside `pkg/convergence` and scheduler policy.

**Non-goals**

- No authority transfer.
- No new Review queue.
- No special executor.
- No requirement to model every Review behavior.

**Dependencies:** A2 and A3 by default.

---

## 10. Gated design/research packs

Sol may prepare a decision memo or targeted research task for these. It must not present an implementation issue as ready until the gate is closed.

### G1 / B1 — Canonical outcome identity and desired-generation ownership

**Status:** REQUIRES MAINTAINER/PRODUCT DECISION.

Need one explicit answer:

> Which current durable authority owns `Project → RepositoryIntent → Outcome` identity and the acceptance generation for the first real outcome vertical?

Candidates may include repo configuration, a typed durable ledger, GitHub-backed declarations, or a deliberately small new store. Beads must remain work/execution records rather than silently becoming canonical desired state.

A decision packet must compare migration, restart reconstruction, authority to mutate acceptance, multi-repository identity, and compatibility with source-neutral work items. Do not turn every conceptual noun into a persisted type.

After G1, B1 may establish:

- canonical `OutcomeRef`;
- desired generation;
- observed generation;
- minimal status persistence/projection;
- explicit acceptance-authority mutation rules.

### G2 / B2 — Exact-subject GitHub proof fingerprint

**Status:** REQUIRES TARGETED RESEARCH + CURRENT-TREE DESIGN.

Define the first narrow proof key before implementation. At minimum evaluate whether it must bind:

- outcome and predicate ID;
- desired/acceptance generation;
- candidate head SHA;
- base SHA or base-reference observation;
- workflow/check identity and relevant configuration/harness generation;
- review identity and reviewed SHA;
- producer/authority;
- result, provenance, and freshness.

The target vertical should be one GitHub candidate and one CI/review predicate. It must prove that acceptance for SHA X does not authorize repaired SHA Y and that base/input movement invalidates only dependent proof.

Do not research every provenance standard before choosing the first predicate.

### G3 — Failure-domain-aware observer authority

**Status:** REQUIRES TARGETED RESEARCH / DESIGN; current behavior is an explicit compromise.

The unresolved question is not “fail open or fail closed globally.” It is:

> How does Hive know which source is authoritative for a candidate’s intent so an outage blocks only candidates whose truth may live there?

The design must distinguish:

- candidate is intentionally unmanaged and has no intent record;
- authoritative record exists and is readable;
- authoritative source is known but unavailable;
- record was terminal and durably retired;
- source ownership itself is unknown.

Research should focus on partial knowledge, source/partition authority, and failure-domain-local admission. Output must be a small contract applicable to Hive’s sparse legacy bead coverage—not a generic distributed-systems survey.

### B3 — Non-monotonic conditions and selective invalidation

**Status:** BLOCKED BY B1/B2.

The first implementation must show an outcome becoming false while its historical work record remains closed, then selectively invalidate only judgments whose declared assumptions changed.

Do not implement “main moved, invalidate everything.”

### C1/C2 — Resource claims, capacity, mutation leases/fencing, and idempotent operations

**Status:** LATER; design-ready only after exact-subject/outcome identity exists.

Reuse current contributor task generations/leases as prior art, but do not conflate assignment identity with resource ownership.

A future parent packet should separate:

1. semantic resource claims and overlap (`paths`, APIs, schemas, workflows, artifacts, release channels, deployments);
2. bounded mutation capacity and waiting-state release;
3. lease + monotonic epoch fencing for authoritative mutation ownership;
4. operation journal/idempotency for crash-after-effect-before-ack;
5. reconciliation after uncertain external effects.

Cross-reference [#4002](https://github.com/kubestellar/hive/issues/4002). Queue-based handoff must reuse existing claim/ownership machinery rather than creating a parallel authority.

### C3 — Authority and anti-self-certification

**Status:** LATER; PARTIAL PRIOR ART EXISTS.

`pkg/intent` already has useful tier/evidence concepts. A convergence design must represent at least:

- `Implement`;
- `GenerateProof`;
- `ReviewSemantic`;
- `MutateAcceptance`;
- `MutateSecurityPolicy`;
- `HumanDecision`.

Different model vendors are not automatically independent authorities. The same worker must not weaken acceptance and then certify the weakened result unless policy explicitly grants both capabilities.

### D1/D2 — Discovery, providers, aggregation, and optional CRD

**Status:** DEFERRED.

Sequence only after the previous verticals:

1. discovery and explicit objective promotion;
2. additional evidence providers;
3. project/cross-repository condition references and cycle handling;
4. aggregation of `Converged`/quiescent status;
5. optional Kubernetes API/CRD adapter if deployment and interoperability evidence justify it.

No global project scheduler is implied.

---

## 11. Issue-authoring transaction protocol

For each pack, the controller must execute this transaction serially:

```text
read packet + #3845
        ↓
pin current v4 SHA
        ↓
re-read named source seams
        ↓
search issues/PRs for existing owner
        ↓
classify pack:
  READY | SUPERSEDED | DUPLICATE | BLOCKED | RESEARCH_REQUIRED
        ↓
if READY, produce Luna payload
        ↓
Luna drafts one issue only
        ↓
Sol source-verifies every claim and acceptance row
        ↓
Sol revises or rejects
        ↓
create issue
        ↓
record issue URL/number, exact source SHA, and dependencies
        ↓
reconcile packet state before next transaction
```

Never emit all issue bodies in one unreviewed batch.

### Required issue shape

Every created issue must include:

- parent #3845;
- exact `v4` source SHA used;
- invariant, not just a feature description;
- current production path with file/function evidence;
- precise gap after landed work;
- smallest end-to-end vertical;
- strict RED acceptance matrix with positive controls;
- restart/race/out-of-order/failure behavior where applicable;
- compatibility and migration behavior;
- explicit non-goals;
- dependency/blocking relationships;
- risk if implemented incorrectly;
- evidence limits and unresolved maintainer decisions;
- statement that #3845 remains the architectural parent.

### Stop conditions

Do not create an issue when:

- current source no longer has the claimed gap;
- an existing issue/PR already owns the same outcome;
- the pack depends on an unclosed decision/research gate;
- acceptance cannot be made falsifiable;
- the proposed issue would introduce a second planner/scheduler/admission authority;
- the implementation would require guessing a policy owner, authority, or failure behavior;
- the only justification is a stale report recommendation contradicted by current code.

---

## 12. Sol/Luna execution contract

### Sol — controller and final authority

Sol owns:

- source pinning and freshness checks;
- issue/PR duplicate discovery;
- dependency graph and ordering;
- deciding which research sections are loaded;
- preparing a bounded payload for Luna;
- adversarial review of every issue;
- factual/source verification;
- final mutation to GitHub;
- checkpointing created issue IDs and current source generation.

Sol must not delegate architecture decisions, issue creation, or final acceptance to Luna.

### Luna Light — bounded issue writer

Luna receives only:

- one pack from this document;
- the exact current source excerpts/paths Sol selected;
- relevant #3845 paragraphs;
- at most the specific old-research excerpts named by the pack;
- existing related issue/PR summaries;
- the required issue template and non-goals.

Luna’s task is to produce a concise, complete candidate issue body. Luna must not:

- browse broadly for a new architecture;
- read both full research reports;
- create or update GitHub issues;
- expand scope beyond the assigned pack;
- resolve a maintainer decision by assumption;
- claim a source fact without a current path/function/commit reference.

Default to one Luna worker at a time because these packs share evolving source and dependencies. Parallelize only genuinely independent read-only drafts, and still serialize Sol’s acceptance and GitHub mutation.

### Escalation

If Luna cannot produce a falsifiable acceptance matrix from the payload, Sol should not compensate by inventing details. Reclassify the pack as `RESEARCH_REQUIRED` or enrich the payload from current source. Use a stronger bounded worker only when the issue requires substantial cross-file reasoning; Sol remains final authority.

---

## 13. Selective research-loading map

Do not preload the full reports. Load only these themes when needed:

| Pack | Research themes to load |
|---|---|
| A1 | Provider-neutral identity and cross-repository identity only; current `pkg/worksource` source is primary. |
| A2 | Tri-state conditions, blocked vs unknown, converged vs quiescent; do not load proof/resource-claim sections. |
| A3 | Admission boundary and planning/convergence/scheduling separation; current scheduler path is primary. |
| A4 | Project Bluefin Review behavioral reference and shadow-mode section only. |
| G1/B1 | Outcome vs transition, canonical intent, generation/authority, persistence cautions. |
| G2/B2 | Proof identity, exact subject/input assumptions, producer/authority, freshness/provenance. |
| G3 | Partial observation, incomplete authority, outage/backoff semantics, candidate-local uncertainty. |
| C1/C2 | Claims/capacity, lease/fencing, idempotent effects, re-entry/handoff; include #2568/#4002 current code/history. |
| C3 | Authority classes and anti-self-certification; include current `pkg/intent`. |
| D1/D2 | Provider neutrality, project/cross-repo aggregation, cycle handling, CRD deferral. |

---

## 14. Current source ledger

Pinned baseline files:

- [`src/pkg/convergence/convergence.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/convergence/convergence.go)
- [`src/pkg/dashboard/contribute_admission.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/dashboard/contribute_admission.go)
- [`src/pkg/dashboard/contribute_admission_deps.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/dashboard/contribute_admission_deps.go)
- [`src/pkg/dashboard/contribute_sse.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/dashboard/contribute_sse.go)
- [`src/pkg/dashboard/contribute_ws.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/dashboard/contribute_ws.go)
- [`src/pkg/dashboard/deps.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/dashboard/deps.go)
- [`src/pkg/beads/beads.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/beads/beads.go)
- [`src/pkg/scheduler/scheduler.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/scheduler/scheduler.go)
- [`src/pkg/worksource/worksource.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/worksource/worksource.go)
- [`src/pkg/worksource/adapt.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/worksource/adapt.go)
- [`src/pkg/intent/intent.go`](https://github.com/kubestellar/hive/blob/380a4ad80cfe6ab65e85b63e84745dfeae545463/src/pkg/intent/intent.go)

Historical evidence:

- [#3845 canonical architecture](https://github.com/kubestellar/hive/issues/3845)
- [#3857 shared contributor admission](https://github.com/kubestellar/hive/pull/3857)
- [#3904 dependency-aware admission and implementation review](https://github.com/kubestellar/hive/pull/3904)
- [#2568 contributor lease/generation prior art](https://github.com/kubestellar/hive/issues/2568)
- [#4002 re-entrant conversation/operation-journal prior art](https://github.com/kubestellar/hive/issues/4002)
- [#4178 work-source RFC](https://github.com/kubestellar/hive/issues/4178)
- [#4185 intended identity migration](https://github.com/kubestellar/hive/issues/4185)
- [#4189 actual WorkSource/TaskKey foundation](https://github.com/kubestellar/hive/pull/4189)
- [#4195 governor WorkSource wiring](https://github.com/kubestellar/hive/pull/4195)

---

## 15. Controller checkpoint table

Sol should maintain this table in its own working state while transacting. Do not modify this design packet merely to record transient draft progress unless the user explicitly asks for a persistent checkpoint commit.

| Pack | Initial classification | Issue | Source SHA | Notes |
|---|---|---|---|---|
| A1 source-aware WorkRef | READY FOR SOURCE RECHECK | — | `380a4ad` | #4185 closure does not prove migration landed. |
| A2 admission diagnostics | READY FOR SOURCE RECHECK | — | `380a4ad` | Must use shared evaluator; read-only. |
| A3 internal-agent parity | BLOCKED BY A1 BY DEFAULT | — | `380a4ad` | Sol may reorder only with source evidence. |
| A4 shadow validation | BLOCKED BY A2/A3 BY DEFAULT | — | `380a4ad` | Narrow behavior only. |
| G1/B1 outcome generation | DECISION REQUIRED | — | `380a4ad` | Canonical intent owner unresolved. |
| G2/B2 exact proof | RESEARCH REQUIRED | — | `380a4ad` | Define first proof fingerprint. |
| G3 observer authority | RESEARCH REQUIRED | — | `380a4ad` | Replace partial-view compromise with local authority semantics. |
| B3 invalidation | BLOCKED BY B1/B2 | — | `380a4ad` | No broad invalidation. |
| C1/C2 mutation ownership | LATER | — | `380a4ad` | Reuse #2568/#4002 prior art. |
| C3 authority | LATER | — | `380a4ad` | Reconcile with `pkg/intent`. |
| D1/D2 scale/adapters | DEFERRED | — | `380a4ad` | No CRD-first work. |
