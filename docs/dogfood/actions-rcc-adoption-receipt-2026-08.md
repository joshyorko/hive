# Actions/RCC Existing-Backlog Adoption Receipt

Date: 2026-08-22

## Authority boundary

- Project: `actions-rcc`
- Repositories: `joshyorko/actions`, `joshyorko/rcc`
- Roadmap roots: `joshyorko/actions#101`, `joshyorko/actions#82`,
  `joshyorko/rcc#118`
- Planner: existing Hive `architect`, `gpt-5.6-terra`, reasoning high
- Planner output: 81 canonical existing-work edges, all `proposed`
- Deterministically promoted: 76
- Left proposed and inert: 5
- Synthetic issues or beads created: 0

The architect independently recovered the required major sequence from exact
roadmap evidence:

```text
joshyorko/rcc#120
  -> joshyorko/actions#134
  -> joshyorko/actions#143
  -> joshyorko/actions#135
  -> joshyorko/actions#148
  -> joshyorko/actions#136
```

## Non-promoted edges

- Four `explicit-producer-consumer-order` proposals remain non-authoritative.
  Their excerpts establish consumption or shared structure, but do not by
  themselves deterministically establish hard execution order.
- `joshyorko/actions#134 <- joshyorko/actions#128` remains proposed because
  #128 is absent from the bounded source snapshot. The same exact excerpt still
  authorizes the independently valid `#134 <- joshyorko/rcc#120` edge.

## Admission comparison

Before an accepted planning generation, the explicit GitHub observer produced:

- admitted: 17
- dependency-blocked: 19
- dependency-unknown: 0

With the accepted architect-derived graph composed with explicit GitHub and
bead observations:

- admitted: 12
- dependency-blocked: 24
- dependency-unknown: 0

Initially admitted work is:

- `joshyorko/actions#129`
- `joshyorko/actions#137`
- `joshyorko/actions#139`
- `joshyorko/actions#91`
- `joshyorko/actions#97`
- `joshyorko/rcc#121` through `joshyorko/rcc#127`

The prior manually encoded plumbing fixture produced 13/23. The one-item
difference is explained: `joshyorko/actions#99` contains the explicit statement
`Depends on #80, #94, #95, #97, and #98` without a colon. The narrow GitHub
field parser missed that syntax, while both the readiness oracle and architect
classify #99 as blocked by the still-open #97. The architect result is therefore
the safer oracle match; the discrepancy is retained in the findings ledger.

Independent Actions and RCC lanes remain admitted while the RCC-to-Actions
consumer sequence is withheld. Proposed-only output does not gate work, and
restart reconstruction uses the accepted generation in the outcome ledger.
