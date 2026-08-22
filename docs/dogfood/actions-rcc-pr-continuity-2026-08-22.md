# Actions/RCC PR and branch continuity dry run — 2026-08-22

## Provenance

- Fresh upstream base: `kubestellar/hive:v4` at `18e8172b98be5663c95b3edd7da0fce068220313`.
- Worktree: `dogfood/pr-branch-continuity-2026-08-22`.
- Source of truth: live read-only `ghx pr view` / `ghx issue view` on `joshyorko/actions`.

## Dry-run summary

| PR | Head | Base | State | Linked issue(s) | Closing semantics | Stack / overlap | Proposed action |
|---:|---|---|---|---|---|---|---|
| 109 | `factory/issue-84-shared-database` @ `103c0788c8ba0dd6cb000658d19fd065a362f997` | `community` | `BLOCKED` | `#84` | `Closes #84` is correct | Draft sibling on the community base; overlaps docs with #112/#115/#119 | Merge the current base without rewriting history, repair the conflict, then re-evaluate |
| 112 | `factory/issue-96-runtime-shell` @ `eb5a55b09c709bbe0292c978b025fa07783113d1` | `community` | `CONTINUE` | `#96` | `Closes #96` needs final acceptance review before leaving draft | Separate draft slice on the same issue as #119; overlaps frontend/docs with #115/#119 | Continue only after explicit adoption; preserve draft state |
| 115 | `factory/issue-98-frontend-build` @ `16da9717822cabc33ca39d39ca896aae7280b0fb` | `community` | `CONTINUE` | `#98` | `Progresses #98` correctly avoids premature closure | Draft sibling on the community base; overlaps runtime/docs with #112/#119 | Continue its remaining draft acceptance work after adoption |
| 119 | `factory/issue-96-product-evidence-harness` @ `08a0d5f0f3630630c2d86190bb157b8fe0f00f65` | `community` | `CONTINUE` | `#96` | `Progresses #96` correctly identifies a partial evidence slice | Separate draft evidence slice for the same issue as #112; overlaps frontend/docs with #112/#115 | Preserve as a distinct adopted slice; do not close #96 from this PR |
| 128 | `chore/community-product-surface-cleanup` @ `bfc3c7276afb2d4d115cb68d66966da1533f7a0f` | `community` | `CONTINUE` | `#125` | `Progresses #125` is correct | Later community checkpoint on a newer base; not a hard predecessor | Continue verification/repair before review |

## Notes

- `#112` and `#119` are separate owned slices on the same issue (`#96`) and should remain distinct.
- `#109` is blocked by merge conflict, not by missing CI.
- All five PRs are currently drafts. `#109` is additionally blocked by a merge
  conflict; the other four remain continuation candidates rather than release
  candidates even where their current CI is green.
- No stack ancestry forced a replacement branch or history rewrite.

## Brownfield Inception archaeology

The read-only audit is recorded in
`docs/dogfood/brownfield-inception-archaeology-2026-08-22.md`. It classifies the
#711/#751 policy sequence as a regression of #695's brownfield repository
inspection contract. No Brownfield repair or live Inception operation is part
of this Continuity branch.
