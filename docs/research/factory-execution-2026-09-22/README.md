# Factory execution research handoff

This directory contains a proposed issue and its supporting source audit. It changes no runtime code.

- **[ISSUE.md](ISSUE.md)** — copy-paste issue draft, including the proposed title as its first heading.
- **[AUDIT.md](AUDIT.md)** — corrections to the earlier adapter thesis, source-level limitations, prior art, gotchas and scope choices.

## Filing

Use the first heading of `ISSUE.md` as the issue title and the rest as the body. File it from your own `joshyorko` account so you can maintain the body. Do not reopen or close/reopen #3828: [Andy requested a new author-owned issue](https://github.com/hivecommons/hive/issues/3828#issuecomment-5294081066) because you could not edit the original.

This is a proposed contract/conformance pilot, not an approved mandate to build a universal factory platform. Request maintainer triage/hold. Link #7620 and follow the maintainer's decision if it should live there instead of becoming a new issue. The body does not apply a GitHub label by itself.

## Branch

- Fork: `joshyorko/hive`
- Topic branch: `research/factory-execution-contract-v6`
- Source: upstream `hivecommons/hive` `v6`
- Exact source commit: `e0e99f7b2d37f1344fcc8b001f59b0133ab660f6`
- Intended changes: these three Markdown files only

The branch base records the requested research snapshot, not an upstream release decision. `CONTRIBUTING.md` at that snapshot reserves v6 for dashboard-optional operation and routes structural/protocol work through v5 RFC review. The issue asks maintainers to select the implementation line.

## Validation limits

The research used source reads and primary documentation. No Hive/factory runtime tests were executed, no production credentials were exercised, and no upstream issue or PR was opened. The proposed acceptance tests in `ISSUE.md` are work to perform after the contract is accepted, not evidence already obtained. Local document checks validate formatting, reference definitions, size and hashes; they do not establish runtime correctness.

The minimum useful result can be an accepted existing-seam integration with a documented limitation. Full outcome-ledger activation, additional transports and automatic publication are deliberately not prerequisites to the first report-only pilot.
