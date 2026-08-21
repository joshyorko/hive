# Dogfood Mission: Local Hive for Actions + RCC and Convergence Validation

## Mission

Set up and operate one durable local Hive managing:

- `joshyorko/actions`
- `joshyorko/rcc`

This is both a real personal software factory and a maintainer-requested validation exercise for the convergence engine from `kubestellar/hive#3845`.

The goal is not merely to boot Hive. The goal is to reduce manual agent babysitting, exercise convergence against a real multi-repository backlog, collect fixed-commit shadow evidence, and repair any proven generic Hive gap in this fork before proposing it upstream.

Do the work. Do not merely narrate a plan.

## Authority and source discipline

- Work in `joshyorko/hive` on a dedicated dogfood branch created from this fork's `v4`.
- Configure `kubestellar/hive` as `upstream` and fetch current `upstream/v4` before implementation.
- Record the exact upstream and fork SHAs used for every deployment/soak.
- Do not use `v5` for this mission unless this document is explicitly updated later.
- Inspect current source before relying on assumptions in this file. If current `v4` materially contradicts an instruction here, preserve safety, document the contradiction, and follow the current source contract.
- Never hard-code Actions/RCC-specific behavior into a patch intended for upstream Hive.

## Local runtime

Run Hive locally without Kubernetes.

Preferred runtime:

1. If Docker Engine and Docker Compose v2 are already healthy, use Hive's supported standalone Docker Compose path.
2. Do not install, replace, or reconfigure system container infrastructure automatically if Docker is unavailable.
3. Do not disturb any Kubernetes cluster on this host.

Use one Hive for both repositories.

Required properties:

- authenticated Hive dashboard reachable on the host LAN through the normal gateway;
- normal host port `3001` unless occupied, in which case stop and report the conflict rather than killing another service;
- no public Internet exposure;
- no raw ttyd port exposure;
- no Watchtower/auto-update profile;
- no Docker socket mounted into the Hive container;
- no `docker compose down -v`;
- durable `/data` through Hive's supported named volume;
- persistent local `hive.yaml`, `.env`, and `secrets/` outside the disposable container;
- local backup and restore scripts before substantive automation begins.

Prefer a local checkout such as `${HOME}/services/hive-local` and a stable Compose project name such as `hive-local`.

After startup, print the LAN dashboard URL in the form:

`http://<LAN-IP>:3001`

Do not change the host firewall automatically.

## Persistence and migration safety

Treat the container/image as disposable and `/data` plus local config/secrets as durable state.

Create local operator scripts for:

- status/health;
- cold backup;
- restore;
- update/rebuild from a new `v4` or dogfood commit.

The cold backup must:

- identify the actual Hive Compose project and data volume instead of guessing;
- refuse ambiguity;
- stop Hive cleanly if running;
- archive the complete named volume;
- separately archive `hive.yaml`, `.env`, and `secrets/`;
- protect sensitive archives with restrictive permissions;
- restart Hive only if it was running before backup;
- never delete old backups automatically.

The restore path must require an explicit backup and destructive confirmation before overwriting non-empty live state.

Before every dogfood image replacement, perform a cold backup first.

## GitHub App

Use a GitHub App for repository authority, not a long-lived PAT for normal operation.

Install it only on:

- `joshyorko/actions`
- `joshyorko/rcc`

Read the exact current `v4` GitHub App documentation before setup and request only permissions Hive currently requires.

Prepare a local secret path for the PEM, e.g. `src/secrets/gh-app-key.pem`, with restrictive permissions. Never put the PEM, App private key, OAuth credential, or installation token in git, logs, issue bodies, PR bodies, or this document.

Use App-authored PRs and leave `project.ai_author` empty when current `v4` supports App-bot authorship that way.

Do not require `/gh-setup` to be publicly reachable for this LAN-only deployment. Manual App ID / installation ID / slug / key-file configuration is acceptable.

If a human browser step is required to create/install the App, complete every safe local step first, then stop with one concise checklist of the exact human actions still required.

## Codex backend

Use the OpenAI Codex CLI as the backend for enabled Hive agents.

Verify the current `v4` implementation before configuring it, including current image pin, per-agent `CODEX_HOME`, and any shared persistent authentication mechanism.

Use subscription/device login rather than an OpenAI API key unless explicitly directed otherwise.

One successful login should be reused by the enabled Codex agents if current source supports that architecture. Verify the auth survives a normal Hive container recreation.

Do not leave incompatible Claude/Copilot model strings attached to Codex agents. Use current Hive model discovery or a safe current default; do not invent model identifiers from memory.

## Initial Hive project

Configure one project that manages both repositories:

```yaml
project:
  org: joshyorko
  repos:
    - actions
    - rcc
  primary_repo: actions
```

Use each repository's existing `AGENTS.md` and repository-local skills as authoritative repo guidance. Verify the running Hive actually injects that guidance into agent work.

Do not create a second custom policy repository unless current source requires one for basic operation.

## Initial autonomy and safety

Start at ACMM L5, not L6.

Before applying L5, verify current `v4` semantics from source. The intended posture is:

- agents can create PRs;
- PRs remain human/hold-gated;
- no autonomous merge;
- no L6;
- `ioscan` remains enabled;
- automatic issue-label planning remains disabled for this first deployment;
- no aggressive custom cadence during setup.

Do not weaken security, hold, App, merge, or agent-mode controls to make setup easier.

## Temporary readiness gate

Create the `hive-ready` label in both managed repositories if absent.

Configure Hive's issue allow-list so only `hive-ready` issues may enter the actionable set.

Do not blindly label every open issue.

Perform a deterministic backlog readiness sweep across `joshyorko/actions` and `joshyorko/rcc` and classify every open issue into at least:

- `READY`
- `BLOCKED`
- `ACTIVE_WORK_EXISTS`
- `UMBRELLA_OR_PLANNING`
- `PARKED`
- `HUMAN_DECISION_REQUIRED`
- `UNKNOWN`

Apply `hive-ready` only when all of the following are supported by current evidence:

- the issue is an independently executable work outcome;
- it is not explicitly parked/held;
- no active implementation PR already owns the same work;
- known hard dependencies are satisfied;
- it is not merely an umbrella/master ledger;
- it does not require an unresolved human decision.

Produce both human-readable and machine-readable readiness reports with evidence for each classification.

This `hive-ready` frontier is the temporary safety/admission bridge while native dependency observation is validated.

## Convergence baseline

Start with:

```yaml
convergence:
  mode: shadow
```

Do not start in enforce.

Verify current `v4` behavior directly from source and the running Hive. Exercise the pieces that actually exist on this line, including the enrolled admission diagnostics/soak path and any outcome/proof/mutation surfaces that are wired into runtime.

Maintain a dogfood report in this fork, e.g.:

`docs/dogfood/convergence-soak-2026-08.md`

For every soak window record:

- exact Hive commit;
- date/time window;
- repositories;
- raw candidate count;
- admitted count;
- blocked count;
- unknown count;
- partial-source/ledger state;
- `would_differ`;
- decision latency;
- notable decisions;
- suspected false positives/false negatives;
- restart/reconstruction observations;
- operator interpretation.

Do not combine telemetry from different Hive commits as if it were one fixed-commit soak.

## Existing Actions/RCC dependency graph: prove before fixing

Audit the actual open issue bodies in both repositories.

Find real examples of structured relationship language, including patterns such as:

- `Depends on: #123`
- `Depends on: joshyorko/rcc#120`
- `Blocked by: ...`
- `Parent: ...`
- `Related: ...`

Do not assume those phrases have identical semantics.

For the first dependency interpretation hypothesis:

Hard dependency candidates:

- `Depends on`
- `Blocked by`

Non-blocking metadata unless evidence proves otherwise:

- `Parent`
- `Related`
- prose mentions;
- arbitrary issue links.

Support same-repo and cross-repo references only after the exact grammar is covered by tests.

Test the current unmodified convergence behavior first. Specifically capture a RED case if one exists where:

1. issue A explicitly declares a hard dependency on issue B;
2. issue B is incomplete;
3. issue A is otherwise actionable/enrolled;
4. current Hive nevertheless observes/adjudicates A as ready because the source relationship is not represented in the dependency observation used by convergence.

Do not implement a fix merely because this document predicts that gap. Prove it on current source and the real dogfood backlog first.

## If the gap is proven: implement the generic Hive capability in this fork

Once a reproducible RED case exists, implement the smallest generic architectural repair in `joshyorko/hive`.

Preserve #3845's separation:

`source observation -> convergence judgment -> admission -> existing scheduler/routing`

Do not put GitHub parsing into the pure convergence evaluator.

Do not create synthetic/shadow beads merely to give convergence something to query unless source analysis proves that is the intended canonical architecture.

Investigate the existing source/runtime-neutral seams first, including current `worksource`, forge/GitHub source representations, admission observers, and convergence observation interfaces.

Prefer a source-observation capability that normalizes dependency evidence before the pure evaluator rather than a GitHub-specific rule embedded in convergence.

The upstreamable implementation must be generic and must not contain `joshyorko/actions` or `joshyorko/rcc` special cases.

## Authority model for source-declared dependencies

Treat issue prose as untrusted external text, not automatically authoritative desired state.

The feature must be explicit/opt-in.

For this dogfood deployment, `hive-ready` may serve as the operator enrollment boundary for interpreting structured dependency declarations from issue bodies, if that fits current architecture.

Test what happens if an untrusted issue author edits dependency text after a maintainer/operator has enrolled the issue.

Do not allow dependency text to create an unbounded denial-of-service or arbitrary cross-repository traversal.

Only configured/allowed repositories may participate unless the operator explicitly expands authority.

Security-sensitive reports must still follow existing Hive disclosure/security handling.

## Required dependency semantics/tests

At minimum cover:

1. same-repo dependency;
2. cross-repo dependency;
3. multiple dependencies;
4. closed dependency becomes satisfied;
5. reopened dependency becomes unsatisfied again without restart;
6. missing dependency -> `UNKNOWN`, never silently satisfied;
7. inaccessible dependency -> `UNKNOWN`;
8. malformed explicit declaration does not silently become satisfied;
9. duplicate references deduplicate;
10. self-dependency is diagnosed;
11. cycles are diagnosed and cannot spin;
12. `Parent`/`Related` do not block;
13. dependency targets outside the configured authority set are not followed arbitrarily;
14. restart reconstructs the same result from authoritative source state;
15. shadow and enforce consume the same normalized observation/evaluator result;
16. existing bead dependency behavior remains compatible;
17. precedence between canonical bead-declared dependencies and source-derived dependencies is explicit, deterministic, tested, and documented.

Do not guess precedence. Derive the least-surprising rule from current architecture and explain it.

## Freshness and level-triggering

Dependency observation must be level-triggered.

A dependency closing or reopening, or the explicit dependency declaration changing, must be reflected without restarting Hive.

Do not use process-local memory as durable truth.

Use authoritative source state and a suitable source generation/freshness marker if current types expose one.

Avoid one GitHub API call per candidate per scheduling decision. Reuse/batch current enumeration/worksource data where possible and measure decision-latency/API impact during dogfood.

## Existing PR ownership and `Progresses`

Audit open Actions/RCC implementation PRs and their issue-reference language.

Compare usages such as `Progresses #N` with the current Hive claim parser.

Do not broaden claim semantics as an accidental side effect of dependency work.

If `Progresses` is independently worth supporting as a weak work-reference keyword, make that a separate focused commit with separate tests and document exactly which queues/guards honor weak claims.

## Dogfood any patch before upstreaming

After the generic patch passes tests:

1. cold-backup the local Hive;
2. build the local Hive image from the exact dogfood commit;
3. deploy without deleting `/data`;
4. keep convergence in shadow initially;
5. reproduce the exact original RED cases;
6. prove the corrected dependency decisions;
7. verify unrelated issues remain admissible;
8. verify both Actions and RCC still work;
9. verify restart reconstruction;
10. record the exact commit and telemetry in the soak report.

Hold the Hive on a fixed commit long enough to gather meaningful evidence. Do not continuously rebuild during a fixed-commit soak.

## Enforce graduation

Do not move to `enforce` just because unit/integration tests pass.

Recommend/perform the transition only after reviewing fixed-commit shadow evidence showing:

- decisions are consistently sensible;
- no unexpected candidate population disappears;
- `UNKNOWN` behavior is understood;
- partial-source behavior is visible;
- decision latency is acceptable;
- restart behavior is correct.

Preserve the live `off` escape hatch.

## Upstream result

If the source-dependency gap is confirmed and the fork implementation survives dogfood, prepare an upstream issue against `kubestellar/hive` and a focused PR targeting current `v4`.

The issue should contain:

- observed dogfood problem;
- exact source commit;
- concrete sanitized examples;
- current behavior;
- desired behavior;
- architecture seam chosen;
- authority model;
- generation/freshness model;
- cross-repo semantics;
- RED tests;
- compatibility/non-goals;
- performance evidence;
- fixed-commit shadow soak evidence.

The PR must not contain local Hive configuration, credentials, personal machine paths, or Actions/RCC-specific hardcoding.

Do not merge the upstream PR yourself.

## Completion criteria

This mission is successful when:

1. one durable local Hive operates both Actions and RCC;
2. the dashboard is reachable on the LAN;
3. GitHub App auth and Codex backend auth are durable;
4. ACMM L5 is hold-gated and not auto-merging;
5. existing repository `AGENTS.md` guidance reaches agents;
6. the backlog has a defensible `hive-ready` frontier;
7. convergence shadow telemetry is being collected on fixed commits;
8. any cross-repo/source dependency gap is demonstrated rather than assumed;
9. if demonstrated, a generic tested implementation runs in this fork;
10. the patch survives real Actions/RCC workload and restart;
11. an upstream-quality issue/PR can be handed to the Hive maintainers with real evidence;
12. the local Hive materially reduces the need to manually babysit individual agent sessions.

At each stage, prefer evidence over assumption and preserve an audit trail of the exact source generation being evaluated.