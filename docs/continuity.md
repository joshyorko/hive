# Hive Continuity

Continuity lets an operator make Hive responsible for existing work without
claiming that Hive created its history or intent. Discovery remains
non-authoritative: source evidence becomes mutable scheduling authority only
after an owner-gated adoption operation records provenance in a durable ledger.

## PR and branch adoption

The first bounded vertical adopts an existing GitHub pull request inside the
configured project boundary.

1. `pkg/github` observes the PR, its exact head and base, linked work, checks,
   stack ancestry, overlapping files, and repository write capability.
2. An owner may dry-run or promote that observation through
   `/api/continuity/pr-adoptions`. A label is not authority.
3. `pkg/continuity` persists the adopted identity and generation in
   `/data/continuity-pr-adoptions.json`.
4. Active owned acceptance slices suppress replacement issue implementation in
   the existing actionable/admission path.
5. Only exact-head, writable `CONTINUE` records enter the existing contributor
   queue. `READY`, `BLOCKED`, `UNKNOWN`, and `SUPERSEDED` remain observable but
   are not implementation assignments.
6. The contributor receives the existing repository, branch, base, PR number,
   original author, and observed head. It must fetch and check out that branch;
   it may not create a replacement PR, rewrite history, force-push, change draft
   state, remove hold, or merge.
7. Immediately before credential delivery Hive verifies the exact head again.
   Unexpected movement fails closed and is reacquired by the next observation.
8. Completion is authoritative only when GitHub proves that the adopted head is
   an ancestor of a newly advanced head, every new commit is attributed to the
   assigned contributor, the original PR author and branch are unchanged, and
   the durable ledger accepts the delivery receipt.

Adoption authorizes continuation, not release. Draft and hold state remain
GitHub review/release boundaries. Auto-merge and ACMM policy are unchanged.

## Lifecycle states

- `CONTINUE`: the existing implementation has remaining owned work and its
  exact branch is writable.
- `READY`: the current exact head is green and its owned slice appears complete;
  it proceeds through normal review/release policy.
- `BLOCKED`: legitimate work exists but a concrete dependency, conflict, or
  write boundary prevents continuation.
- `SUPERSEDED`: a separately authorized decision replaced this implementation.
- `UNKNOWN`: identity, ownership, access, or evidence is ambiguous. Ownership
  suppression remains active until an owner revokes or reacquires it.

Repeated adoption and unchanged observation are idempotent. Revocation is
owner-gated and releases the owned slice. A verified contributor delivery may
advance the observed head because the active assignment is already authorized
continuation; it does not grant merge authority.

## Many-to-many work relationships

Issue and PR relationships are retained as acceptance slices. A closing keyword
is not assumed to prove that one PR owns an entire issue. Explicit partial
language combined with `Closes` is surfaced as ambiguous, and a draft carrying
a closing keyword is flagged for review. References without an owned slice do
not suppress unrelated work.

## Future Continuity observers

PR/branch adoption is one observer, not a generic controller. A broader
Continuity vertical will need authoritative observers for accepted plans and
roadmap roots, issue dependencies, merged and released repository state,
branches without PRs, CI/artifact state, prior agent receipts, work leases and
ownership, supersession decisions, and operator-defined adoption boundaries.
Those observers should produce source-neutral evidence and uncertainty for the
existing convergence/admission and scheduler paths; they must not move source
semantics or planning inference into `pkg/convergence`.

## Current bounds

The implementation intentionally supports only same-repository, unprotected
heads for which the App proves repository push permission. Protected heads stay
`UNKNOWN` because repository permission alone does not prove a ruleset bypass.
Forks, deleted branches, inaccessible PRs, ambiguous links, and unexpected head
movement fail closed. File-overlap discovery is bounded by the existing open-PR
pagination limit and is advisory evidence, not an automatic merge or rebase
decision. Adoption does not infer acceptance criteria from arbitrary prose.
