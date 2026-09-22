# External software-factory governance: adversarial source audit and prior art

**Prepared:** September 22, 2026 UTC (September 21 evening in America/New_York).
**Hive baseline:** `v6` commit `e0e99f7b2d37f1344fcc8b001f59b0133ab660f6`.
**Purpose:** test the “Hive only needs a thin factory adapter” thesis and support the adjacent `ISSUE.md`, not certify a deployment or announce a shipped integration.

## 1. Findings that change the proposal

The broad architectural direction remains plausible: external execution engines can sit below Hive's work admission and policy. The earlier characterization of the remaining work as almost a thin wrapper was too strong. Transport exists; guarantees must be traced through actual ownership, credentials, stores and target mutations.

The most important corrections are:

1. **Assignment fencing is not external-effect fencing.** A rejected `task_complete` protects Hive's accounting, not a remote API that still accepts the old worker's credentials.
2. **An imported proof package is not a deployed proof gate.** The inspected mutation executor uses proof mode helpers; that cannot establish that a live transition calls `Verify` and consumes its decision.
3. **The wired mutation adapter uses a synthetic outcome and generation 1.** Its event journal must not be presented as the complete accepted-outcome system.
4. **Durability differs among stores.** The mutation claim ledger has cross-process locking. Its neighboring journal does not inherit that protection. Contributor leases have stronger write/fsync mechanics, but issuance does not receive persistence errors from `saveLeasesLocked`.
5. **Task MCP P1 is not ready-made remote delegated authority.** The actual authentication and stated deployment scope matter more than the MCP method names.
6. **Flue already supplies more than the previous account credited.** Current keyed admission and incarnation checks can eliminate custom glue, provided the integration uses and tests them correctly.
7. **This is not an unclaimed conceptual gap.** Josh's existing #7620 comments already propose several of the relevant boundaries. A new issue must be a bounded, non-duplicating decision/pilot.
8. **Full generalized convergence is not a prerequisite to a useful pilot.** Requiring every staged subsystem up front would inflate the scope and delay a safe report-only execution path.

Source anchors for these corrections: [contributor protocol][H1], [leases][H2], [mutation adapter][H4], [executor][H5], [journal][H6], [claim ledger][H7], [boot wiring][H8], [proof verifier][H9], [task MCP][H12], [Flue dispatch][F1], [Flue public send][F2], [existing discussion][R2].

## 2. Method and evidence limits

The audit read pinned Hive implementation files and focused source ranges, followed actual boot construction and selected effect callers, read issue discussions, and inspected source in Flue, Astro's triage Action, the Nexus Go SDK, Tekton and GitHub Agentic Workflows. It also consulted A2A, in-toto and OpenHands primary material. This is not an exhaustive audit of every file or branch in every organization.

GitHub code search was useful for discovery but not reliable evidence of absence. In this investigation, searches did not surface the construction in the large `src/cmd/hive/main.go`; direct reads of `bootAdvisoryWith` did. The source proves that mutation wiring exists. An empty search result is not proof that a feature is missing.

No full Hive clone, Go test suite, live service, privileged mutation, remote factory run or fault-injection test was executed for this research. The available shell could not resolve `github.com` for a clone. Code was inspected through the authenticated GitHub connector. The requested deliverables are documentation, not runtime changes. Proposed tests below are acceptance requirements, not reported passes.

Labels used here:

- **Observed:** directly present in inspected source or an attributed primary document.
- **Inference:** follows from inspected paths but may depend on an unexamined caller/deployment condition.
- **Needs reproduction:** a concrete failure hypothesis that must be tested before calling it a confirmed defect.

The pinned baseline prevents silent source drift, but later work must re-fetch. Refer to symbols as well as paths; do not assume line numbers or old issue bodies remain current. External source snapshots are pinned where code was read; moving documentation links must be version-checked before implementation.

## 3. Hive: detailed guarantee audit

### 3.1 The contributor protocol is real prior art, not a universal executor contract

**Observed:** protocol 1.2 advertises named capabilities, including credentials-after-accept and capability routing. Client capability data is explicitly self-reported and not a trust signal. Assignment handling has server-issued generations; persisted leases identify the task, owning identity, source-aware key and expiry. Reconnection must match a server-issued record. These are valuable existing seams. [Protocol][H1]; [lease implementation][H2]; [wire/settlement path][H3].

**Inference:** a factory could implement this protocol, but its internal task/run cannot simply be equated with a socket session. It might continue after the relay dies, use multiple internal workers, or wait much longer than a coding-agent turn. The adapter needs an explicit durable mapping and a single ownership policy across those lifetimes.

**Gotchas:** backend/model display metadata is not factory authority. A legacy peer being permitted without declaring capabilities is compatible behavior, not adequate enrollment for a new restricted workflow. Task reassignment, remote-run adoption and publication authorization need separate checks. A model's internal tool-call ID is a poor cross-owner logical operation key.

### 3.2 Contributor lease persistence is implemented, with a weaker failure contract than “durable grant”

**Observed:** `saveLeasesLocked` uses a unique temporary file, restrictive mode, file fsync, rename and directory fsync. `loadLeases` restores unexpired records and advances the assignment counter past the restored maximum. That is materially better than process-only ownership. However, persistence methods log failures and return no error to `recordLeaseForKey`; unreadable state can result in an empty restored registry. [Lease persistence][H2].

**Needs reproduction:** a disk failure during issuance or revocation can leave the live view and restart view different. A new protocol promising durable-before-accept must prove the grant does not become externally authoritative before the necessary durable commit. Do not silently claim that promise from “there is a file.” Do not rewrite all contributor storage as a side project; isolate the selected path or require the existing owner to supply the missing contract.

### 3.3 Mutation wiring exists; outcome ownership is still synthetic at this adapter

**Observed:** `bootAdvisoryWith` opens the mutation ledger/journal and installs `mutation.Boundary` on the GitHub client. This is not merely unused library code. `Boundary.Execute` constructs `OutcomeKey = repo + "@mutation"` and `DesiredGeneration = 1`, then derives an effect from the mutation kind, target and inputs. [Composition root][H8]; [boundary][H4].

**Implication:** this can be useful operation deduplication without being the accepted outcome contract. The public effect seam exposes a callback and provenance result; it is not a remote start/get/cancel API or a validated binding from accepted outcome to artifact. [Effect seam][H15].

**Needs reproduction:** repeated, intentionally new applications of an identical effect after real-world drift may need a distinct accepted generation or operation intent. “This effect happened once” is history; “this desired condition is true now” is state. A factory integration must explicitly choose both identities rather than inherit a permanent dedup key by accident.

### 3.4 Epoch checks cannot revoke an independently credentialed remote writer

**Observed:** `Executor.Execute` validates a lease before effect execution and again before acknowledgment; an effect error becomes Unknown. `Boundary` releases its claim after the call. [Executor][H5]; [boundary][H4].

**Inference:** the local pre-check and the eventual remote write are not atomic. If a remote operation outlives the lease or a second controller, rejecting its later acknowledgment does not undo the write. Even perfectly durable local storage cannot solve that by itself.

A valid architecture either keeps target credentials in a trusted broker that enforces current authorization, uses target-supported conditional operations/idempotency with precisely bounded guarantees, or establishes a termination/fencing contract that prevents the old attempt acting later. For the first generation-only pilot, withholding target write credentials removes much of this risk; it does not remove compute cost, data exposure or malicious-artifact risk.

### 3.5 Claim-ledger serialization is not journal serialization

**Observed:** `Ledger.lockAndRefreshLocked` uses `flock` and reloads current file contents while locked. `Journal` instead holds its own in-process `sync.Mutex` over an in-memory snapshot; persistence rewrites the whole JSON via a fixed temporary path and rename, without the corresponding fsync/refresh protocol. [Ledger][H7]; [journal][H6].

**Needs reproduction:** two independently opened journals can each persist an old snapshot and lose the other's changes, even if their resource claims are disjoint. A fixed temporary path also needs careful concurrent-writer treatment. Claim mutual exclusion around one resource does not serialize every operation-file write across resources.

For a strictly single-owner process, cross-process journal writes may be outside the supported envelope. State that envelope and test takeover/rolling-deployment behavior. Do not advertise HA or cross-process exactly-once from component names; do not choose a new database before deciding which ownership mode the pilot actually requires.

### 3.6 An observation function exists; automatic authoritative recovery is not established

**Observed:** `Journal.Reconcile` consumes `ExternalState{Known, Applied, Result}` supplied by a caller. `Executor` returns a needs-reconciliation error for uncertain work. The inspected boot construction does not install an effect-specific observer into that adapter. [Journal][H6]; [executor][H5]; [boot][H8].

**Scope of conclusion:** the inspected boundary does not itself implement general authoritative recovery. This does not prove that no other caller performs recovery; it prevents claiming that every enrolled effect automatically does.

A negative lookup is especially dangerous: a delayed old request can still create the object after the new owner observes absence. Before retry, the design needs either a target-side stable dedup key, proof that the old execution cannot later apply, or a conservative Unknown state. Observation freshness and write exclusion are different properties.

### 3.7 Deduplicated acknowledgment must rehydrate the caller's typed result

**Observed:** the boundary handles `ErrAlreadyApplied` by returning stored provenance with no error and without running the callback. `CreatePR` and `MergePR` capture typed API response pointers inside that callback, then use them after `effects.Execute` returns. `CreatePR` also has an earlier open-PR/tree dedup path that often avoids this situation. [Boundary][H4]; [PR callers][H13].

**Needs reproduction:** a journal replay that skips the callback can leave callers without the populated typed result they expect. Do not claim a nil-pointer panic: SDK getters may be nil-safe and instead return zero-valued results. The acceptance requirement is a stable, fully reconstructed result on replay, including the required identity and status, not merely `err == nil`.

### 3.8 “Proof is wired” was too imprecise

**Observed:** the proof package contains `VerifyAgainst`, `Verify` and a conversion to an admission dependency. Its predicate/fingerprint is GitHub-specific. The mutation executor imports proof helpers for mode checks. The staged-package guard distinguishes package reachability, and only `outcome` remains in its explicitly unwired list. [Verifier][H9]; [proof model][H19]; [executor][H5]; [reachability test][H10].

**Correction:** package linkage does not prove a production observer creates current receipts and a dependent transition enforces their verdict. The integration must cite and test that specific path. No factory should be accepted merely because it returns a structure called Proof.

The outcome package explicitly remains staged; its existence does not demonstrate a generalized predicted-versus-observed loop. Conversely, activating that whole loop should not be forced into a small report-only adapter. Reuse exact-subject concepts without faking a PR or declaring a repository converged. [Outcome status][H11].

### 3.9 Shadow behavior requires tests, not trust in a name

**Observed:** the executor's shadow path still opens/begins journal entries and can return journal errors; its mode distinction changes epoch enforcement, not every possible behavioral effect. [Executor][H5].

**Needs reproduction:** repeated identical operations or broken persistence can therefore affect a caller even though the selected mode is shadow. This is not a claim that a deployed fleet is failing; it is why the proposed integration defines its own enrollment boundary and tests that observation-only mode causes no extra dispatch or mutation. A second factory run “just for comparison” is not read-only shadowing.

### 3.10 Task MCP is a context interface, not lifecycle or remote authorization

**Observed:** the P1 provider authenticates against the dashboard token, resolves active task context, and marks itself hub-launched-only. The inspected boot helper constructs the agent-facing MCP URL using dashboard authentication. [Task MCP][H12]; [boot helper][H8].

**Required boundary:** do not forward this privileged credential to an external service to make a demo work. Either send a minimal immutable context bundle or wait for the separately accepted task-scoped, revocable remote authorization path. Context may contain attacker-controlled issue text; provenance and data/instruction separation still matter. These are integration constraints, not a public exploit report.

### 3.11 Existing sandbox/broker separation is the lowest-risk leverage point

The existing `SandboxExecutor` prepares work, invokes a `sandbox.Launcher`, collects candidate output and then uses a trusted push path. A launch command can already choose a different program. This is a credible alternative to a new remote protocol when an operator only wants local Flue execution. A synchronous `Run(ctx, spec)` returning stdout/stderr/exit code is nevertheless different from a durable remote operation token. [Sandbox executor][H17]; [launcher][H16]; [broker][H18]; [manager][H22].

The right question is which existing boundary needs extension—not how many features a new factory SDK can collect. Inspect artifact admission and command execution separately: applying a patch, running tests, or importing a repository can execute attacker-controlled behavior even when the external worker has no publication token.

## 4. Prior art: what is already solved elsewhere

### Flue: native receipts, keyed admission and incarnation binding

The current source supplies a practical answer to the ambiguous-start problem. `enqueueDispatch` derives a keyed submission identity; unkeyed requests create a new identity. The public send options carry a bounded idempotency key and a conditional instance `uid`. The API describes a conflict for reusing a key with a different payload and returns submission ID, stream offset and deduplication metadata. A deliberately new attempt after a failed submission needs a new delivery key. [Dispatch implementation][F1]; [public API][F2].

**Borrow:** persist Hive's logical execution binding, use the native keyed request, and recover/read that exact submission. Test instance deletion/recreation and dedup-retention boundaries rather than assuming a friendly conversation name is an immutable identity.

The durability guide distinguishes database persistence, workspace persistence and external side effects. Node recovery depends on the configured durable store and a single live conversation owner; an ephemeral workspace is not rescued by a durable transcript. These are deployment-specific conformance requirements, not just framework features. [Durability guide][F3].

Do not lift Astro's whole Action unchanged. Its triage handler includes preview publishing, GitHub effects and project-specific package/main-branch assumptions. That is an opinionated factory, not a pure generation service. [Actual handler][F4].

### Nexus / Temporal: the closest lifecycle contract

The Nexus Go SDK defines start request identity, async operation tokens and callbacks. It explicitly makes cancellation asynchronous and allows the underlying implementation not to stop; an acknowledgment is delivery, not proof of termination. [Options][N1]; [handler interface][N2].

Temporal supplies reliable cross-service operation orchestration. Its documented base execution is at-least-once; workflow-backed duplicate rejection is bounded by workflow identity and retention, not a universal guarantee over third-party side effects. Terminating a caller can orphan external work rather than cancel it. [Temporal semantics][N3].

**Borrow:** durable handle, separate acceptance/settlement, bounded deadlines and honest cancellation state. **Do not borrow by default:** an additional mandatory workflow cluster or a second Hive scheduler. A small adapter can implement equivalent obligations using the selected executor's native mechanism.

### Tekton CustomRun: external ownership with explicit obligations

The typed API separates requested cancellation in `Spec` from observed completion, carries timeout/retry information and delegates execution-specific state to the external task controller. Tekton documentation requires that controller to implement timeout/cancel cleanup or reject unsupported requests. [Types][T1]; [controller contract][T2].

**Borrow:** explicit owner and supported-operation contract, stable identity, terminal-condition semantics and refusal of unsupported guarantees. **Do not assume:** deleting a Kubernetes object guarantees a remote build stopped, or that a CRD is required to give Hive these semantics.

### A2A: interoperable tasks/artifacts, not an autonomy governor

A2A provides task discovery and lifecycle operations, artifacts and transport/security extension points. It is worth evaluating before inventing an externally facing task protocol. But an interoperable task handle does not define Hive's accepted scope, publication authority, target-side fence or evidence predicate. Idempotent submission needs a tested implementation/profile; a message identifier alone is insufficient. [A2A specification][A1].

**Borrow:** versioned task/artifact exchange where an actual executor supports it. **Do not do:** require a broad new A2A service just to wrap one Flue endpoint. Pin the concrete spec and SDK; a moving `latest` page is research, not an implementation contract.

### GitHub Agentic Workflows: constrained publication rather than agent write authority

Safe outputs separate agent-generated structured requests from permission-controlled jobs that apply them. The inspected PR-handler builders carry per-handler credentials, allowed repositories, protected paths, patch-size limits and operation caps. [Architecture][G1]; [configuration source][G2].

**Borrow:** unprivileged generation, validated output requests, narrow trusted writers and a staged/dry-run mode. This aligns with Hive's sandbox/broker seam. It does not prove a patch correct or eliminate the need to re-check target identity and policy at publication time.

### OpenHands: control service separate from execution server

OpenHands Automation explicitly owns schedules, webhooks, run history, dispatch and sandbox orchestration; the Agent Server/SDK executes conversations. Its README labels the service beta. This is concrete prior art against treating control-plane/executor separation as unique to Hive. [Automation boundary][O1].

A source-search spot-check of `RemoteConversation` also shows attach-or-create behavior: a missing supplied conversation can trigger creation. That ergonomic SDK behavior must not be blindly used when an adapter promises “reattach only, never start new work.” This is a focused source observation, not a review of that entire implementation. [Remote-conversation path][O2].

**Borrow:** explicit repository/component ownership and a stable execution API. **Do not compose:** two independently scheduling automation controllers over the same global backlog without deciding which owns admission.

### in-toto: a standard statement shape, not a truth oracle

Statement v1 identifies immutable subjects with digests and distinguishes predicate types. The Go validation helper validates structural fields; it does not independently run the claimed checks or decide which producer Hive trusts. [Statement specification][I1]; [validator][I2].

**Borrow:** subject-by-digest and explicit predicate identity. **Optional initially:** cross-organization signatures and attestation distribution. A signed false claim is still a false claim; verification authority and acceptance policy remain separate.

## 5. Non-obvious failure scenarios the pilot should force

### Lost start acknowledgment

Hive records intent, the external engine accepts, and the response is lost. A restart must retry the same native idempotency key or recover the same handle. Generating a fresh run ID burns duplicate compute and can later publish twice. An execution key must survive assignment-owner replacement; an attempt counter belongs inside the record, not necessarily inside the logical identity.

### Late result after revocation

Hive revokes G, admits G+1, and the old factory returns a validly signed artifact for G. Preserve the artifact as evidence of what ran; do not accept it for G+1. Stopping its local relay does not prove a hosted workflow, detached child or queued publication stopped. A read-only factory can continue wasting compute even while publication is safely blocked.

### Negative lookup races a delayed writer

The new owner queries the target and finds no PR/artifact. The old request is still in flight. A retry justified only by absence can race it. “Not found” becomes “safe to retry” only under the selected target's idempotency/conditional-write or proven termination contract.

### Correct computation, wrong authority

A report or patch can be technically sound yet unauthorized because scope, acceptance policy, target or grant changed. Do not make all success states one enum. Run completion, candidate verification, publication and outcome satisfaction require separate evidence and may have different authorities.

### Durable transcript, missing filesystem

A recovered factory has the conversation but lost uncommitted source edits or the artifact. It must reconstruct or report missing output, not return the old completed prose. Persist/retrieve artifact bytes independently of conversational memory.

### Two engines independently solve the same work

An idempotency key handles transport replay of one operation. It does not deduplicate two semantically equivalent fixes, two independently discovered findings, or two PRs with different titles. Reuse existing work claims and duplicate detection; do not claim an operation journal solves semantic duplication.

### Publication-boundary TOCTOU

The broker checks one branch head or artifact digest, then a writer changes it before use. Fetch/read by immutable identity, validate the exact bytes used, and select a real conditional target operation where needed. A mutable URL with a digest displayed beside it is not sufficient.

### Retention, restore and incarnation reuse

Idempotency records may expire; database restore can resurrect pre-revocation authority; a service name may refer to a new deployment. Specify a maximum replay horizon, persistent tombstones or equivalent protection, and a recovery policy for restored state. Do not promise perpetual exactly-once on a finite-retention system.

### Nested retry amplification and budget exhaustion

Hive, relay, workflow runtime and model API can each retry. Multiply their budgets and an apparently small task can become expensive. Designate one owner per retry class, expose native attempts, cap total elapsed/resource use and distinguish remote waiting from active compute. Budget self-report is telemetry, not a hard cap unless an enforceable mechanism exists.

### Unsafe artifact intake and source-data leakage

No write credential does not make arbitrary external execution safe. Define repository-data permission, outbound destinations, secret handling, artifact size/digest/path policy and workspace isolation. Do not execute returned tests in the publisher's privileged process. Security findings require the project's private disclosure path, not automatic public filing.

## 6. Must-haves versus optional scope

| Must establish for the selected pilot | Defer unless the selected workload actually needs it |
|---|---|
| One admission owner, explicit enrollment and current authorization | Marketplace, fleet discovery or general factory taxonomy |
| Durable execution-to-native-run binding and unambiguous retries | A new workflow engine or universal distributed journal |
| Correct revoke/late-result behavior and orphan visibility | Seamless conversation migration across model vendors |
| Immutable artifact binding and a genuinely independent acceptance check | A catalog of every convergence predicate/provider |
| Bounded execution, observations and artifact intake | Performance-based autonomous provider purchasing |
| Explicit supported persistence/owner topology | Fleet-wide active-active HA redesign |
| Removal-sensitive conformance tests on the actual integration path | Rich dashboard, scoreboards and automatic platform comparisons |
| Legacy compatibility without enrolled-path downgrade | Multiple protocols or several executor implementations |

A useful small pilot does not need the entire staged outcome ledger. It does need an honest status that stops short of claiming repository convergence. An optional publication phase cannot begin until its exact authority and effect contract is tested.

## 7. Recommended agent handoff and acceptance evidence

The accompanying issue intentionally has two gates. The first produces an accepted contract and native-runtime probe, not a broad implementation. The second implements only the chosen restricted vertical. That is preferable to an “agent-ready” prompt that silently delegates unresolved authority decisions to the same agent building the feature.

A useful decision packet includes: current source SHAs; related issue/PR disposition; selected integration boundary and owner; one workflow and acceptance predicate; identity/retention rules; state transition table; auth and credential flow; storage/topology assumptions; crash/cancel/replay witnesses; compatibility strategy; exact test commands; and a named maintainer acceptance. “No new abstraction needed” is an acceptable evidence-backed result.

Measure the comparison against today's local/existing relay path: accepted native runs per logical request, duplicated compute, incorrect acceptance count in negative fixtures, restart recovery, orphan visibility and operator intervention. Do not use PR count, queue emptiness or self-reported completion as the only success metric.

## 8. Source map and further inspection before code changes

**Pinned implementation read in this investigation:** contributor protocol and lease code; mutation adapter/executor/journal/claim ledger; boot composition ranges; PR creation/merge callers; proof verification; staged-package guard; outcome status; task MCP provider; and contribution policy. Neighboring sandbox, broker, work-source, wire and proof-model paths were examined in the preceding investigation and are linked for revalidation before modification.

**External code read:** Flue dispatch and public send; Astro triage handler; Nexus options and handler; Tekton CustomRun types; GitHub Agentic Workflows PR handler builders. OpenHands remote-conversation behavior was a source-search spot-check, with repository-boundary documentation read separately. A2A and in-toto were specification/API-reference review, not full implementation audits.

Before implementation, trace the selected current dispatcher to launch, credential issuance, renewal/revoke, result intake, verifier and any publisher. Enumerate every bypassable writer for that target. Inspect supported failure paths rather than derive behavior from comments. Run the proposed conformance tests in a real checkout; this research has not done so.

## 9. Filing and branch mechanics

Andy closed #3828 because Josh could not edit the original issue body and wanted an issue he could own and revise. Andy explicitly asked for a fresh issue authored by `joshyorko`; he did not require a close/reopen trigger ritual. [First comment][R12]; [clarification][R13].

Post the new issue from Josh's own GitHub account. Link #7620 and let maintainers decide whether it is a separate bounded slice or belongs there. Request a hold/discussion disposition; do not treat historical approval on another issue as approval here. Posting an issue body cannot guarantee agents will ignore it, so obtain the appropriate label/triage state before treating it as held.

The personal documentation branch is based on the requested v6 commit. Upstream release policy still determines where actual code belongs. [Current contribution policy][H14]. No upstream issue, PR, release or workflow dispatch is part of this delivery.


[A1]: https://a2a-protocol.org/latest/specification/
[F1]: https://github.com/withastro/flue/blob/c5a2a725fe1d93209ed294cca90af97060f6f2e2/packages/runtime/src/runtime/dispatch.ts
[F2]: https://github.com/withastro/flue/blob/c5a2a725fe1d93209ed294cca90af97060f6f2e2/packages/sdk/src/public/send.ts
[F3]: https://flueframework.com/docs/guide/durability/
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
[H10]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/staged_packages_7281_test.go
[H11]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/outcome/doc.go
[H12]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/dashboard/task_mcp.go
[H13]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/github/pullrequest.go
[H14]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/CONTRIBUTING.md
[H15]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/effects/effects.go
[H16]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/sandbox/sandbox.go
[H17]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/agent/sandbox_executor.go
[H18]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/pushbroker/pushbroker.go
[H19]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/convergence/proof/proof.go
[H22]: https://github.com/hivecommons/hive/blob/e0e99f7b2d37f1344fcc8b001f59b0133ab660f6/src/pkg/agent/manager_sandbox.go
[I1]: https://github.com/in-toto/attestation/blob/main/spec/v1/statement.md
[I2]: https://github.com/in-toto/attestation/blob/main/go/v1/statement.go
[N1]: https://github.com/nexus-rpc/sdk-go/blob/4a2c168174b43b2f609619a69f82eb1bb8c11bec/nexus/options.go
[N2]: https://github.com/nexus-rpc/sdk-go/blob/4a2c168174b43b2f609619a69f82eb1bb8c11bec/nexus/handler.go
[N3]: https://docs.temporal.io/nexus/operations
[O1]: https://github.com/OpenHands/automation/blob/main/README.md
[O2]: https://github.com/OpenHands/software-agent-sdk/blob/9bc452ebf6b0093c531f25394c5ba5e9817ff910/openhands-sdk/openhands/sdk/conversation/impl/remote_conversation.py
[R2]: https://github.com/hivecommons/hive/issues/7620#issuecomment-5751706247
[R12]: https://github.com/hivecommons/hive/issues/3828#issuecomment-5294081066
[R13]: https://github.com/hivecommons/hive/issues/3828#issuecomment-5294084564
[T1]: https://github.com/tektoncd/pipeline/blob/1ede02cec7df95abd1c0cb54d7c22881a6844153/pkg/apis/pipeline/v1beta1/customrun_types.go
[T2]: https://tekton.dev/docs/pipelines/customruns/
