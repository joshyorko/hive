# Actions/RCC dogfood findings — 2026-08

This ledger is updated while commissioning the local Actions/RCC factory.
Discovery here does not itself authorize an upstream report or a runtime
change. Security-sensitive details stay in the local private packet.

## Private security findings

### Operator Compose inspection expanded a dashboard credential

- **Observed behavior:** `docker compose config` expanded an environment-backed
  dashboard credential into command output during source-build verification.
- **Expected behavior:** Operational inspection must use secret-safe selectors
  and must not render credential-bearing environment blocks.
- **Minimal reproduction:** Run fully expanded Compose configuration output on
  a deployment whose service environment references a dashboard token.
- **Affected source/functions:** Local operator verification procedure; this is
  not yet established as a Hive source defect.
- **Impact:** Credential exposure in local tool/transcript output.
- **Reproduced more than once:** No.
- **Tests/evidence:** The exposed token was rotated immediately; replacement
  length was verified without printing it.
- **Likely severity:** Medium in this local environment.
- **Duplicate search:** Not searched upstream because this is currently an
  operator-procedure finding, not a confirmed generic Hive defect.
- **Disposition:** PRIVATE SECURITY REVIEW REQUIRED; retain in the local packet.

### Session-store schema inspection exposed an opaque session handle

- **Observed behavior:** A diagnostic `jq keys` probe printed the sole persisted
  device-flow session identifier into tool output.
- **Expected behavior:** Session-store inspection must report counts and schema
  only, never bearer-like map keys.
- **Minimal reproduction:** Run a key-listing query against the persisted
  dashboard session object.
- **Affected source/functions:** Local operator verification procedure; this is
  not established as a Hive source defect.
- **Impact:** Exposure of a reusable authenticated-session handle.
- **Reproduced more than once:** No.
- **Tests/evidence:** The exposed handle was atomically replaced with a fresh
  opaque identifier, the validated session record was preserved, and Hive was
  restarted to reload the rotated store.
- **Likely severity:** Medium in this local environment.
- **Duplicate search:** Not searched upstream because this is an operator
  diagnostic failure, not a confirmed generic Hive defect.
- **Disposition:** PRIVATE SECURITY REVIEW REQUIRED; retain in the local packet.

### Published gateway could promote anonymous API traffic to owner

- **Observed behavior:** An anonymous owner-only GET through the published
  gateway returned success while the authenticated internal path rejected it.
- **Expected behavior:** The gateway must preserve client authentication and
  owner-only routes must return 401 or 403 without valid owner credentials.
- **Minimal reproduction:** Request an owner-only API route through published
  port 3001 without credentials and compare its status with authenticated
  owner access.
- **Affected source/functions:** Standalone gateway proxy configuration and Go
  owner middleware at the API trust boundary.
- **Impact:** Authentication and authorization bypass at the published API.
- **Reproduced more than once:** No; one confirmed pre-fix reproduction plus a
  complete post-fix published-gateway regression matrix.
- **Tests/evidence:** Local commit `e683f934`; anonymous, invalid bearer,
  invalid cookie, valid dashboard, valid device-session, public-route, and
  mutation/no-mutation probes captured locally.
- **Likely severity:** High.
- **Duplicate search:** No matching open or closed `kubestellar/hive` issue was
  found by a read-only search for the internal-header boundary on 2026-08-22.
- **Disposition:** PRIVATE SECURITY REVIEW REQUIRED. Never file publicly from
  this ledger.

### Codex kick clearing exited to Bash and executed prompt text

- **Observed behavior:** Hive verified an idle Codex prompt, sent Ctrl-C as an
  input-clear operation, and then typed the full Markdown kick into the Bash
  prompt left behind. Backticks and example command lines were executed.
- **Expected behavior:** Agent prompts must remain CLI input and must never be
  interpreted by a shell.
- **Minimal reproduction:** Start a Codex-backed tmux agent, wait at the idle
  prompt, deliver a Markdown kick containing a harmless shell-substitution
  sentinel, and verify that the sentinel is not executed outside Codex.
- **Affected source/functions:** `pkg/agent.Manager.SendKick`,
  `deliverKickLocked`, and the tmux key-delivery boundary.
- **Impact:** Unintended command execution as the agent account; prompt content
  can cross the CLI/shell privilege boundary.
- **Reproduced more than once:** Yes; independently observed on supervisor and
  scanner during the first enforced launch.
- **Tests/evidence:** Preserved shutdown kick logs; RED/GREEN key-sequence test;
  real-tmux Codex-stub regression proving Markdown stays literal and creates no
  sentinel file.
- **Likely severity:** High.
- **Duplicate search:** No matching open or closed `kubestellar/hive` issue was
  found by a read-only search for Codex/Bash kick delivery on 2026-08-22.
- **Disposition:** PRIVATE SECURITY REVIEW REQUIRED. Never file publicly from
  this ledger.

## Dogfood and design findings

### GitHub dependency snapshots enumerate complete issue history

- **Observed behavior:** Source dependency observation currently obtains
  `state=all` issue snapshots and paginates the complete issue history.
- **Expected behavior:** A future large-repository implementation should bound
  source acquisition while retaining authoritative closed/reopened state.
- **Minimal reproduction:** Run one normal GitHub actionable sweep and record
  issue-list pages and source snapshot size.
- **Affected source/functions:** GitHub actionable enumeration and
  `pkg/worksource` dependency snapshot adaptation.
- **Impact:** Scaling and API-budget risk on repositories with long histories.
- **Reproduced more than once:** Yes, across measured Actions/RCC sweeps.
- **Tests/evidence:** Four issue-list calls per two-repository sweep, eight total
  core calls, 123 issue records, about 602 KB of body text, and 56–125 ms
  projection latency.
- **Likely severity:** Low for this soak; potentially medium at large scale.
- **Duplicate search:** No matching `kubestellar/hive` issue found by a
  read-only state-all/pagination search on 2026-08-22.
- **Disposition:** DOGFOOD / DESIGN FINDING; retain for upstream design work.

### Continuity needs explicit adoption authority beyond discovery

- **Observed behavior:** The source dependency observer can normalize existing
  GitHub declarations without synthetic beads, but tonight's adoption boundary
  is still an operator-configured issue label.
- **Expected behavior:** Future Continuity can inventory existing work while an
  explicit operator policy—not discovery—defines authority and ownership.
- **Minimal reproduction:** Adopt a pre-existing issue graph and compare
  discovered repository state with the explicitly enrolled candidate set.
- **Affected source/functions:** Worksource adapters, convergence observations,
  enrollment policy, and future authoritative observers.
- **Impact:** Architectural seam; premature inference could turn ambiguous
  intent into unauthorized desired state.
- **Reproduced more than once:** Not applicable; design constraint.
- **Tests/evidence:** `docs/continuity.md` and the source-neutral dependency
  observation path.
- **Likely severity:** Design requirement.
- **Duplicate search:** Deferred until a concrete upstream Continuity proposal
  is prepared; no defect is asserted yet.
- **Disposition:** DOGFOOD / DESIGN FINDING; needs more evidence.

## Public issue disposition

- **Filed publicly:** None.
- **Prepared but not filed:** None.
- **Duplicates with upstream owners:** None identified so far.
