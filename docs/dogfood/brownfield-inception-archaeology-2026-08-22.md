# Brownfield Inception archaeology — 2026-08-22

## Boundary

This was a source/history audit only. No Inception API was called; no live
Inception state, wiki, ACMM level, agent, scheduler, or Actions/RCC repository
was changed. The source inspected was freshly fetched `kubestellar/hive:v4` at
`18e8172b98be5663c95b3edd7da0fce068220313`.

## Classification

**3 — the #711/#751 sequence regressed Brownfield repository inspection.**

PR #695 (`7c4ddc514f76824897ff712e265352eb3de3852d`) introduced a distinct
brownfield capture contract. Its policy consumed `${INCEPTION_REPO_URL}`,
cloned or navigated to the repository, inspected documentation, manifests,
workflows, tests, CI, and architecture, and produced facts plus missing-file or
amendment suggestions. Its PR test plan left the brownfield end-to-end check
unchecked.

PR #711 (`e6c439a6255a4f21eb4eb7f89a1007458cf0c7eb`) replaced the separate
Inception policies with one brainstorm template. The replacement retained
`${INCEPTION_MODE}` but removed the brownfield capture branch and did not
consume `${INCEPTION_REPO_URL}`.

PR #751 (`b244f249988baf99dcfb6750e117459fb92b2cce`) fixed brainstorm agents
following ordinary ACMM policies instead of the Inception kick. Its stated
intent was to make the kick authoritative, but the resulting override forbids
cloning, scanning, reading repository files, and searching GitHub for every
Inception mode. The PR did not state that the brownfield product contract was
being removed, and the documentation, dashboard copy, scheduler variable, and
`StartBrownfield` comment were left intact. This makes classification 2
unsupported: there is evidence of an over-broad safety/attention fix, not an
intentional product decision to retire brownfield inspection.

Later capture-policy fixes through #1043 retained the mode-agnostic no-scan
shape. The operator guide was added later in #3630 and still documents an
actual brownfield scan, further ruling out a completed intentional contract
removal. A read-only GitHub search found no current kubestellar/hive issue or PR
owning a brownfield Inception restoration.

## Current data flow

- `handleInceptionScan` validates owner role, optionally resets, calls
  `StartBrownfield`, then kicks brainstorm.
- `StartBrownfield` accepts any `http://` or `https://` URL, clears the current
  inception wiki, persists capture state and the URL, and performs no source
  acquisition.
- Scheduler substitution carries `${INCEPTION_MODE}` and
  `${INCEPTION_REPO_URL}`. The current brainstorm policy uses the mode only as
  displayed context and never references the repository URL.
- The capture policy always creates generic clarification beads and explicitly
  forbids repository evidence acquisition.
- The watcher can derive a vision from the stored URL, but a URL string is not
  repository evidence.

No alternative authoritative repository-evidence path repairs this flow.
Configured Git-backed knowledge sources can clone, validate, index, and refresh
repository Markdown with SSRF and redirect controls, and the knowledge primer
can query their file stores. They are separately configured sources; starting
brownfield Inception neither creates nor selects one, and the scheduler does
not bind one to the brownfield URL. Worksource repository identities authorize
issue routing, not repository-content acquisition. The scheduler's optional
AGENTS.md priming hook is not a brownfield scan and currently has no per-repo
checkout binding.

## Tests required before repair

1. A brownfield capture prompt must consume evidence from an explicitly
   authorized configured repository and must not treat the URL string alone as
   evidence.
2. Greenfield capture must retain the no-clone/no-scan boundary.
3. Brownfield and greenfield must select distinct capture behavior from
   `INCEPTION_MODE`.
4. An unconfigured repository, alternate host, redirect, private/link-local
   destination, invalid branch, inaccessible source, or missing App permission
   must fail closed without clearing established knowledge.
5. Evidence acquisition must preserve repository credentials and sandbox
   boundaries and must never interpolate an operator URL into an agent shell
   command.
6. Brownfield facts and amendment proposals must cite repository/ref/SHA/path
   provenance; ambiguous evidence must remain context, not admission authority.
7. A regression fixture must prove that the #695-style README/manifest/CI facts
   are available during capture and that existing files are not replaced by
   greenfield scaffold output.
8. A non-destructive refresh must produce a new evidence generation without
   invoking `StartBrownfield`, clearing the wiki, changing ACMM level, or
   disturbing active scheduling.

## Bounded recommendation

Do not restore agent-side `git clone <operator-url>`. Add a separate owner-only
brownfield evidence operation that selects a repository from the configured
project boundary or an existing approved Git knowledge source. Reuse the Git
source URL, redirect, branch, timeout, and SSRF validation; add repository/App
authorization and immutable ref/SHA provenance. Feed the resulting versioned,
read-only evidence snapshot into Inception capture. Model output remains
proposed project knowledge, never convergence admission or mutation authority.

For established L2–L6 hives, expose a distinct non-destructive refresh that
reacquires the same authorized source into a new evidence generation, diffs it
against the prior generation, and proposes fact amendments. It must not start a
new L1 Inception, clear or replace the wiki, generate scaffolding, or pause the
factory. Promotion of amendments remains an explicit authority decision.

Brownfield Inception and Continuity remain separate: Inception bootstraps or
refreshes project knowledge and intent; Continuity adopts existing work,
branches, dependencies, ownership, and delivery state.
