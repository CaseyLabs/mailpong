# MAINT-5 workflow helper validation — 2026-10-14

[Milestone](../../PLAN.md#maint-5), [checkpoint](../plans/maint-5.md). Branch `feat/project-workflow-helpers`; pre-commit base `e2e64b8`. User authorized all four audit recommendations; deferred G4/D2 are unchanged.

## Delivered behavior

- `tools/project-context.py`: read-only Portfolio/Progress or exact milestone/project context, linked issues and checkpoints with source lines and bounded output; no selection or authorization.
- `tools/project-check.py`: changed-scope union, plan by default, serial logged execution, failure/unrun visibility, private evidence bound to source content; preserved full frontend/Go checks and explicit additional argv checks.
- `tools/project-land.sh` / `project_land.py`: read-only preview; execution requires clean feature checkout and matching complete evidence. Retains recovery branches, aborts conflicts, reruns checks after rebase, preserves configured identity/upstream/hooks, rejects hook-altered source/commit trees before push, and deletes the feature only after successful push and equal trees.
- `tools/check-doc-links.py`: changed Markdown by default, explicit paths relative to invocation cwd, local/site links and anchors, code masking, references, Unicode/code/setext/duplicate headings; bounded errors and optional new full JSON report. Relative repository directory links require a directory; `/docs/` routes require a real user-guide page. Existing report files are not overwritten.

Existing next/finish/ship skills and CONTRIBUTING/AGENTS now call these helpers and reuse required unchanged guidance within one session. New sessions load required instructions again. Acceptance/review and external/live authorization remain with the workflow, not scripts.

## Passed checks and review

- `python3 tools/project-check.py --base main --run`: all five checks passed: `git diff --check`, changed Markdown links, `python3 -m unittest discover -s tools/tests -p 'test_*.py'` (30 cases), existing `project-next-milestone/tests/paths.py`, and `make lint-brand`. Full logs/evidence under `~/.cache/mailven-project-checks/run-*/`; console summary in `/tmp/mailven-maint-5-check-run.log` is ephemeral. Final manifest must be generated after checkpoint/status documentation is final.
- Disposable fixture coverage: missing/ambiguous/exact milestone IDs, selected-only context and missing references, path spaces/worktrees, local paths and site pages, fenced code, references and heading anchors, error limits/report preservation; complete frontend check plans, source fingerprints including deletions/modes/untracked files, serial-operation lock, failed checks/log retention; read-only landing preview, stale/partial evidence, configured identity/upstream, other worktrees, successful landing, conflict abort, post-rebase check failure, hook rejection/alteration and rejected push with retained recovery.
- Real context/check-plan/link CLI runs passed. Landing preview/execution is the final authorized closeout action on this verified branch; fixtures already prove their success/failure paths without a production push.
- Independent native `project-acceptance-checker`: final PASS against the MAINT-5 acceptance criterion, based on final source and the real 30-case runner evidence. No added stubs or unresolved findings. Direct diff/correctness review substituted for a separate code-review tool. Simplify review reused small common Git/Markdown primitives and avoided additional abstraction.
- Generic skill validation: finish/ship PASS. Next-milestone reports `Unexpected key(s): model`, the existing documented Claude-only extension; comparison with HEAD proves frontmatter is identical. This schema limitation is not a new regression and the field is preserved.
- `git diff --name-only -- archive LICENSE`: empty; attribution/reference content untouched.

Initial unit run failed eight cases: seven due to unsupported full-ref upstream shorthand and one due to an incorrect fixture baseline. Both causes were corrected, and final tests passed. A later directory-link review concern was resolved by documenting and testing the distinction between repository links and site routes.

## Context-size observation and limits

An illustrative pre-closeout run returned 29 lines / 3,530 characters for overview context and 58 lines / 6,046 characters for MAINT-5 context, compared with 59,223 characters for the whole PLAN plus ISSUES. Scoped output deliberately omits unrelated work; these are text-size observations, not measured tokens, cost savings or cross-host runtime evidence. G4 remains deferred.

No application Go/frontend/UI/native/provider tests, dependency changes or account actions were needed or run. Helper fixtures execute only disposable Git repositories/remotes and fake commands; real landing follows repository authorization. Windows runtime and fresh Claude/Codex workflow execution trials were unrun. Checks cannot replace human/agent review, prove live provider compatibility or authorize additional milestones. No confirmed finding was left unfixed.

## Review follow-up MA6

User authorized fixes for both P2 review findings on 2026-10-14; [issue MA6](../../ISSUES.md#ma6). Branch `fix/landing-evidence-coverage`, pre-commit base `ddaf5d5`.

`project-check.py` now records the common ancestor of the supplied base and HEAD, so the documented `--base main --run` command checks feature changes when main has diverged. Landing requires that recorded baseline to be an ancestor of the landing merge base and verifies actual changed-path coverage independently of the evidence path list. A later feature baseline is refused even if scopes match. The path check also rejects older baselines that hide a feature edit restoring earlier content. Safe older evidence and custom `--main` remain supported; post-rebase checks still run before squash.

Passed `python3 tools/project-check.py --base main --run`: whitespace, changed-doc links, all 35 disposable helper tests, skill/worktree path checks and brand check. Initial focused workflow run passed 20 tests; the full suite passed 33 before the final two acceptance fixtures were added. New fixtures cover unchecked earlier Markdown, older restored-file omission, safe older evidence, a custom target, and fresh diverged-main evidence with check reruns; the existing post-rebase failure fixture now generates evidence after main diverges. No failed checks. Final source-bound evidence is regenerated after these documentation updates.

Independent `project-acceptance-checker` reviewed the implementation, fixtures, documented commands and preserved landing gates read-only: PASS, no defects. Primary diff/simplification review found no warranted additional abstraction. No application Go/frontend/UI/native or live provider checks were needed or run; fixtures use disposable repositories and bare remotes.
