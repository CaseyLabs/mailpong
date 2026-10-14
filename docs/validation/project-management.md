# Project-management reorganization validation — 2026-10-14

Initial reorganization scope: documentation and shared workflow instructions only. No application code, feature/issue implementation, live integration checks or dependency/configuration changes. No commits or pushes occurred during that initial scope; the owner subsequently authorized committing and pushing these reviewed changes to main. No milestone was activated; G4 and D2 remain deferred. Initial checkout was clean on `main`.

## Coverage and mapping

All 21 original unresolved finding blocks from pre-reorganization `21e190b:ISSUES.md` are retained verbatim exactly once, including severities, dates, locations, uncertainty and recorded fixture evidence. Product findings were not independently reverified. Existing named issue IDs are unchanged; six unnamed findings now have stable IDs:

| New ID | Original location / finding | Owner / milestone |
| --- | --- | --- |
| SY1 | Gmail/Graph read clients: duplicated reader logic | MAINT / MAINT-2 |
| SY2 | Graph GetRaw: oversized message blocks cursor | SYNC / SYNC-3 |
| SY3 | Graph deltaRun: seen IDs and per-message reconciliation cost | SYNC / SYNC-4 |
| SY4 | Gmail reconcile: all remote IDs held in memory | SYNC / SYNC-4 |
| SY5 | Gmail reconcile: minimal GET reset cost and 5xx classification | SYNC / SYNC-4 |
| SY6 | parseAPIRaw: missing Date uses ingest time | SYNC / SYNC-3 |

AI6 (the next unused AI issue ID; AI1–AI5 remain historical) names the existing G4 runtime/context follow-up, without asserting a verified defect. Existing L2/L3 legal flags are retained from current draft comments and original `33c1851:PLAN.md`; current sign-off is unknown, not presumed absent or complete. No old resolved issue is reopened. Original Wails proposal M0–M7 remains in `docs/PLAN-migration.md`, mapped to W3-M0–W3-M7 to avoid historical ID collisions; no assessment was executed.

The register has 24 entries: SEND 3, API 4, SYNC 8, CHAT 1, MAINT 7, AI 1, WAILS 0. The portfolio has 26 proposed/blocked/deferred milestones. New priorities are recommendations: P1 for encryption/Bcc/Microsoft compatibility and API re-key integrity; P2 for functional/migration/ingest/UX follow-ups and external legal review; P3 for bounded-cost research, docs/refactoring, upstream dependency compatibility and AI/Wails research. These do not change original severity or authorize implementation.

## Checks and limitations

- `git diff --check`: passed.
- One-off local Python checks (no validation framework added): all 21 original issue blocks occur once; 24 unique issue and 26 milestone anchors; project ownership counts; all milestone/issue links bidirectional; explicit prerequisite edges precede dependents and are acyclic; local Markdown paths/fragments in changed files resolve. G4/D2 remain deferred, selected candidates remain not started. New validation-document links were also checked.
- `python3 .agents/skills/project-next-milestone/tests/paths.py`: passed default/explicit/missing/spaced plans and root/subdirectory/disposable-worktree resource identity. Existing outline helper required no change: it exposes roadmap headings and Progress for targeted reading.
- `skill-creator/scripts/quick_validate.py` for project-handoff and project-finish-milestone: passed. For project-next-milestone it rejects the preserved `model` field. This pre-existing generic-schema limitation is documented in CONTRIBUTING and the G3 record; Python YAML validation confirmed the existing Claude-only `sonnet` value is unchanged and no permission fields were introduced. No native host execution or new context-savings claim follows from parsing.
- Independent read-only review confirmed original issue preservation and walked: bare continuation with no active milestone requires selection; explicit SEND-1 selection proceeds within scope; completed API-2 design leaves F5b-1 unresolved and API-3 gated; no-commit handoff leaves an accurate uncommitted checkpoint. Three guidance wording gaps were corrected (linked plans, commit authorization, canonical shared skill paths) and directly rechecked.

No dedicated Markdown/link checker exists in the inspected repository tooling. Targeted one-off checks and diff review were used; external URLs in carried technical records were not exhaustively revalidated. Current official OpenAI guidance was fetched separately and is linked in [the shared procedure](../PROJECT_MANAGEMENT.md#basis-and-conventions). Its ExecPlan recipe is marked archived; only its restartable, living, evidence-focused principles were adopted. Historical validation files were not rewritten, and local completion remains distinct from live compatibility.

## Remaining selection questions

No product milestone is active. SEND-1 is the recommended next selection; its encryption-failure policy still needs scope reconciliation. SYNC-1 is an independent high-priority alternative. Live gates require specific authorization; legal sign-off and Wails deliverable-path reconciliation remain explicit. G4 and D2 cannot be resumed by a generic continuation request.
