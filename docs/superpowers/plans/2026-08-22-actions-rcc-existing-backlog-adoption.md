# Actions/RCC Existing-Backlog Adoption

## Boundary

Adoption is a Planning Intelligence input adapter, not a convergence feature. It
describes a finite project over canonical `worksource.Ref` identities and stores
that description as a generation in the existing `convergence/outcome` ledger.
It never creates issues or beads.

The dogfood project is bounded to `joshyorko/actions` and `joshyorko/rcc`, with
roadmap roots `actions#101`, `actions#82`, and `rcc#118`. Discovery may inspect
only those roots and work they explicitly reference inside that repository set.

## Durable contract

`planning.adoption` names the finite project, repository set, and canonical
roadmap roots. `hive-adopt prompt` builds a bounded inventory from the current
source snapshot and asks the existing architect role to emit proposal-only JSON
over existing refs. The parser rejects any architect-authored authoritative
state. A closed classification vocabulary plus exact excerpt verification,
missing-ref/scope/self/cycle checks, and the operator-authorized promotion step
separate model inference from admission authority.

The outcome record spec is typed JSON containing:

- project identity, repository scope, and roadmap roots;
- canonical candidate-to-prerequisite edges;
- exact source evidence and its hash;
- edge state (`proposed`, `promoted`, or `rejected`) and classification;
- planner/provenance metadata.

The outcome ledger remains the authority and generation store. A proposed
outcome generation is inert. Promotion validates both refs against one bounded
source snapshot, rejects self-edges, missing refs, scope escapes, cycles, and
contradictions, then accepts the exact generation through the ledger CAS.

Only promoted edges from an accepted generation are projected into dependency
observation. Ambiguous evidence remains proposed and cannot gate work.

## Admission

The adoption observer resolves prerequisite state level-triggered from the same
bounded source snapshot as explicit GitHub dependencies. Closed is satisfied;
open is blocked; missing/inaccessible/unknown is Unknown. Its observation is
conjunctively composed with bead and explicit-source observations at the
existing dashboard caller seam before the unchanged source-neutral
`convergence.Evaluate` call.

ReadyQueue, `selectTask`, and internal kick projection therefore consume one
judgment. No scheduler, ranking, routing, or convergence semantics change.

## Continuity seam

This is one narrow HIVE CONTINUITY observer: pre-existing authoritative planning
intent becomes a normalized observation over pre-existing work. Future
Continuity can add authoritative observers for existing PR/branch ownership,
CI and artifact state, prior accepted plans/agent receipts, completed work, and
operator adoption boundaries. Those observers should share canonical refs,
generation/provenance, promotion, and caller-side composition; discovery must
never become authority merely because it found something.

The current label filter is only this dogfood's enrollment boundary. The typed
project scope and roots deliberately do not encode labels as the universal
future adoption mechanism.
