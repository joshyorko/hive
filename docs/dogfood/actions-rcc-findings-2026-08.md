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

### Resume API can briefly return before status reflects the resumed agent

- **Observed behavior:** `POST /api/resume/scanner` returned 200, while the
  immediately following `/api/status` sample still reported scanner paused.
  A second status sample about three seconds later reported it running, and
  the Codex process and tmux session were healthy.
- **Expected behavior:** Either the resume response should mean the observable
  state transition is complete, or it should expose a transitional state that
  prevents operators from reading a successful resume as a failed one.
- **Minimal reproduction:** Resume a persisted-paused agent and fetch status in
  the same command immediately after the successful response.
- **Affected source/functions:** Dashboard pause/resume handlers, agent manager
  start lifecycle, and fast status snapshot publication.
- **Impact:** Operational ambiguity during safety-critical launch/park checks;
  no lost work or unauthorized dispatch was observed.
- **Reproduced more than once:** No.
- **Tests/evidence:** 200 resume response, immediate paused status sample, later
  running sample, lifecycle audit logs, live Codex processes, and tmux panes.
- **Likely severity:** Low.
- **Duplicate search:** Deferred until the behavior reproduces or source review
  establishes a generic contract violation.
- **Disposition:** DOGFOOD / DESIGN FINDING; needs more soak evidence.

### Codex agents block unattended when the inner bubblewrap sandbox is unavailable

- **Observed behavior:** Scanner received its first real kick inside Codex, but
  its first read-only shell command failed because bubblewrap could not create
  an unprivileged namespace. Codex then waited indefinitely for an interactive
  approval instead of continuing the scheduled cycle.
- **Expected behavior:** A scheduled Codex agent must either use a working
  sandbox or run non-interactively within Hive's declared container, UID, and
  proxy boundary; it must not wait for an unattached approval prompt.
- **Minimal reproduction:** Launch a Codex-backed scheduled agent in the local
  Hive container, kick it with work requiring one shell command, and observe
  the namespace failure followed by the approval selector.
- **Affected source/functions:** Codex backend launch policy in
  `pkg/agent.Manager.launchInTmux`; local durable Codex-home generation.
- **Impact:** Complete loss of unattended factory throughput; no command ran
  and no GitHub mutation occurred in this reproduction.
- **Reproduced more than once:** No; the first scanner tool call reproduced it.
- **Tests/evidence:** Scanner pane, Codex process state, and the exact bubblewrap
  error/approval prompt were captured. A non-interactive canary will gate the
  local configuration change before relaunch.
- **Likely severity:** Medium operational reliability.
- **Duplicate search:** Pending source-level triage after the soak; do not delay
  commissioning to prepare an upstream issue.
- **Disposition:** DOGFOOD / DESIGN FINDING pending generic-source confirmation.

### Park fail-safe assumed exactly one persisted owner session

- **Observed behavior:** A second valid GitHub Device Flow owner session was
  persisted during the run. `park-dogfood.sh` required exactly one map entry,
  could no longer authenticate, and correctly fell back to stopping Hive and
  gateway without deleting the volume.
- **Expected behavior:** The watchdog should select a valid persisted owner
  session deterministically when several exist and still fail closed if none
  authenticate.
- **Minimal reproduction:** Persist two valid owner sessions, run
  `park-dogfood.sh`, and observe the unique-entry selector return no handle.
- **Affected source/functions:** Local dogfood watchdog session selection; Hive
  session persistence itself behaved normally.
- **Impact:** Safe but unnecessary factory shutdown and lost soak time.
- **Reproduced more than once:** No.
- **Tests/evidence:** Session count and non-secret role/expiry metadata, failed
  park output, and container exit codes 143/137 were captured.
- **Likely severity:** Low, local operational tooling.
- **Duplicate search:** Not searched; this is not currently a Hive source defect.
- **Disposition:** DOGFOOD / DESIGN FINDING; local watchdog corrected.

### Root-run Codex canary changed an agent-owned config file's ownership

- **Observed behavior:** A diagnostic `docker exec` canary ran Codex as root
  against scanner's real `CODEX_HOME`. Codex rewrote `config.toml` as root,
  after which UID 2006 could not launch and Hive entered crash recovery.
- **Expected behavior:** Runtime canaries must execute as the target agent UID
  and preserve the durable per-agent ownership contract.
- **Minimal reproduction:** Run Codex as root with
  `CODEX_HOME=/data/home/.codex-scanner`, then launch the scheduled scanner UID.
- **Affected source/functions:** Local operator canary procedure; per-agent
  Codex-home ownership established by `configure-codex-headroom.py`.
- **Impact:** Temporary scanner outage; no issue, branch, PR, or source mutation.
- **Reproduced more than once:** No.
- **Tests/evidence:** Root ownership and permission-denied pane output captured;
  the configurator restored `2006:1000`, and the shell-tool canary then passed
  as UID 2006 while preserving that ownership.
- **Likely severity:** Low, operator procedure.
- **Duplicate search:** Not searched; not established as a Hive defect.
- **Disposition:** DOGFOOD / DESIGN FINDING; procedure corrected.

### Login detector mistakes an optional MCP login warning for backend logout

- **Observed behavior:** A healthy Codex scanner was automatically paused when
  its pane displayed `The cloudflare-api MCP server is not logged in`. The
  detector matched the built-in `Not logged in` pattern even though ChatGPT
  OAuth, Headroom, and a Codex tool-use canary were all healthy.
- **Expected behavior:** Login detection should identify the scheduled agent's
  primary CLI/backend authentication state, not pause it for an optional MCP
  integration that is unrelated to its assigned work.
- **Minimal reproduction:** Launch a Codex agent with an unauthenticated
  optional MCP plugin, wait for its startup warning, and run one governor
  evaluation with the default login patterns.
- **Affected source/functions:** `scanForLoginRequired` in
  `src/cmd/hive/main.go`; `defaultLoginPatterns` in
  `src/pkg/config/config.go`.
- **Impact:** Deterministic repeated auto-pause of otherwise healthy scheduled
  Codex agents; zero unattended throughput.
- **Reproduced more than once:** Yes; startup warnings and the subsequent
  governor scan reproduced the same pane text/match path.
- **Tests/evidence:** Pane warning, healthy OAuth/Headroom canaries, detector log
  naming `(?i)Not logged in`, token re-cache, and automatic pause audit event.
- **Likely severity:** Medium operational reliability.
- **Duplicate search:** No matching open or closed issue for `Not logged in MCP`.
  Closed issues #4041/#4042/#4072 own related login-detector/token cases but not
  optional MCP warnings.
- **Disposition:** PUBLIC / NORMAL BUG; prepare a concise upstream issue after
  the factory reaches a stable first cycle. Tonight's all-Codex deployment
  omits only the ambiguous `Not logged in` pattern while retaining the other
  exact backend login indicators.

## Public issue disposition

### Architect adoption handoff directory was not group-writable

- **Observed behavior:** The Terra architect completed its bounded backlog
  reconciliation but could not write `architect-proposed.json` because the
  operator-created `/data/planning/adoption` directory inherited mode `0755`.
- **Expected behavior:** A planning artifact directory created by Hive's
  operator-owned adoption command must be writable by the isolated architect
  UID through the shared `node` group.
- **Minimal reproduction:** Create the directory as `dev` with the default
  umask, then attempt a file write as `hive-architect` (UID 2001, GID 1000).
- **Affected source/functions:** New `cmd/hive-adopt` output handoff; existing
  per-agent UID/group boundary in `pkg/agent.Manager`.
- **Impact:** Planning completes inference but cannot durably hand the proposal
  to validation/promotion.
- **Reproduced more than once:** No; reproduced deterministically with
  `test -w` before and after the mode correction.
- **Tests/evidence:** Architect pane reported the directory read-only; mode was
  `0755 dev:node`; `chmod 2770` made the same UID write check pass. The command
  now creates/chmods output parents to setgid group-writable mode.
- **Likely severity:** Medium for the new adoption vertical.
- **Duplicate search:** Not applicable yet; this was found in uncommitted local
  integration code.
- **Disposition:** DOGFOOD / DESIGN FINDING; retain in the soak receipt.

### Scheduled scanner starts without a source checkout

- **Observed behavior:** The first post-restart production scanner kick received
  the admitted Actions/RCC work list and executed tools unattended, but neither
  `/data/agents/scanner` nor the advertised `HIVE_REPO_DIR=/tmp/hive` was a Git
  checkout. The latter contained only Hive runtime scripts.
- **Expected behavior:** An implementation-capable scheduled worker should begin
  a real issue cycle with the selected repository available at its declared
  workspace path, or deterministically acquire it before source inspection.
- **Minimal reproduction:** Resume scanner, send one normal empty kick, and run
  `git status` in its working directory and `git -C "$HIVE_REPO_DIR" status`.
- **Affected source/functions:** Agent workspace/bootstrap preparation and the
  scheduled tmux kick path in `pkg/agent.Manager`.
- **Impact:** The sandbox blocker is fixed, but scanner cannot inspect, test, or
  implement the admitted issue until it obtains a real checkout.
- **Reproduced more than once:** No; captured on the first production kick after
  the paused Compose recreation.
- **Tests/evidence:** Scanner pane captured the admitted 17-item work list,
  unattended tool calls, both failed Git checks, and the non-repository runtime
  script directory.
- **Likely severity:** Medium operational reliability.
- **Duplicate search:** Pending; do not interrupt the active soak to classify it.
- **Disposition:** DOGFOOD / DESIGN FINDING pending evidence from later cycles.

### One invalid adoption edge serialized the entire otherwise-valid graph

- **Observed behavior:** Deterministic authorization promoted 80 valid proposed
  edges, then rejected the complete generation because one architect edge
  referenced missing `joshyorko/actions#128`.
- **Expected behavior:** Missing, self-referential, or cyclic edges remain inert
  and diagnosed locally while unrelated validated edges can be promoted.
- **Minimal reproduction:** Authorize one valid existing-ref edge together with
  one missing-ref edge in the same bounded adoption spec.
- **Affected source/functions:**
  `pkg/planning/adoption.AuthorizeExplicitEdges` and `ValidatePromotion`.
- **Impact:** A single stale roadmap reference could prevent all adopted work
  from reaching convergence.
- **Reproduced more than once:** Yes, with the real architect graph and a focused
  regression fixture.
- **Tests/evidence:** RED/GREEN mixed valid/missing/cycle authorization test;
  the real graph now promotes 76 edges and leaves five inert with reasons.
- **Likely severity:** Medium for the new adoption vertical.
- **Duplicate search:** Not applicable; the affected implementation is local
  and uncommitted.
- **Disposition:** DOGFOOD / DESIGN FINDING; fixed locally before launch.

### Explicit dependency prose without a colon bypasses the source field parser

- **Observed behavior:** `joshyorko/actions#99` says `Depends on #80, #94, #95,
  #97, and #98`, but the explicit GitHub field observer admitted it because the
  declaration lacks the supported colon. Architect adoption independently
  recovered the edge set and withheld the issue.
- **Expected behavior:** The supported explicit dependency grammar and its
  diagnostics should make this common near-field syntax unambiguous rather
  than silently treating it as no dependency.
- **Minimal reproduction:** Evaluate an issue body containing `Depends on #97`
  with #97 open.
- **Affected source/functions:** GitHub dependency-field parsing in
  `pkg/worksource` and the dashboard source observation adapter.
- **Impact:** False admission when planning adoption is absent; the approved
  planning overlay prevents it in this bounded run.
- **Reproduced more than once:** No; confirmed against the live #99 body and
  fresh source snapshot.
- **Tests/evidence:** Explicit-only frontier was 17/19; the readiness oracle and
  architect-approved graph both classify #99 as blocked.
- **Likely severity:** Medium admission correctness.
- **Duplicate search:** Deferred until the syntax contract is triaged after the
  soak; no public issue filed during commissioning.
- **Disposition:** DOGFOOD / DESIGN FINDING needing upstream grammar decision.

- **Filed publicly:** None.
- **Prepared but not filed:** Optional MCP login warning false-pauses healthy
  Codex agents (pending concise issue body after stable first cycle).
- **Duplicates with upstream owners:** Related but non-owning closed issues
  #4041, #4042, and #4072.
