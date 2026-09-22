# [RFC] Govern one external software-factory workflow through Hive's existing admission path

**Status:** Proposed; maintainer decision required before runtime changes. Requested disposition: hold for contract review, not immediate autonomous implementation.

**Scope:** One external execution integration and a removal-sensitive conformance pilot. Not a new planner, scheduler, factory platform, or general-purpose distributed transaction system.

**Research baseline:** `hivecommons/hive` `v6` at `e0e99f7b2d37f1344fcc8b001f59b0133ab660f6`, inspected September 22, 2026 UTC. This draft is stored on a personal branch based on that commit. **That does not select the upstream implementation branch.** Current [contribution policy][H14] routes structural/protocol changes through a v5 RFC and reserves v6 for dashboard-optional work. Maintainers should assign the release line explicitly.

## Problem and concrete use case

An operator wants Hive to admit one repository-maintenance task, have an independently managed Flue workflow perform bounded analysis and candidate generation, and return an immutable report or patch for Hive-controlled verification and optional publication. The workflow may use several internal agents, survive a restart, and wait on external work. Hive should not need to own its internal conversation or reproduce its planner.

The useful outcome is **less orchestration glue without weakening authority or evidence**. It is not another backend name or a larger number of completed tasks.

Hive's contributor protocol already provides an important external-worker boundary. The unresolved question is whether it can safely host an opaque, asynchronous workflow with durable native receipts, rather than whether a new `FactoryAdapter` package should exist. [Protocol][H1]; [assignment lifecycle][H3].

## Existing work this must not duplicate

[Archetypes RFC #7620][R1] and the [existing external-workload comment][R2] already raise the contract question. The [report-only campaign discussion][R3] separates inspection, evidence and publication. This proposal supplies a narrower **execution lifecycle, authority and conformance decision**; it does not rename or supersede those efforts.

Preserve [#3845][R4] convergence/admission boundaries, [#4256][R5] operation-journal invariants, [#4257][R6] authority-policy decisions, and [#4258][R7] provider/aggregation boundaries. Coordinate capability negotiation with [#6825][R10], completion evidence with [#7864][R11], and remote context with [#8033][R8]. A closed design issue is not evidence that every runtime path implements its contract.

Before opening new implementation work, re-check those issues and linked PRs. A valid disposition is to attach the decision and pilot to an existing owner, or conclude an existing integration already suffices. Do not create a competing epic merely to preserve this proposal's name.

## What the inspected code establishes—and what it does not

| Existing seam | Verified boundary | Consequence for this proposal |
|---|---|---|
| Contributor protocol 1.2 and capability declarations | Optional compatibility negotiation and self-reported runtime fit; declarations are not trust. [H1][H1] | Reuse transport and routing where appropriate. New restricted execution must negotiate its actual guarantees; legacy permissiveness is not its security model. |
| Persisted assignment leases | `taskLease`, `lookupLease`, and `loadLeases` preserve identity/task/generation ownership across reconnects. `saveLeasesLocked` uses unique temporary files and fsync, but reports persistence failures by logging rather than returning an issuance error. [H2][H2] | Reuse identities, but prove durable-before-accept semantics for the enrolled path. Assignment generation is not a remote write fence. |
| External effect boundary | `bootAdvisoryWith` really constructs a mutation boundary. Its adapter uses synthetic `repo@mutation` and `DesiredGeneration: 1`; executor mode helpers do not constitute outcome proof verification. [H8][H8], [H4][H4], [H5][H5] | Do not claim current remote calls already carry accepted outcome generation or verified proof. Explicitly map the chosen operation's identities and authority. |
| Mutation stores | The claim ledger has file locking and reload. The journal has an in-process mutex and whole-file temporary-write/rename persistence. `Reconcile` accepts an externally supplied observation. [H7][H7], [H6][H6] | Do not infer multi-process journal safety or automatic external reconciliation from the ledger's properties. Establish the selected storage/observer contract before reuse. |
| Proof/outcome machinery | The proof API is a specific GitHub exact-head predicate; the outcome package explicitly remains staged. [H19][H19], [H9][H9], [H11][H11] | No fabricated PR number, invented producer, or hardcoded generation to make a report fit. A report-only pilot need not activate the entire outcome ledger. |
| Task MCP P1 | `authorizeTaskMCP` uses dashboard authentication and the provider declares `HubLaunchedOnly`. [H12][H12] | Do not distribute dashboard credentials to an external factory. Use a bounded context bundle first, or the separately accepted task-scoped authorization contract. |
| Sandboxed generation and publishing | `SandboxExecutor` separates workload execution from the credentialed push/PR path. [H17][H17], [H18][H18] | Prefer this separation over granting an external orchestrator general repository writes. Confirm the broker's selected operations independently; its existence is not a blanket guarantee. |

These are source-level findings, not a claim that a production deployment has been exploited or that the proposed integration was executed. The [accompanying audit](https://github.com/joshyorko/hive/blob/research/factory-execution-contract-v6/docs/research/factory-execution-2026-09-22/AUDIT.md) distinguishes direct observations from hypotheses requiring reproduction.

## Decision requested

Choose the smallest of these options for **one named workflow**:

1. **Existing local sandbox/launch-command integration.** Appropriate when the requirement is merely to run a different local workflow. Prove it and document it; no new remote protocol is required.
2. **Existing contributor protocol plus a durable external-execution binding.** Reuse Hive admission, identity, capacity and revocation; map one external engine's native start/receipt/observe/cancel behavior. This is the recommended candidate for the asynchronous use case above, not a preselected package layout.
3. **A standards-based remote service binding.** Evaluate A2A or Nexus only where its implemented semantics reduce work. No mandatory Temporal deployment, new broker, CRD, or second scheduling system merely to obtain familiar method names.

Record the selected owner and integration path, upstream branch, public API surface, store, workload, authority model, failure assumptions, and rejected alternatives. Reusing the current seam with a documented limitation is preferable to inventing a universal framework.

## First vertical: generation without publication authority

Recommended pilot: **one externally hosted, multi-stage Flue workflow that accepts a pinned source/context bundle and returns a bounded report plus an optional patch artifact.** The host is outside the Hive process, but can be a second local test process. It must not receive target-repository write credentials, a Hive dashboard token, production deployment credentials, or an unrestricted publication tool.

The pilot ends with Hive accepting or rejecting the report/candidate under an independently configured predicate. It does not merge, release, or publicly file findings. A follow-on may enable exactly one trusted publication effect after maintainers accept its separate boundary and tests.

Use Flue's actual native receipt/idempotency features rather than emulate them. The inspected [`enqueueDispatch`][F1] derives stable submission identities for keyed requests; [`AgentPromptOptions` and `AgentSendResult`][F2] expose `idempotencyKey`, incarnation `uid`, submission identity and deduplication. Pin the selected release/commit and test the precise public entry point: different payload under the same key must not be treated as the same work. Native admission idempotency does not make arbitrary external effects exactly-once.

The existing Astro triage Action is **not** this restricted pilot unchanged: its handler owns preview publishing and other GitHub lifecycle behavior. Reuse a bounded generation workflow, not a competing whole issue controller. [Triage handler][F4].

### Authority split

- **Hive:** global work admission, accepted scope, task ownership, resource policy, current authorization and the decision to accept or publish a candidate.
- **External workflow:** its internal stages, tools and checkpoints within that scope. It may not autonomously widen repository scope, weaken acceptance criteria, or dispatch unrelated Hive work.
- **Verifier/publisher:** an explicitly authorized component under the selected policy. A factory's result is a claim; receipt authenticity and schema validity do not establish semantic correctness.

Local candidate writes are distinct from target publication. Executing returned tests or applying a patch must itself occur in an isolated environment without publisher credentials. Do not move untrusted build hooks into the credentialed broker.

## Required contract, with existing identities preserved

These are semantic requirements, not a frozen new Go interface.

### A. Identity and acceptance

Keep separate: canonical work key; assignment ID and `TaskGen`; accepted scope/contract revision; logical execution key; remote instance incarnation and submission/run ID; and, only when an external mutation is permitted, that mutation's authority/epoch.

`TaskGen` must not be reused as desired-outcome generation or as proof that a GitHub token is revoked. Reassignment of the *same* logical execution should adopt/observe the same native run, not silently create another. Changed inputs or accepted scope require an explicit new execution identity. A caller retry and a deliberate new attempt after a terminal failure are different operations.

Persist admission intent, request digest, selected engine/workflow version and authority binding before dispatch. Recover the ambiguous start-ack window using native keyed admission or a reliable lookup. Pin the external incarnation as well as its friendly name; deletion/recreation must not silently attach to unrelated work. Define key retention and what happens after receipts are purged.

### B. Lifecycle and recovery

Support accepted/running/waiting/terminal observations, bounded polling or event resubscription, explicit failure and unknown states, and cancellation requests. A transport error is not task failure; a missing lookup is not proof that a delayed start or publication cannot still occur.

Cancellation requested, cancellation acknowledged, execution stopped, and publication authority revoked are separate facts. When stop cannot be established, keep the operation visibly uncertain and disable its acceptance/publication path. A stale artifact may be retained for diagnosis without being promoted.

Ordinary progress or a heartbeat must not renew authority indefinitely. Specify maximum run age, observation freshness, retry budget and orphan-recovery policy. Bound costs even for read-only generation. Do not consume repository mutation slots while merely waiting for a remote result; do account separately for outstanding remote compute.

### C. Receipts and verification

The result must bind engine/workflow version, remote run identity, logical execution key, immutable input revision, contract revision, immutable output digest, result class, timestamps, and bounded provenance. Use a versioned schema and reject missing, malformed or conflicting bindings. Evidence should identify producer and predicate; reported model names are metadata, not independent authority.

Verify downloaded bytes against the receipt. Apply size, archive/path and endpoint restrictions before processing artifacts. No arbitrary URL fetch with privileged network access. Empty/no-change and blocked results need their own semantics: neither is automatically successful remediation.

Acceptance is a second decision against **current** authorization and the **exact** artifact. Define expected base movement versus material assumption change; do not invalidate every run merely because unrelated main-branch work landed. A successful invocation, a correct candidate, an authorized publication and a satisfied repository outcome remain different statuses.

### D. Publication boundary, when separately enabled

Choose one mutation with a known conditional-write/idempotency/reconciliation mechanism. Every writer for that target must pass through the chosen authority boundary; a local preflight epoch check does not fence a remote writer that has independent credentials.

After an uncertain effect, query authoritative external state before retry. Negative evidence permits retry only when prior attempts cannot still apply or the target's idempotency/CAS mechanism absorbs the race. Replays must restore the full typed result expected by callers, not only return nil error with a provenance string. Preserve existing exact-head and PR-deduplication guards.

### E. Compatibility and rollout

Default off for the new binding. Preserve legacy contributors, existing scheduling, queue hold behavior, source-aware identities and human controls. An opted-in peer missing a required new capability must be refused rather than silently downgraded to the legacy path.

An observation-only shadow phase must not dispatch a second external run or perform an external mutation. Test actual behavior; neither a flag named `shadow` nor imported packages establish that property. Do not piggyback on the current convergence default without a deliberate enrollment decision.

## Conformance matrix

Exercise the real selected Hive admission/assignment boundary, the actual adapter, the real external runtime with a deterministic test workload, and the chosen persistence/verification path. A mock-only interface test is insufficient. No paid model or production credential is required for this protocol pilot.

| Scenario | Required observable result |
|---|---|
| Admitted task and valid immutable report | One accepted native run; report accepted for exactly the bound task/scope/input. An unrelated admitted task also succeeds. |
| Held/blocked task | No external start. Unrelated ready work remains executable. |
| Capability missing, unsupported version, or attempted downgrade | Enrolled path refuses with a reason; unchanged legacy clients remain compatible. |
| Same execution key and same payload delivered twice | Same native submission/result; no second billable run. |
| Same key with changed payload or recreated engine instance | Conflict or explicit new authorized identity, never silent adoption. |
| Process dies before start, after remote acceptance before receipt persistence, or after receipt persistence | Recovery resolves each window without blind duplicate dispatch. Missing durable admission prevents issuing new authority. |
| Assignment owner changes while external run continues | Replacement observes/adopts the existing logical run or records uncertainty; old messages cannot settle the replacement. |
| Cancel delivered but workload ignores it; success races cancellation | No false stopped state. Current acceptance/publication policy decides; late output cannot resurrect revoked authority. |
| Poll/stream outage, duplicate or out-of-order events | Current authoritative state eventually wins; local timeout alone cannot invent remote failure or completion. |
| Artifact missing, truncated, malicious path, wrong digest, or unavailable store | No acceptance or privileged execution; affected run is failed/unknown as defined, unrelated work remains available. |
| Exact input, contract revision or output subject changes | Old receipt cannot authorize the new subject. Expected unrelated base movement does not invalidate everything. |
| Factory says success but verifier rejects | Retain the execution fact; refuse candidate acceptance. No unearned trust/promotion or misleading completion suppression. |
| Remote task waits for a human or reaches cost/time cap | Waiting/blocked is visible; admission/compute/mutation capacity are accounted separately; no infinite heartbeat-based renewal. |
| Store write fails, record is corrupt, or supported process overlap occurs | No unpersisted authority becomes active. Demonstrate the actual selected store's behavior; do not assume atomic rename equals multi-writer safety. |
| Replay of settled result | Same typed output and provenance are recoverable without repeating the effect. |
| New binding disabled or shadow-only | No external starts or writes; existing paths preserve their documented behavior. |
| Guard or binding removed | A production-path negative test fails; disjoint positive controls prevent a reject-everything implementation. |

For any later publication pilot add crash-after-effect/before-ack, delayed old writer, ambiguous negative lookup, grant revocation, and exact-target conditional-write tests **before** granting repository write credentials. Reuse or extend the existing contributor-lease formal model where it fits; state its abstractions and do not claim model checking proves the whole implementation.

## Deliverables and stopping rules

**Gate 0 — bounded decision, not implementation by assumption.** Re-pin live code and existing issues/PRs. Produce a disposition against #7620/#8033 and a short decision record selecting the one workflow, existing integration seam, authority model, durable owner, acceptance predicate and release line. Include a source-to-guarantee matrix and a native Flue receipt/abort probe. Stop for maintainer acceptance if the selection is not already authorized. Do not build three alternatives or create a new distributed store while waiting.

**Gate 1 — only the accepted restricted pilot.** Implement the one selected binding and deterministic multi-stage fixture, with no remote publication credentials. Ship conformance tests, operational status/recovery documentation, a runnable example and a small comparison against running the same workflow through today's local integration. Record dispatch count, recovery behavior, repeated work, operator intervention and resource accounting—not issue-count reduction as a success proxy.

**Gate 2 — separate decision.** Enabling publication, generalized outcome aggregation, or a second executor requires a new accepted slice with its own evidence. Gate 1 is valuable without Gate 2.

Split implementation only after Gate 0 identifies genuinely independent owned paths. No parallel writers on the contributor protocol/store/boot wiring without explicit file ownership. Publish test evidence against the exact candidate SHA; a docs assertion or green helper test does not close the contract.

## Prior art to reuse instead of rebuilding

- **Flue:** keyed admission, incarnation checks and durable submission receipts are already exposed. Reuse those exact contracts. [F1][F1], [F2][F2].
- **Nexus / Temporal:** async operation tokens, request identity, lifecycle and cancellation semantics are a useful reference. The SDK explicitly distinguishes cancellation delivery from termination; handler retries still require idempotency. Borrow the contract without imposing the service. [SDK options][N1], [SDK handler][N2], [execution semantics][N3].
- **Tekton CustomRun:** separates requested operation state from observed completion and makes timeout/cancel support the external controller's responsibility. Borrow the ownership contract, not a CRD-first rewrite. [Types][T1], [controller obligations][T2].
- **A2A:** evaluate task/artifact interoperability and capability negotiation before inventing public transport. Base A2A does not supply Hive-specific admission, target fencing or predicate authority. Pin a concrete spec/SDK version and test idempotency rather than infer it from a message ID. [Specification][A1].
- **GitHub Agentic Workflows safe outputs:** useful precedent for unprivileged generation plus constrained, credentialed publication. Its handler configuration already expresses repository/path/patch limits. [Architecture][G1], [handler builders][G2].
- **in-toto:** reuse immutable subject digests and typed predicates where appropriate; signing is not semantic verification. Full attestation distribution is not required for a local report-only pilot. [Statement v1][I1].

## Explicit non-goals / nice-to-haves

No new global planner, scheduler, event bus, worker marketplace, universal capability taxonomy, mandatory MCP gateway, provider purchasing, autonomous benchmark routing, cross-factory model-context migration, complete outcome-ledger activation, fleet-wide HA redesign, all GitHub mutation coverage, or automatic publication of security findings.

Factory discovery, multiple transports, richer dashboards, signed cross-organization attestations and multi-repository campaigns may be useful later. None should hide the first question: **can one existing external workflow be admitted, recovered, revoked and verified through Hive without adding another authority?**


[A1]: https://a2a-protocol.org/latest/specification/
[F1]: https://github.com/withastro/flue/blob/c5a2a725fe1d93209ed294cca90af97060f6f2e2/packages/runtime/src/runtime/dispatch.ts
[F2]: https://github.com/withastro/flue/blob/c5a2a725fe1d93209ed294cca90af97060f6f2e2/packages/sdk/src/public/send.ts
[F4]: https://github.com/withastro/triagebot-action/blob/51d30da13c0bd571a3822fc56882de40fead0086/src/handlers/triage.ts
[G1]: https://github.github.io/gh-aw/reference/safe-outputs/
[G2]: https://github.com/github/gh-aw/blob/c3baa0c6728f05d52abe92b8e05e7264c3161faa/pkg/workflow/safe_outputs_handler_registry_pull_requests.go
[H1]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/dashboard/contribute_protocol.go
[H2]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/dashboard/contribute_leases.go
[H3]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/dashboard/contribute_ws.go
[H4]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/mutation/boundary.go
[H5]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/mutation/executor.go
[H6]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/mutation/journal.go
[H7]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/mutation/ledger.go
[H8]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/cmd/hive/main.go
[H9]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/proof/verify.go
[H11]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/outcome/doc.go
[H12]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/dashboard/task_mcp.go
[H14]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/CONTRIBUTING.md
[H17]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/agent/sandbox_executor.go
[H18]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/pushbroker/pushbroker.go
[H19]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/proof/proof.go
[I1]: https://github.com/in-toto/attestation/blob/main/spec/v1/statement.md
[N1]: https://github.com/nexus-rpc/sdk-go/blob/4a2c168174b43b2f609619a69f82eb1bb8c11bec/nexus/options.go
[N2]: https://github.com/nexus-rpc/sdk-go/blob/4a2c168174b43b2f609619a69f82eb1bb8c11bec/nexus/handler.go
[N3]: https://docs.temporal.io/nexus/operations
[R1]: https://github.com/hivecommons/hive/issues/7620
[R2]: https://github.com/hivecommons/hive/issues/7620#issuecomment-5751706247
[R3]: https://github.com/hivecommons/hive/issues/7620#issuecomment-5752038860
[R4]: https://github.com/hivecommons/hive/issues/3845
[R5]: https://github.com/hivecommons/hive/issues/4256
[R6]: https://github.com/hivecommons/hive/issues/4257
[R7]: https://github.com/hivecommons/hive/issues/4258
[R8]: https://github.com/hivecommons/hive/issues/8033
[R10]: https://github.com/hivecommons/hive/issues/6825
[R11]: https://github.com/hivecommons/hive/issues/7864
[T1]: https://github.com/tektoncd/pipeline/blob/1ede02cec7df95abd1c0cb54d7c22881a6844153/pkg/apis/pipeline/v1beta1/customrun_types.go
[T2]: https://tekton.dev/docs/pipelines/customruns/
