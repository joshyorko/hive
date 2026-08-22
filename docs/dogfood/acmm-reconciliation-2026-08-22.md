# Actions/RCC ACMM reconciliation — 2026-08-22

## Provenance

- Fresh upstream base: `kubestellar/hive:v4` at `18e8172b98be5663c95b3edd7da0fce068220313`.
- Public dogfood evidence inspected: `joshyorko/hive:dogfood/actions-rcc-2026-08-22` at `6c4226416ef2b37a51abc8e692e48fc2cc9a4e97`.
- Shareable implementation commit: `0240425a` (`fix(acmm): reconcile generated remediation issues`).
- Runtime integration commit used for the one-time reconciliation: `5060456ee038bd5aabe8d617f7d993d75d18b479`.
- Private security work was not read into, copied to, or published from this branch.

No open upstream issue, pull request, commit, or current-v4 source was found that already implemented criterion-keyed ACMM issue idempotency or reconciliation. Existing exact-title issue-request deduplication, advisory reconciliation, GitHub App attribution, and audit facilities were reused as design precedents.

## Findings and decisions

### Non-idempotent creation

`handleACMMCreateIssue` created a new issue from static criterion metadata without looking for an existing issue or checking current repository evidence. New issues now carry a machine-readable repository + criterion marker; legacy `**Criterion ID:**` bodies remain discoverable. Creation is serialized with reconciliation, reuses an existing active criterion issue, respects a human `not_planned` disposition, and is confined to configured repositories.

### Missing lifecycle

An owner-only `POST /api/acmm/reconcile` endpoint now performs fresh evaluation and classifies only Hive ACMM Evaluation issues as `satisfied`, `duplicate`, `still_failing`, `evaluator_gap`, or `human_dispositioned`. Dry-run is side-effect free. Apply closes only mechanically proven satisfied issues and duplicates, leaves failures and ambiguity open, emits an issue receipt, and records dashboard audit events. Duplicate closure uses GitHub's `duplicate` state reason and canonical issue database ID.

### Stale mutation evidence

The one-hour cache remains available for dashboard display. Create and reconcile bypass it and reacquire repository contents from GitHub. Non-404 source errors fail closed. Reconciliation receipts include the evaluated `HEAD` SHA when GitHub supplies it.

### Literal `\\n` issue bodies

The relay path was not converting newlines: `hive-open-issue`, request JSON, the watcher, and `CreateIssue` preserve their input. The malformed strategist issue was produced before that boundary as a shell argument containing literal escapes. Strategist policies now require the already-supported body-file/heredoc protocol. Tests preserve real Markdown line breaks and intentional literal backslashes.

### Evaluator vocabulary

Generated issue text no longer instructs agents to create placeholder files. Tool-specific criteria are reported as evaluator-gap candidates and remain open unless the evaluator itself proves satisfaction. Broader semantic detection remains follow-up work.

## One-time live reconciliation

Fresh source refs:

- `joshyorko/actions`: `f6a3a92192620dec7dafa057975b39856f4f7909`
- `joshyorko/rcc`: `2c5af886f4058974369109f9dd3fd47633c2ac1b`

Before apply:

- Actions: 33 generated ACMM issues total, 24 open, 9 closed.
- RCC: 32 generated ACMM issues total, 25 open, 7 closed.

Dry-run inventory:

| Repo | Issue | Criterion | Role | Result | Proposed disposition |
|---|---:|---|---|---|---|
| actions | 156 | `acmm:github-actions-ai` | canonical | failing | evaluator gap |
| actions | 157 | `acmm:github-actions-ai` | duplicate of #156 | failing | duplicate |
| actions | 159 | `acmm:prereq-e2e` | canonical | failing | still failing |
| actions | 160 | `acmm:prereq-test-suite` | canonical | failing | still failing |
| actions | 161 | `acmm:prereq-code-style` | canonical | failing | still failing |
| actions | 162 | `acmm:prereq-coverage-gate` | canonical | failing | still failing |
| actions | 163 | `acmm:cursor-rules` | canonical | failing | evaluator gap |
| actions | 164 | `acmm:prompts-catalog` | canonical | failing | still failing |
| actions | 166 | `acmm:simple-skills` | canonical | failing | still failing |
| actions | 167 | `acmm:correction-capture` | canonical | failing | still failing |
| actions | 168 | `acmm:auto-qa-self-tuning` | canonical | failing | still failing |
| actions | 169 | `acmm:public-metrics` | canonical | failing | still failing |
| actions | 170 | `acmm:policy-as-code` | canonical | failing | still failing |
| actions | 171 | `acmm:reflection-log` | canonical | failing | still failing |
| actions | 173 | `acmm:auto-issue-gen` | canonical | failing | still failing |
| actions | 174 | `acmm:multi-agent-orchestration` | canonical | failing | still failing |
| actions | 175 | `acmm:strategic-dashboard` | canonical | failing | still failing |
| actions | 176 | `acmm:merge-queue` | canonical | failing | still failing |
| actions | 177 | `acmm:risk-assessment-config` | canonical | failing | still failing |
| actions | 179 | `aef:task-traceability` | canonical | failing | still failing |
| actions | 186 | `acmm:ci-matrix` | canonical | failing | still failing |
| actions | 187 | `acmm:mechanical-enforcement` | canonical | failing | evaluator gap |
| actions | 188 | `acmm:session-summary` | canonical | failing | evaluator gap |
| actions | 189 | `acmm:structural-gates` | canonical | failing | evaluator gap |
| rcc | 136 | `acmm:prereq-e2e` | canonical | failing | still failing |
| rcc | 139 | `acmm:claude-md` | canonical | failing | evaluator gap |
| rcc | 140 | `acmm:copilot-instructions` | canonical | failing | evaluator gap |
| rcc | 141 | `acmm:cursor-rules` | canonical | failing | evaluator gap |
| rcc | 142 | `acmm:prompts-catalog` | canonical | failing | still failing |
| rcc | 144 | `acmm:simple-skills` | canonical | failing | still failing |
| rcc | 145 | `acmm:correction-capture` | canonical | failing | still failing |
| rcc | 147 | `acmm:pr-review-rubric` | canonical | failing | still failing |
| rcc | 149 | `acmm:ci-matrix` | canonical | failing | still failing |
| rcc | 150 | `acmm:layered-safety` | canonical | failing | evaluator gap |
| rcc | 151 | `acmm:mechanical-enforcement` | canonical | failing | evaluator gap |
| rcc | 152 | `acmm:session-summary` | canonical | failing | evaluator gap |
| rcc | 153 | `acmm:structural-gates` | canonical | failing | evaluator gap |
| rcc | 154 | `acmm:github-actions-ai` | canonical | failing | evaluator gap |
| rcc | 155 | `acmm:auto-qa-self-tuning` | canonical | failing | still failing |
| rcc | 157 | `acmm:policy-as-code` | canonical | failing | still failing |
| rcc | 158 | `acmm:reflection-log` | canonical | failing | still failing |
| rcc | 159 | `acmm:auto-issue-gen` | canonical | failing | still failing |
| rcc | 160 | `acmm:multi-agent-orchestration` | canonical | failing | still failing |
| rcc | 161 | `acmm:strategic-dashboard` | canonical | failing | still failing |
| rcc | 162 | `acmm:merge-queue` | canonical | failing | still failing |
| rcc | 163 | `acmm:risk-assessment-config` | canonical | failing | still failing |
| rcc | 165 | `aef:task-traceability` | canonical | failing | still failing |
| rcc | 166 | `aef:audit-trail` | canonical | failing | still failing |
| rcc | 167 | `aef:change-classification` | canonical | failing | still failing |

Applied result:

- Closed `joshyorko/actions#157` with GitHub state reason `DUPLICATE`, linked to canonical `#156`.
- Closed no issue as satisfied because no currently open generated issue passed the fresh evaluator.
- Left all real failures and evaluator-gap candidates open.
- A second apply produced zero mutations.
- Actions after: 33 total, 23 open, 10 closed.
- RCC after: 32 total, 25 open, 7 closed.

## Dogfood branch comparison

The dependency observation/parser, adoption graph, internal-kick canonicalization, scanner disclosure/completion policy, App contributor delivery, headless contributor, and Codex CLI-boundary commits are not present in fresh upstream v4 and remain relevant to their own dogfood lanes. They are unrelated to this ACMM reconciliation/body-formatting patch and were not replayed here. The three dogfood documentation commits are evidence only. No whole stale source file was copied onto v4.

## Verification

- Focused dashboard ACMM tests, including race mode.
- Dashboard, GitHub, and policy package tests.
- `go vet` for changed packages.
- `go build ./...`.
- `bin/test_hive_open_issue.sh` (26/26).
- Live dry-run, one conservative App-authorized apply, GitHub state verification, and a zero-mutation second apply.

Remaining limitations: evaluator-gap detection currently flags clearly tool-specific evidence vocabularies but does not semantically prove equivalent capabilities; legacy issue identity depends on the existing criterion field; and cross-process concurrent creators rely on subsequent reconciliation because GitHub does not provide a repository-scoped uniqueness constraint.
