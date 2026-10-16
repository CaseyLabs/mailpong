# Git-native project management

Read this when managing roadmap work, recording issues or preparing continuation. For unrelated changes, read applicable AGENTS.md and only relevant documentation; loading the whole portfolio/register is unnecessary.

## Documents and identifiers

- `PLAN.md` is the authoritative portfolio: project objective/status/priority, ordered milestones, acceptance/validation, dependencies and issue links. Its Portfolio and Progress sections identify candidates and active work.
- `ISSUES.md` is the authoritative unresolved register: one primary owner per issue, original severity, proposed implementation priority, evidence/uncertainty, dependencies and closure criteria. Cross-project relevance is a link, never a duplicate entry.
- Substantial selected milestones use a living `docs/plans/<milestone-id>.md` ExecPlan, linked from their roadmap milestone. No plan is needed for every backlog item; simple tasks can keep a short checkpoint under the roadmap milestone. Existing detailed proposals remain references until selected and reconciled.
- Technical behavior/architecture belongs in the relevant `docs/` reference. Durable verification evidence and limitations belong in `docs/validation/`. Git history preserves routine execution chronology and old identifiers.

Use immutable, unique IDs and explicit HTML anchors (`<a id="send-1"></a>`) with standard Markdown links (`[SEND-1](../PLAN.md#send-1)`). Titles can change without breaking links. Project IDs use a short uppercase name; new milestones use `<project>-<number>` (Wails research uses W3-M0–W3-M7). Preserve legacy G4, D2 and existing issue IDs, including F5b-1–F5b-3. Do not recycle IDs after closure; check register and Git history before allocation. Unnamed sync/read findings now use SY1–SY6; the G4 follow-up uses AI6. Original Wails M0–M7 map to W3-M0–W3-M7 to distinguish them from older projects' M numbers.

Each issue links its project and relevant milestone(s); each milestone links its issues and sits under its project anchor. A project links its register section. If work spans projects, put the issue under one owner and add links from the other project/milestone. An investigation may yield a new proposed milestone; it cannot implicitly approve implementation.

## Status, priority and eligibility

Projects and milestones use `proposed`, `active`, `blocked`, `deferred`, `done`. Issues use `open`, `active`, `blocked`, `deferred`, `resolved`. Evidence certainty is a separate attribute, not a status. `blocked` identifies a specific unsatisfied gate; `deferred` records a decision to wait (and can list an upstream blocker). `proposed`/`open` permit consideration, not execution. `active` requires an explicit request or recorded prior authorization. `done`/`resolved` require proved acceptance/closure criteria, with limitations retained.

Priorities: P1 privacy, security, data integrity or consequential correctness; P2 functional follow-ups; P3 maintenance or optional investigation. Priorities newly assigned during reorganization are recommendations, not owner decisions. Preserve severity independently; do not infer a severity for an unverified follow-up or legal flag. Dependencies override priority. Order prerequisites first, then higher priorities among independent work; do not invent dependencies from thematic grouping. Include external gates separately from milestone prerequisites.

A candidate is eligible for selection when its known prerequisites permit its scope. It becomes executable only when the user's request authorizes that scope. Starting the [unattended runner](../CONTRIBUTING.md#unattended-milestone-runs) authorizes the local scope of each milestone it selects, one at a time; it never reopens deferred work or waives an external gate. A local investigation can be selected while its live test or implementation remains blocked. Deferred work, including G4 and D2, requires explicit reopening. Outside such a run, if multiple candidates materially differ in scope, present candidates and obtain selection instead of implementing the first unfinished entry.

## Lifecycle

### Add an issue

Search the relevant register section and IDs for duplicates. Assign a stable unused ID; associate an existing project, or justify a small proposed project for a distinct outcome. Link an existing relevant milestone or propose one, with reverse links. Record problem, affected symbols, impact, original severity (or unspecified), recommended priority, status, evidence/date, verification limits, reproduction if known, dependencies and closure criteria. Record confirmed findings left unfixed in the same task. Reports not yet verified stay labeled as such. Adding an entry does not authorize a fix.

### Recurring problems

`ISSUES.md` Recurring problems records environment and tool symptoms with their workaround and an occurrence count. Check it before debugging. On a second occurrence, or a costly first one, promote the workaround into the applicable guidance and delete the tally.

### Start or resume one milestone

Check Git status and preserve unrelated work. Read Portfolio/Progress, the selected project/milestone and linked issues/reference sections only. Reconcile scope with implementation and dated evidence; ask about material scope changes. Confirm prerequisites, gates and acceptance-to-validation mapping. Record authorization and any live/upgrade/migration exclusions; change only selected statuses to active. A blocked gate is not waived by selecting adjacent local work.

Create an ExecPlan only for sufficiently complex selected work. Include:

- Goal, scope/exclusions, project/milestone/issue links and authorization.
- Necessary repository context, symbols and implementation approach; explain decisions a fresh agent cannot infer.
- Dependencies, risks, concrete steps and checkboxes under `## Progress`.
- Acceptance criteria mapped to exact commands, fixtures, expected outcomes and external gates.
- `## Surprises & Discoveries`, `## Decision Log`, `## Outcomes & Retrospective` as living sections.
- Checkpoint: branch/base revision, current step, next concrete action, changed files, remaining work, exact command results, unrun checks and blockers; distinguish pre-commit base from subsequent revision.

An ExecPlan must support continuation without chat history. Include essential facts in the plan and link maintained local references for detail; avoid creating parallel sources of status. Link the plan from the roadmap; roadmap status/summary stays authoritative, detailed progress stays in that plan. Small tasks use the same checkpoint facts inline without the full structure.

### Implement and checkpoint

Keep the active checkpoint current at meaningful completions, discoveries, blockers, scope changes and before handoff/compaction. Update related issue evidence when facts change; avoid rewriting unrelated projects. Run proportionate checks and distinguish failed, passed and unrun results. Do not claim live compatibility from local fixtures. Respect external authorization gates in PLAN, issues and technical references; roadmap organization grants no provider, email, credential, migration, tenant/DNS or dependency permissions.

### Session boundaries

Run one milestone per session; start the next milestone in a fresh session from the committed roadmap and checkpoint, not by continuing a long conversation. A persisted handoff is preferred to repeated compaction: when the host reports context usage nearing its compaction point (Claude Code `/context`, Codex `/status`), finish the current sub-step, checkpoint everything the remaining work needs and use project-handoff. Because that signal is easy to miss, a session that has already been compacted once (it resumes from a summary) does not continue into a second compaction: it hands off at the next sub-step boundary. Automatic compaction stays enabled as the fallback for work interrupted mid-step; the checkpoint, not the summary, is the continuation state. No fixed token threshold applies: context windows and compaction points vary by model and host configuration.

### Complete and stop

Use project-finish-milestone for checks, independent acceptance review, correctness fixes and behavior-preserving simplification under authorized scope. Mark done only when acceptance is proved; outstanding required checks keep work active or blocked. Resolve only issues actually fixed and verified; a design milestone can be done while its implementation issue stays unresolved. Preserve constraints and unresolved follow-ups.

Move lasting behavior/decisions to technical docs and verification evidence to validation records. Collapse completed roadmap details to a short ID/status/evidence link while referenced; remove superseded detail and resolved register entries once no live reference needs them. Preserve an anchor or update all live links before removal; IDs remain traceable in Git. Do not archive entire routine plans merely for chronology. Commit/push/landing follows repository policy subject to explicit user/runtime limits; a no-commit request always wins. Completing one milestone never authorizes the next; an unattended run selects each one afresh.

After completing a milestone, record the next recommended milestone in roadmap Progress as one line, `**Next recommended:** [ID](#id) — reason`, replacing any earlier one. Choose it by the eligibility and ordering rules above; if no milestone is eligible, name the best issue to turn into one, or what would unblock work. A recommendation is not authorization: the next session still needs selection, but a bare continuation request offers it first.

### No active milestone / handoff

State “No active milestone” in roadmap Progress, keep the **Next recommended** line current, and identify candidates and blockers without changing their statuses. A handoff names the project/milestone IDs, roadmap section, execution checkpoint, checkout/branch/revision/dirty state, next authorized action, results/limits, and commit/push status. If no work is authorized, its next action is selection or blocker resolution, never an invitation to activate deferred work.

## Basis and conventions

Reviewed official guidance on 2026-10-14: [AGENTS.md discovery](https://learn.chatgpt.com/docs/agent-configuration/agents-md) supports concise layered instructions; [skills](https://learn.chatgpt.com/docs/build-skills) load detail progressively; [Codex commands](https://learn.chatgpt.com/docs/developer-commands) document `/plan` for proposing execution before implementation. Plan mode is a host behavior, distinct from these Git documents; obey the actual host restrictions. Session guidance follows Codex [best practices](https://learn.chatgpt.com/guides/best-practices) (one chat per coherent unit of work; `/compact` and automatic compaction for long chats) and Claude Code [best practices](https://code.claude.com/docs/en/best-practices) (`/clear` between unrelated tasks; a fresh session with a precise prompt over accumulated corrections). Claude re-injects project CLAUDE.md and invoked skill bodies after [compaction](https://code.claude.com/docs/en/context-window); conversation-only state is summarized, so it must be checkpointed.

The [OpenAI ExecPlan recipe](https://developers.openai.com/cookbook/articles/codex_exec_plans) is currently marked archived. We adopt its living, restartable, outcome/validation-focused principles, not its model suggestions, automatic next-milestone execution or frequent-commit instructions. An additional `.agent/PLANS.md` would duplicate this existing shared policy/skill arrangement and is unnecessary. `CLAUDE.md` imports AGENTS.md; `.claude/skills` links the canonical `.agents/skills` tree. Keep one workflow policy, with native settings in existing adapters.
