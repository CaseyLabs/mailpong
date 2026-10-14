# MAINT-5 — Compact project workflow helpers

[Milestone](../../PLAN.md#maint-5). Authorization: user requested implementation of all four session-audit recommendations on 2026-10-14. Branch `feat/project-workflow-helpers`, base `e2e64b8`. This selects only these helpers; G4/D2 remain deferred.

## Scope and design

Implement `tools/project-context.py`, `tools/project-check.py`, `tools/project-land.sh` and `tools/check-doc-links.py` with Python standard library and a thin shell wrapper for landing. Integrate canonical skills and existing developer docs. Keep decision/authorization/review in the skills: context is read-only, checks plan before execution, landing previews unless --execute. Use explicit argv, compact stdout, full logs and machine-readable evidence. No automatic dependency installs, live provider calls, hidden check omission or global configuration edits.

Context/link worker owns context loader, Markdown parser, link checker and their tests. Primary owns Git/common mechanics, check runner, landing implementation, integration docs and final tests. Only primary runs tests/builds.

## Progress

- [x] Inspect existing helpers and define interfaces/safety rules.
- [x] Implement context/link tools and fixtures.
- [x] Implement check/landing tools and temporary-repository fixtures.
- [x] Integrate skills/developer guidance and run real compact helper checks (full checks pending).
- [x] Independent acceptance, fixes/simplify and evidence/status reconciliation.
- [x] Ready for one milestone commit and guarded landing under project policy.

## Acceptance and verification

Acceptance is the MAINT-5 roadmap criterion. Prove with `python3 -m unittest discover -s tools/tests -p 'test_*.py'`, `python3 .agents/skills/project-next-milestone/tests/paths.py`, real context/docs/check-plan/landing-preview commands and a logged tooling run. Test failed command exit/log retention, missing plan/ID/link errors, worktree/path spaces, frontend full-suite selection, stale/partial check evidence, clean-tree gate, default preview, upstream/push errors and divergent-main rebase/conflict abort. Use local bare remotes and hooks only inside fixtures. No Mail UI/native tests required for tooling-only changes.

## Decisions

Use one Markdown parser for source sections/anchors. Landing checks content-bound evidence matching the current source files, changed scope and clean status; committing identical content retains evidence validity, including deleted files; a rebase invalidates evidence and requires rerunning recorded checks. Preserve a recovery branch before rewriting/landing. No script grants authorization or chooses a milestone.

## Checkpoint

Complete; final recorded runner refresh and commit/landing remain as mechanics. Changed: four CLI entrypoints, shared Git/Markdown modules and tests under tools/, canonical next/finish/ship skills, .gitignore, AGENTS.md, CONTRIBUTING.md, PLAN.md, this checkpoint and validation evidence. No confirmed findings left unfixed. Exact checks: `python3 tools/project-check.py --base main --run` PASS all five planned checks (whitespace, changed-doc links, 30 helper tests, skill worktree paths, brand); evidence/logs are private and source-bound. Independent native acceptance PASS on the final code and real logs. Direct correctness review substituted for a separate review tool; simplify review reused common Git/Markdown primitives and found no further warranted changes. Finish/ship skill generic validators PASS; next-milestone validator rejects its existing documented Claude model extension, and HEAD frontmatter equality proves it was not changed. No application Go/UI/native/live tests were run for this tooling-only task. Source edits to the final checkpoint/status/evidence invalidate the prior manifest; run the helper once more after these docs are final, then commit, preview and execute landing without source changes. The landing helper itself will be exercised on this branch. No active milestone remains; select the next work in a fresh session.

## Discoveries / outcomes

All four recommendations implemented. The existing outline helper remains for compatibility; the new loader scopes context without selecting work. Relative directory links browse repository paths; /docs/ routes require real pages. Recovery branches are retained intentionally. Initial test failures (Git upstream shorthand and a baseline fixture mismatch) were corrected; strengthened fixtures also cover deletions, check failure after rebase and hook-altered commit trees. The earlier reviewer objection to relative directories was clarified as a contract distinction, not an unresolved defect. No new confirmed defects remain. G4/D2 remain deferred; output character counts do not establish token/billing savings.

## Review follow-up checkpoint — MA6

User authorized both P2 review fixes on 2026-10-14. Branch `fix/landing-evidence-coverage`, base `ddaf5d5`; [issue](../../ISSUES.md#ma6), [validation](../validation/maint-5.md#review-follow-up-ma6). Complete: common-ancestor check baselines, complete landing evidence ancestry/path coverage, five new regression fixtures and fresh-divergence failure coverage. Changed files: `tools/project-check.py`, `tools/project_land.py`, `tools/tests/test_workflow.py`, `CONTRIBUTING.md`, `PLAN.md`, `ISSUES.md`, this checkpoint and validation record. No unresolved findings; no new milestone activated.

Checks: focused workflow suite PASS (20 cases); initial full suite PASS (33); final `python3 tools/project-check.py --base main --run` PASS (35 tests, whitespace/docs/skill paths/brand). Independent acceptance PASS; direct diff and simplification review complete. Next: refresh source-bound check evidence after documentation, commit all fix files, preview and execute guarded landing under repository policy. No app/backend/UI/live checks required for these helper-only fixes.
