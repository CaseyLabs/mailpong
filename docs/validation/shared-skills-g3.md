# G3 runtime verification — 2026-10-12

Status: incomplete. Base: `83cd04b` on `docs/shared-ai-policy`.
Screenshot tooling was repaired; no application code, dependencies or host
configuration changed.

## Environment and discovery

Installed Claude Code: 2.1.294. Installed Codex CLI: 0.162.1.
Disposable symlink-preserving clone:
`git clone --quiet --no-hardlinks /home/user/sync/git/projects/mailven /tmp/mailven-g3-wqlKpr/fixture`.
Its origin was removed before trials. Starts used that root and its `frontend/`
directory. `.claude/skills` remains `../.agents/skills`; Codex sources are
`.agents/skills/<name>/SKILL.md`. No custom discovery flags or personal skill
installations were added. Existing user/global configuration remained active;
these are ordinary configured-host observations, not isolated-default results.

All five project names (`project-next-milestone`, `project-finish-milestone`,
`project-handoff`, `project-ship`, `run-mailven`) appeared exactly once:

- Claude: observed in `system/init.skills` and `system/init.slash_commands`
  for both cwd values. Model execution failed with HTTP 429
  `usage_limit_reached` (seven-day limit; reported reset Oct 13, 9pm Vancouver).
  Registration does not prove policy loading, linked-body reads or execution.
- Codex: both completed responses listed all five canonical fixture paths,
  one entry each, and correctly reproduced the persisted Aerion identifier rule.
  No tool calls occurred. This is the model's report of its startup registry,
  not an independently exported registry. Policy loading was behaviorally observed.

Commands, run separately from root and `frontend/`:

```sh
timeout 30 claude -p 'Read-only discovery check; reply OK without tools.' --permission-mode plan --no-session-persistence --output-format stream-json --verbose
timeout 90 codex exec --sandbox read-only --ephemeral --json 'Read-only discovery check. Without searching the filesystem for skills, list the five Mailven project skills from your startup registry, their source paths and duplicate counts. State the project rule for persisted Aerion identifiers from loaded guidance. Do not invoke workflows or change files.'
```

The Codex subdirectory prompt used “with source paths and duplicate counts”
instead of “their source paths and duplicate counts”; otherwise equivalent.
The initial Claude quota probe ran from `/tmp`; it is excluded from project
registration evidence. Project-root and subdirectory probes confirmed discovery
without recovering model access.

Explicit Codex selection, from fixture `frontend/`, was attempted with the same
`timeout 90 codex exec --sandbox read-only --ephemeral --json` command and:

```text
$project-next-milestone Use absent-g3-plan.md. This is a read-only disposable-fixture smoke test: do not create a plan, mutate files, commit, push or delegate. Follow the skill through the missing-plan blocker, then report the loaded skill path, invoked helper and result.
```

It failed before execution with a usage-limit error (“try again at 2:04 PM”).
No skill-body or helper-execution claim follows from that attempt.
Natural-language triggers/non-triggers and both cross-host continuations remain
unrun. Fixture working tree remained clean; no test pushed or advanced a milestone.

## Context evidence

Codex completed-turn usage reported 19,630 input tokens (7,168 cached) at root
and 19,632 (0 cached) in `frontend/`. These are full-session API usage counters,
including personal configuration and all available skill metadata. They neither
isolate project guidance nor establish savings against a baseline. Claude
execution reported zero input/output usage because the request was rejected;
that is not a context-size measurement. Equivalent `/context` diagnostics and
accuracy/extra-read comparisons remain incomplete. G1/G2 byte and word counts
in PLAN.md remain reproducible proxies only.

The [official skills documentation](https://learn.chatgpt.com/docs/build-skills)
was re-opened on 2026-10-12. Documentation is distinct from the observed results
above; discovery is not proof of workflow compliance.

## UI lifecycle

`xdpyinfo` confirmed a usable existing display `:0`; no Xvfb was started.
`dev.sh status` was `down`, then `.agents/skills/run-mailven/dev.sh start` and
`wait` succeeded. Headless Chromium used the existing Playwright launcher.

```sh
node /tmp/mailven-g3-wqlKpr/ui-smoke.js
.agents/skills/run-mailven/dev.sh stop
.agents/skills/run-mailven/dev.sh status
```

The initial disposable guarded smoke script reads UI state from a bindings-only page, blocks app mount
there, and installs the existing screenshot fakes before mounting the test page.
No real message or composer is opened. It checks both toggle states after 1.7s,
writes `mail-expanded.png` and `mail-collapsed.png`, reloads to remove fakes,
and compares persisted UI state. Both screenshots were visually inspected:
fake Mail renders with the Accounts column present/absent as intended.

Output: `PASS fake Mail Accounts expanded`; `PASS fake Mail Accounts collapsed`;
`PASS fakes removed; persisted UI state unchanged; no unexpected page errors`.
Cleanup output: `stopped`, then `down`. Browser pages closed in `finally`.
No fake server was needed; fakes were injected from disk into the served entry
module. No native-window/dock check was performed. No generated binding diff
remained. This verifies the guarded smoke path, not the older screenshot helpers.
The old helpers enabled Contacts/Calendar in the real profile before installing
fakes and did not restore those settings. This confirmed finding was repaired:
`fake-page.js` now guards mount and verifies UI plus both extension settings;
`headless-shot.js` and `ui-matrix.js` share it. No live setup setters remain.
Both final helper runs passed (5 README/crop PNGs and 36 matrix PNGs), including
unchanged persisted UI/extension assertions. Representative dark/light images
in both Accounts states were visually checked. Reproduce with:

```sh
.agents/skills/run-mailven/dev.sh start
.agents/skills/run-mailven/dev.sh wait
node .agents/skills/run-mailven/headless-shot.js /tmp/mailven-g3-wqlKpr/repaired
node .agents/skills/run-mailven/ui-matrix.js /tmp/mailven-g3-wqlKpr/repaired g3
.agents/skills/run-mailven/dev.sh stop
```

Create the output directory before running. No fake server is needed.

Raw JSONL, errors, screenshots and the original smoke script remain under
`/tmp/mailven-g3-wqlKpr` (ephemeral). This tracked summary and repaired helpers preserve
the commands, findings and limitations without private account state or raw logs.

## Review and failure cleanup

`node --check` passed for `fake-page.js`, `headless-shot.js` and `ui-matrix.js`;
`git diff --check` passed. The callback-failure path was exercised with
`withFakePage({width:1400,height:800}, async () => { throw new Error('intentional cleanup test') })`:
the persisted-state check passed and the intentional error reached the caller
after browser closure. Dev cleanup again reported `stopped` and `down`.
Application builds/tests were not required: changes affect verification tooling.

Independent read-only acceptance review: UI PASS; skill execution, bidirectional
continuation and context reduction UNVERIFIABLE. No code defects or added stubs
were found. Direct diff review covered correctness; simplify review consolidated
the shared setup and removed obsolete frame aliases. G3 stays open. The user requested a local checkpoint commit on 2026-10-12
while Claude quota remains blocked; no push or merge. AI5 remains open.

## Model and effort routing repair

Compared tracked configuration at `cd37705` with G1/G2 and restored it in the
G3 follow-up requested by the user on 2026-10-12 (base `58a9f49`):

| Configuration | Before migration | Restored configuration |
| --- | --- | --- |
| next-milestone skill | `model: sonnet`; no explicit effort | Same Claude turn override in the one canonical skill; Codex keeps session model/effort |
| Claude acceptance agent | Sonnet, `effort: low` | Preserved; native role required for acceptance |
| Claude worker agent | Sonnet, `effort: medium` | Preserved; native role required for general delegated work |
| Codex project roles | none | GPT-6.1 Sol, low acceptance / medium worker |

Both hosts use the names `project-acceptance-checker` and `project-worker`.
Native adapters point to the same shared briefs. No permissions, session-wide
model defaults, personal configuration, wrapper skills or duplicate bodies were
added. Specialist workflows keep their own agent selection. A missing acceptance
role falls back to the native worker with the read-only acceptance brief; if both
are missing, the workflow discloses that its pins were not applied before using
an available read-only agent or direct review. Explicit user/runtime settings and
model availability still take precedence.

The old `/code-review medium` invocation was a skill argument, not verified
model reasoning-effort configuration. It remains capability-based review.

Official docs checked 2026-10-12: [Claude skills](https://code.claude.com/docs/en/skills)
support per-skill model/effort fields; [Claude subagents](https://code.claude.com/docs/en/sub-agents)
support model/effort on native agents; [Codex subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)
support `model` and `model_reasoning_effort` in native `.codex/agents/*.toml`.
To retain one symlinked skill tree, the Claude-only `model: sonnet` field is an
explicit exception to the earlier name/description-only frontmatter rule. Its
YAML comment and skill body distinguish it from a Codex model request. No new
Claude effort override was introduced.

### Configuration validation

- Python `tomllib` parsed both Codex definitions. Each has only name, description,
  model, reasoning effort and developer instructions; both resolve an existing
  shared brief and define no permission override.
- Python `yaml.safe_load` confirmed the unchanged Claude Sonnet/low and
  Sonnet/medium definitions, and restored milestone Sonnet with no effort field.
  `Path.samefile()` confirmed the canonical and Claude-linked skill are identical.
- Codex CLI 0.162.1 native `skills/list` returned all five project skills enabled
  exactly once, with zero errors, for both root and `frontend/` after adding
  the Claude-only field. This proves loader compatibility, not turn-model behavior.
- Codex `model/list` returned `gpt-6.1-sol` with both low and medium in
  `supportedReasoningEfforts`. This checks installed catalog support, not inference
  availability under the account quota.
- `python3 /home/user/.codex/skills/.system/skill-creator/scripts/quick_validate.py
  .agents/skills/project-finish-milestone` passed. Running the same validator on
  `project-next-milestone` failed solely with `Unexpected key(s) in SKILL.md
  frontmatter: model`. Its generic allowed-key list excludes the intentional
  Claude extension; native Codex loading and direct YAML checks supplement it.
  The validator was not modified or reported as passing.
- `git diff --check` passed. No application tests/UI reruns were needed because
  this follow-up changes only agent configuration, guidance and documentation.

The native loader/catalog probe launched `codex app-server --stdio` from the
checkout, sent `initialize` with experimental API enabled, then `initialized`,
`skills/list` with `cwds` set to the root and `frontend/` and `forceReload: true`,
and `model/list`. It made no inference request and stopped its server in `finally`.
The generated protocol schemas came from
`codex app-server generate-json-schema --experimental --out /tmp/mailven-routing-leg9mP/schema`.
Probe script, selected JSON evidence and logs are under
`/tmp/mailven-routing-leg9mP` (ephemeral); no private configuration is copied here.

Actual native agent discovery/selection and applied model/effort still require
fresh-session runtime checks after quotas recover. The current session's role
catalog predates the added Codex adapters; its existing reviewer is a disclosed
fallback, not proof that the new roles were spawned. AI6's configuration defect
is repaired; remaining runtime proof stays in G3, alongside AI5's handoff checks.

Independent read-only follow-up review passed preservation of Claude pins and
required selection, Codex model/effort/shared-brief configuration, the single-tree
Claude skill override, and honest fallback/runtime reporting. A wording concern
was corrected to describe the Claude field as a supported declaration rather
than an observed runtime override. Direct diff and simplification review found
no additional changes warranted. This review used the pre-existing explorer
role because this session did not load the new native project roles.

## Codex-only follow-up — 2026-10-12

User scope: verify Codex for now; Claude was not invoked. Base `3f21369`,
branch `docs/shared-ai-policy`; installed CLI remains 0.162.1. Fresh ordinary
configured-host sessions used a new symlink-preserving local clone at
`/tmp/mailven-codex-g3-zyztr6lq/fixture`, with its origin removed. No custom
skill discovery flags, personal installations, model/effort overrides or
configuration changes were used. Passing discovery and UI checks were retained.

All smoke commands used `codex exec --sandbox read-only --json PROMPT` with
stdin closed and a 120s (explicit) or 240s (others) timeout. All except the
native routing trial used `--ephemeral`. Prompt text and JSONL are under
`/tmp/mailven-codex-g3-zyztr6lq` (ephemeral). Tests were serialized.

| Trial / cwd | Prompt | Observed result |
| --- | --- | --- |
| Explicit / `frontend/` | `$project-next-milestone Use absent-g3-plan.md.` Read-only; stop at missing plan; no changes or delegation. | PASS: read canonical skill; ran `sh "<fixture>/.agents/skills/project-next-milestone/plan-outline.sh" "<fixture>/frontend/absent-g3-plan.md"`; helper exit 1, missing-plan message, safe stop. |
| Natural / root | `Continue the next milestone using absent-natural-plan.md.` Read-only; stop if missing. | Selection PASS; step compliance FAIL: read canonical skill but used `test -f` instead of mandatory outline helper; safe stop. Recorded AI7, left unfixed under verification scope. |
| Non-trigger / root | `Read-only conceptual question: what is a software milestone? Answer in one sentence. Do not inspect files or run any workflow.` | PASS: one-sentence answer, zero tool calls. |
| References / root | `$run-mailven` Read-only documentation task: how to take headless README screenshots safely; no launches or changes. | PASS: command read canonical core, `references/screenshot.md` and `references/playwright.md`; answer retained fake isolation, both Accounts states, state restoration and task-owned cleanup. |
| Continuation / root | Use only tracked guidance, PLAN checkpoint and working-tree inspection; identify current milestone, next Codex-only action, dirty files, valid checks, blockers and landing constraints; no changes/checks/delegation. | PASS for Codex receiving repository-backed state: identified G3 and next routing inspection, unrelated dirty sentinel, retained earlier checks and no push/merge/completion. This is not bidirectional cross-host evidence. |

Before continuation, the harness added a checkpoint beneath fixture Progress
naming the next action, base revision, existing valid checks and no-push scope,
and an unrelated HTML-comment sentinel to fixture `CONTRIBUTING.md`. Both are
tracked dirty files, not private session memory. The receiving Codex used Git
inspection and the checkpoint without rerunning application checks. The fixture
setup was controlled by this session; no Claude-produced handoff was claimed.

Completed-turn usage (input / cached input / output): explicit
62,906 / 48,384 / 368; natural 40,430 / 14,336 / 184; non-trigger
19,808 / 7,168 / 28; references 44,961 / 20,736 / 711; continuation
105,206 / 75,904 / 785. These are full-turn counters (including repeated
requests, tool reads and user/global context), not initial context-window sizes.
They do not prove project token savings or comparable cross-host context.

The earlier follow-up did not persist native routing results or final fixture
integrity evidence; those results cannot be inferred from the statement that
they would be recorded below. Current-adapter routing is retried in the resume
section below.
Official [Codex subagent documentation](https://learn.chatgpt.com/docs/agent-configuration/subagents)
was re-opened; documented configuration support is distinct from runtime evidence.


## G3 resume — 2026-10-12

Base `138cce1`, branch `docs/shared-ai-policy`, clean starting tree. Progress now
reflects the recorded Codex-only passing trials instead of marking all pending.
Earlier UI/discovery/reference/continuation checks remain valid for their covered
revisions; no application or UI-tooling changes require repeating them.
Current adapters at this base use GPT-6 Luna Medium for acceptance/worker and
Luna Low for file_reader. Earlier Sol Low/Medium results above are historical.

AI7 correction: Start step 1 explicitly requires the outline helper even when a
plan appears absent and forbids substituting a missing-file preflight. A fresh
ordinary configured Codex 0.162.1 session in a symlink-preserving local clone
(`/tmp/mailven-g3-resume-cfe6s9xd/fixture`, origin removed) selected the skill
from natural language, read its canonical body, resolved the checkout, and ran:

```sh
codex exec --sandbox read-only --ephemeral --json 'Continue the next milestone using absent-natural-plan.md. This is a read-only disposable-fixture smoke test. Stop at the missing-plan blocker; do not create files, commit, push or delegate. Report the selected skill, exact command and result.'
sh "/tmp/mailven-g3-resume-cfe6s9xd/fixture/.agents/skills/project-next-milestone/plan-outline.sh" "/tmp/mailven-g3-resume-cfe6s9xd/fixture/absent-natural-plan.md"
python3 .agents/skills/project-next-milestone/tests/paths.py
```

PASS: helper exit 1, `No plan file at .../absent-natural-plan.md`; safe stop.
Path fixtures PASS: default/explicit/missing/spaced plans and
root/subdirectory/worktree resource identity. AI7 closed with observed behavior,
not wording alone. Full-turn usage: 82,872 input / 68,480 cached / 335 output;
these are not context-window or savings measurements. Clone contains only the
intended skill-body patch before trials. Raw prompts/JSONL remain in the temporary
directory; this section preserves the evidence without private session logs.

The official [Codex subagent documentation](https://learn.chatgpt.com/docs/agent-configuration/subagents)
was opened for current native-file model/effort support; installed runtime
application is checked separately. No configuration or model overrides added.
The stale signing issue was pruned: the configuration changes are already
committed at `138cce1` and SSH-agent identities are available. A new signed
checkpoint will be attempted without changing identity or bypassing signing.


### Native routing and host availability

The fresh clone trial used `codex exec --sandbox read-only --json PROMPT`
(non-ephemeral to inspect runtime evidence). Prompt: “Read-only native agent
routing smoke test in this disposable checkout. Sequentially delegate one trivial
task to the native project-acceptance-checker and one to the native project-worker.
Select their native agent_type without explicit model or effort overrides. Each
should read its own adapter and shared brief and report one required responsibility
without edits, tests, further delegation or workflow execution. Do not substitute
another role if unavailable; disclose it. Report the child IDs and observed
results. Do not commit or push.”

FAIL: both `spawn_agent` calls returned `unknown agent_type`; stderr and the
session's function-call outputs independently confirm this, with no children or
substitutions. Parent session `01a132e6-a35d-7240-a0c3-3fe36511ef91` used
GPT-6.1 Sol Medium. Both adapter files are present in the clone.

To distinguish clone portability from current-checkout routing, a fresh
read-only session started from the active checkout with the same command and:
“Read-only native-role availability diagnostic. Sequentially attempt to spawn
the native project-acceptance-checker and project-worker, one at a time, with no
model/effort override or substitute. Each child must only answer OK without tools
or further delegation. Do not invoke skills, edit files, run checks, commit or
push. Report exact spawn errors or IDs.”

PASS: both native roles spawned sequentially and replied OK. The parent rollout
records `agent_type`, `fork_turns: none` and no explicit model/effort arguments.
Child `session_meta.source.subagent.thread_spawn` ties each role to parent
`01a132e8-cb99-7d12-b133-23dbdb269f0a`; child `turn_context` confirms:

| Role | Child session ID | Applied model | Applied effort |
| --- | --- | --- | --- |
| project-acceptance-checker | `01a132e8-e298-7853-bc36-6bbbbe595b7a` | `gpt-6-luna` | `medium` |
| project-worker | `01a132e9-032c-7e01-b1e4-c88b1cd3b785` | `gpt-6-luna` | `medium` |

This proves current-checkout routing and inference, not full worker/acceptance
workflow compliance in the clone. AI8 records the clone rejection separately.
No personal configuration was changed. Read-only `config/read` via the installed
app-server returned the same agent defaults (Luna Medium) for both cwd values;
that output does not enumerate native roles or explain the discrepancy. Existing
trust entries include the active checkout and `/tmp`; no claim is made that the
clone had equivalent effective native-agent trust/loading. Diagnose the cause
before claiming portable routing or adding configuration workarounds.

Claude probe command, from clone root:

```sh
claude -p 'Read-only G3 availability probe. Reply OK without tools.' --permission-mode plan --no-session-persistence --output-format stream-json --verbose
```

Exit 1, HTTP 429 `usage_limit_reached`, `seven_day`; reported reset Oct 13,
9pm America/Vancouver. No model execution or context measurement occurred.
Claude selection/model override, both cross-host continuation directions and
comparable context diagnostics remain unavailable. Do not repeatedly submit
dependent trials while this limit is active.

Routing full-turn counters (input / cached / output): clone 81,593 / 47,232 / 410;
checkout 102,065 / 87,936 / 273. These are usage counters, not context savings.

### Available context diagnostics and integrity

`codex debug prompt-input 'G3 read-only context diagnostic.'` succeeded without
inference from both checkout and clone root. It exports the model-visible message
list, not a token count or the entire tool-schema/request envelope:

| cwd | Messages | Message-text bytes | Message-text words | JSON output bytes |
| --- | ---: | ---: | ---: | ---: |
| checkout | 5 | 28,023 | 3,830 | 30,739 |
| clone | 5 | 28,035 | 3,830 | 30,751 |

Counts sum `content[].text` UTF-8 bytes and whitespace-separated words. Both
exports contain the persisted Aerion identifier rule and all five skill names.
Differences reflect path text, not project-context savings. No equivalent Claude
measurement or pre-migration paired diagnostic exists; G3's comparison criterion
remains incomplete. Raw exports stay ephemeral because they contain global
instructions; no private/global configuration is copied into the repository.

Final clone inspection: `git diff --name-only` lists only
`.agents/skills/project-next-milestone/SKILL.md`; `git ls-files --others
--exclude-standard` is empty. No trial created a plan, changed app state, pushed
or advanced a real milestone. The app-server diagnostic stopped in `finally`;
no task-owned application server was started. Direct correctness and simplify
review found no further implementation change warranted. Application builds,
frontend checks and UI reruns are unnecessary for this guidance-only patch.


Independent read-only `project-acceptance-checker` review verified the raw natural
trial and final evidence consistency: AI7 correction PASS, UI PASS; full
both-host compliance, bidirectional continuation and context reduction remain
UNVERIFIABLE. No correctness gap or added stub found. `git diff --check 138cce1`
passed. Direct correctness review substituted for unavailable named code-review
tooling; simplify review found the instruction patch sufficiently simple.
The previous unpushed checkpoint is amended locally per repository policy;
G3 remains open and no push/merge is performed.


## Milestone split — 2026-10-12

At the user's request, unrun Claude compliance/selection/routing and model-override
checks, both cross-host continuation directions (AI5), and comparable context
analysis move to PLAN.md G4, “to do later.” Their verification status remains
incomplete. G3 retains passing Codex/UI evidence and the actionable AI8 clone
routing gap. Earlier references to G3's full cross-host acceptance describe the
original scope; the current milestone criteria are in PLAN.md. Resume G4 on
request when Claude inference is available, preserving checks still valid for
the covered revisions. This plan change does not enable paid access or authorize
branch landing. No runtime trials were repeated for this documentation edit.
Validation for the split: `git diff --check` and the canonical `plan-outline.sh`
helper PASS; direct review confirms G3 retains AI8, G4 has explicit deferred
acceptance/validation, and ISSUES.md points AI5 to G4. No runtime checks needed.


## AI8 resolution — trusted clone routing (2026-10-12)

Follow-up base `3d048b2`, branch `docs/shared-ai-policy`, clean starting tree;
Codex CLI 0.162.1. No application code, dependencies, native adapters or personal
configuration changed. Raw evidence lives at `/tmp/mailven-g3-ai8-7pc0kzd1`
(ephemeral); the following commands, results and child IDs are the durable record.

### Cause and prerequisite

Read-only app-server `config/read` with `includeLayers: true` and each checkout's
absolute `cwd` exposes the decisive difference. The active checkout's project
layer has `disabledReason: null`. The previous clone's layer reports:

> To load project-local config, hooks, and exec policies, add /tmp/mailven-g3-resume-cfe6s9xd/fixture as a trusted project in /home/user/.codex/config.toml.

The existing `/tmp` trust entry does not trust that nested Git checkout. This
explains why skill discovery passed but native `spawn_agent` rejected the roles.
The [official configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
says untrusted projects skip project-scoped `.codex/` layers; the
[official subagent reference](https://learn.chatgpt.com/docs/agent-configuration/subagents)
places project roles under `.codex/agents/`. Trust remains a host security
prerequisite, not a setting committed in this repository. CONTRIBUTING.md now
explains normal onboarding and the diagnostic; AGENTS.md adds a short pointer.

To obey the no-personal-configuration-edit scope, the new disposable clone used
an absent fixture path already explicitly trusted in the existing user config:
`/tmp/kc-context-pilot.oSxUrS/repository`. It was created with
`git clone --quiet --no-hardlinks /home/user/sync/git/projects/mailven /tmp/kc-context-pilot.oSxUrS/repository`.
No trust, model, effort, plugin or discovery flags were supplied. This proves
routing in a trusted clone; it does not claim arbitrary new clones are trusted
automatically or that onboarding was exercised. `config/read` shows this clone's
project layer enabled (`disabledReason: null`); all three native adapters are
byte-identical to the active checkout. `.claude/skills` is a preserved symlink.

### Native inference and integrity

From that clone root: `codex exec --sandbox read-only --json -`, with this stdin:

> Read-only native role routing smoke in this disposable clone. Sequentially spawn project-acceptance-checker and project-worker with their native agent_type, fork_turns none, no explicit model or effort overrides and no substitute. Each child must resolve the active checkout, read its own .codex/agents adapter and linked shared brief, and report one required responsibility. No edits, tests, builds, commits, pushes, further delegation or workflow execution. Wait for each child and report child IDs and results. Do not execute project-next-milestone or advance PLAN.

PASS, exit 0. Parent `01a13305-b08a-77e3-82b1-1c152b7a5374` actually spawned both
roles sequentially with `fork_turns: none` and no model/effort arguments. Both
children resolved the clone, read their adapter and canonical shared brief, and
reported the required read-only/scope responsibility. Child rollout metadata ties
them to that parent and `turn_context` records the applied settings:

| Native role | Child session ID | Applied model | Applied effort |
| --- | --- | --- | --- |
| project-acceptance-checker | `01a13305-d519-7d13-a2c8-5f1094dad915` | `gpt-6-luna` | `medium` |
| project-worker | `01a13306-2966-71f0-89c8-21cab55e5b3b` | `gpt-6-luna` | `medium` |

`git diff --exit-code` PASS; `git ls-files --others --exclude-standard` empty;
fixture HEAD remains `3d048b2aa3e383f5eae6fa8a3b08d7cf3b6d0cc3`. The user config
SHA-256 before/after is identical. App-server diagnostics terminated their child
processes; no application server was started. Fixtures are retained for review.
The smoke turn's input/cached/output counters were 103,434 / 61,568 / 571; these
are not context-savings evidence.

Prior discovery/selection/helper, repository-backed Codex continuation, context
diagnostics and guarded UI evidence remain revision-specific passing checks;
no covered helper, adapter or application implementation changed in this task.
Claude compliance/routing, both cross-host continuation directions and comparable
context/accuracy checks remain G4, deferred and unverified. AI8 is resolved for
trusted clones; G3 is complete after final review. Keep the feature branch unpushed
and unlanded under the recorded local-checkpoint-only decision.


### Final review and checkpoint

The native read-only `project-acceptance-checker` independently checked all five
G3 criteria, including raw config and child rollout evidence. Four initially
passed; it caught a stale PLAN checkpoint under the continuation criterion.
The checkpoint was updated with base/branch, five changed files, command results,
evidence provenance, current step, next action and G4/AI5/D2 blockers; the reviewer
rechecked it and passed the fifth criterion. No added stubs or unresolved G3
findings remain. Direct correctness review substituted for unavailable named
code-review tooling. Behavior-preserving simplification review found no further
change warranted; no implementation code changed in this follow-up.

Final changed files: `AGENTS.md`, `CONTRIBUTING.md`, `PLAN.md`, `ISSUES.md`,
`docs/validation/shared-skills-g3.md`. `git diff --check` PASS; canonical
`sh /home/user/sync/git/projects/mailven/.agents/skills/project-next-milestone/plan-outline.sh /home/user/sync/git/projects/mailven/PLAN.md`
PASS. Read-only `archive/` has no staged or unstaged changes. No application build,
frontend check or UI rerun was needed for this guidance/evidence-only follow-up;
prior application checks and their native-window limitations remain as recorded.

G3 Progress is collapsed into its completed line; AI8 is removed from ISSUES.md.
The existing unpushed G3 milestone commit is amended locally, including the
previous AI7 helper correction and retest. No push/merge or second milestone is
performed. Next: G4 only on request when Claude inference is available; AI5 and
cross-host/context acceptance remain incomplete, and D2 remains blocked.


## Branch closeout (2026-10-12)

The user requested closing `docs/shared-ai-policy` for now, deferring remaining
work, committing outstanding changes and merging/pushing main. This supersedes
all earlier local-checkpoint-only and hold-until-G4 landing decisions in this
historical record; those entries describe their original session scope.
G1–G3 are complete. G4 (Claude compliance/routing, bidirectional continuation and
comparable context evidence), AI5 and D2 remain explicitly deferred and incomplete.
PLAN.md now contains only future work and current decisions; CONTRIBUTING.md and
ISSUES.md reflect this status. No new cross-host acceptance is claimed.

Closeout changes affect documentation only. Previously recorded Codex, routing
and guarded UI evidence remains valid for its covered revisions; no application
or UI rerun is needed. Narrow helper regression, syntax, link/symlink and diff
checks and an independent read-only closeout review are recorded below before
landing. Direct review of the branch found no further behavior-preserving
simplification warranted.


Closeout results:

- `python3 .agents/skills/project-next-milestone/tests/paths.py` — PASS default,
  explicit, missing and spaced plans; root/subdirectory/worktree resource identity.
- `node .agents/skills/run-mailven/tests/launcher.js` — PASS capability selection,
  overrides and missing dependencies.
- `sh .agents/skills/project-next-milestone/plan-outline.sh PLAN.md` — PASS;
  only deferred G4 and D2 remain unchecked.
- Inline Python assertions — PASS canonical symlink, CLAUDE policy import,
  skill reference targets and native-agent TOML parsing.
- `node --check` all skill JavaScript, Python `compile()` all helper sources,
  `bash -n` dev.sh and `sh -n` plan-outline.sh — PASS.
- `git diff --check` and `git diff --check main` — PASS.
- Native `project-acceptance-checker` read-only review — PASS all three closeout
  criteria: consistent deferral, landing authorization without false acceptance,
  and pruning with preserved follow-up criteria/provenance. No material findings
  or added stubs. Primary direct review found no further simplification needed.

No app build/test or visual UI rerun was performed for this documentation-only
closeout. Deferred Claude/cross-host/context verification was not attempted.
