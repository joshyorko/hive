# HIVE CONTINUITY

Continuity is Hive's ability to accept responsibility for an existing body of
work without requiring Hive to have created its plan, issues, dependencies,
branches, pull requests, or earlier agent activity.

Discovery is not authority. An operator must establish an ownership boundary;
observed repository state may then inform convergence within that boundary.
Ambiguous or contradictory evidence remains unknown or requires a human
decision. It must not be promoted into desired state by inference.

The GitHub source-dependency observer is one narrow Continuity observer:

```text
pre-existing explicit GitHub dependency declarations
    -> source-specific normalization
    -> convergence.Observation
    -> convergence.Evaluate
    -> existing admission and scheduling
```

The current seam deliberately keeps GitHub grammar and source authority in
`pkg/worksource`. `pkg/convergence` remains source- and runtime-independent,
and authoritative observations do not require synthetic or Hive-created beads.
The configured issue filter supplies tonight's adoption boundary, but
convergence does not assume that a label is the only possible future boundary.
ACMM, enrollment, hold, exemption, and repository policy continue to determine
which transitions Hive may execute.

A complete Continuity vertical would add authoritative observers for:

- completion and accepted outcome state;
- existing pull requests, branches, and work ownership;
- CI and required-check state;
- accepted plans, declared intent, and operator decisions;
- prior agent activity, claims, and leases;
- other configured work sources;
- contradictions, inaccessible evidence, and uncertainty.

Those observations should compose into the existing convergence contract,
which continuously reacquires reality and exposes unknown states instead of
guessing. A future adoption policy may use labels, an operator-owned manifest,
or another explicit boundary; it must keep discovery separate from authority.

This note preserves the architectural seam only. It does not introduce a
generic Continuity controller.
