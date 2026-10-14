# AI5 handoff verification — 2026-10-14

Scope: close the bidirectional continuation gap in AI5. Source revision:
`6bbea2f`, branch `docs/ai-guidance-fixes`. No application code, dependencies,
personal configuration or shared skill instructions changed. The existing
checkpoint-before-commit guidance passed the trials below.

## Method

Installed hosts: Codex CLI 0.162.1 and Claude Code 2.1.296. Tests used independent,
symlink-preserving `git clone --quiet --no-hardlinks` clones under
`/tmp/mailven-ai5-bld5njix/`; each clone's origin was removed before invocation.
Normal personal/project discovery remained active. No discovery overrides or
model overrides were supplied. Claude reported `claude-opus-5-5`; these tests do
not establish native worker/reviewer routing or next-milestone model selection.

Each fixture seeded a committed PLAN.md and three files:

- `completed.txt`: initially `pending\n`, changed to verified `ready\n` before handoff.
- `result.txt`: initially `pending\n`, changed to unverified `candidate\n` before handoff.
- `verify.py`: asserts completed.txt is exactly `ready\n` and result.txt is exactly
  `final\n`, then prints `PASS local fixture`.

A new, unrelated `user-notes.txt` was staged before invocation. Its working bytes,
index entry and staged blob had to survive both sessions. The plan authorized
producers to commit only completed.txt and the checkpoint, preserving result.txt
uncommitted. It recorded the completed-file check (exit 0), `python3 verify.py`
(exit 1, `AssertionError: result not final`), and an unrun synthetic external gate:
recipient identity missing. No external contact was authorized or attempted.

Producer requests selected project-handoff with PLAN.md and required the fixture's
scope limits. They did not supply a handoff template. Each recipient was a fresh
host process receiving only the producer's single fenced prompt, plus an
instruction to execute its next action, update the checkpoint and stop without
committing or delegating. No producer transcript or session resume was supplied.

Representative native invocations (PROMPT is passed as one argument):

```sh
codex exec --sandbox workspace-write --ephemeral --json -o FINAL PROMPT
claude -p PROMPT --permission-mode acceptEdits \
  --allowedTools 'Bash(git *)' 'Bash(python3 *)' 'Bash(sh *)' \
  --no-session-persistence --output-format stream-json --verbose
```

The additional committed Codex producer used `--sandbox danger-full-access` in
its disposable clone, matching this task's writable runtime. No personal settings
or permission rules were edited. All trial invocations were bounded by `timeout 240`.
The initial Claude availability probe used plan mode and no tools; it passed.
The CLI invocation follows [official non-interactive guidance](https://learn.chatgpt.com/docs/non-interactive-mode), checked on 2026-10-14.

## Results

| Direction / case | Producer checkpoint | Fresh recipient | Preservation |
| --- | --- | --- | --- |
| Claude → Codex, committed checkpoint plus dirty work | PASS: one commit `9ed1a09`, only PLAN.md and completed.txt; checkpoint records pre-commit base `76f758e` | PASS: result.txt became `final\n`; verifier exited 0; checkpoint updated; no recipient commit | PASS: unrelated notes retained identical working bytes, index entry and staged blob; verified completed.txt unchanged |
| Codex → Claude, entirely uncommitted checkpoint | PASS with explicit runtime blocker: commit exited 128 because `.git/index.lock` could not be created; HEAD stayed `76f758e`; checkpoint/prompt reported this rather than claiming a commit | PASS: result.txt became `final\n`; verifier exited 0; checkpoint updated; producer commit remained outstanding | PASS: all protected bytes/index state retained; HEAD unchanged |
| Codex → Claude, committed checkpoint plus dirty work | PASS: one commit `ec1d2ea`, only PLAN.md and completed.txt; checkpoint records pre-commit base `0da1d8a` | PASS: result.txt became `final\n`; verifier exited 0; checkpoint updated; no recipient commit | PASS: protected file bytes, staged blob/index entry and HEAD retained |

In all three cases, receiving sessions explicitly retained the missing
recipient blocker, distinguished prior failing validation from current success,
and marked application checks, independent fixture review and the external check
as unrun. No recipient claimed that a local pass cleared the external gate. This tests
blocker transfer with synthetic state; it is not provider verification.

Primary inspections compared Git history and commit file lists,
pre-commit base/branch in the persisted checkpoint, HEAD before/after receiving,
exact dirty-file sets, file hashes and staged state. The verified completed-file
hash stayed unchanged. Notes SHA-256 values:

- Claude → Codex: `c62633a3963a1aef264281abffbbe87e29d438299f03ed8463693d9e632818fb`.
- Codex → Claude, uncommitted: `174521efb86ebea7c6ee8bd0368c3b15c873876648ab238be3f1a05d84508d3e`.
- Codex → Claude, committed: `515282d32c518774d2e16163953874b267f8551524ca6b9b80ddc4851263f029`.

Native session identifiers:

- Claude producer: `bbd8ddef-c1b5-4cc7-b5a2-e463512e883e`; Codex recipient:
  `01a13b09-c989-7ad3-89e7-04229a7febc4`.
- Codex uncommitted producer: `01a13b08-e37e-77f0-8480-0e8a3061cd65`;
  Claude recipient: `5bdb59d0-f3e3-4dab-b0a7-7cca21584b7e`.
- Codex committed producer: `01a13b0a-828f-7142-b3a1-3004b123404c`;
  Claude recipient: `83e2c3ba-e254-4bbe-88e8-17115d1e6e0f`.

Raw prompts, native JSONL, final responses, baseline hashes and inspection JSON
are temporary artifacts in the directory above. This record retains the scenario,
commands, revisions and observed outcomes; temporary artifact availability is not
promised after this session.

## Limits and closeout

AI5 is closed for these synthetic continuation scenarios. Native independent
`project-acceptance-checker` review passed the bidirectional evidence, DOC3
correction and bounded claims; the primary also inspected all three final
fixture states. No shared-skill correction was needed. DOC3 now points its
execution contract to the existing shared AGENTS.md guidance. Resolved issue
entries and the completed AI5 plan item were removed.

Narrow checks: `python3 .agents/skills/project-next-milestone/tests/paths.py`
passed; `sh .agents/skills/project-next-milestone/plan-outline.sh PLAN.md` passed
with no actionable milestone; guidance links/symlink and `git diff --check`
passed. Direct diff/simplification review found no code to simplify. Application
builds/tests and visual UI checks were not run for this documentation-only change.

Broader G4 Claude compliance/routing and comparable context savings remain
unverified and deferred. Handoff success does not establish those claims.
