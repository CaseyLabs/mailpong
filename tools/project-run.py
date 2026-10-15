#!/usr/bin/env python3
"""Run roadmap milestones unattended: one fresh Claude Code or Codex session per milestone.

The runner picks the best eligible milestone from PLAN.md (or takes --milestones), authorizes
exactly that milestone's local scope, and starts a headless session that runs the
project-next-milestone skill, including its finish step and one local commit. It cross-checks
the commit and the plan status, continues a committed handoff in a fresh session, skips blocked
milestones, and waits out usage limits. Progress lives in PLAN.md and Git history, so an
interrupted run resumes by rerunning. It never reads credentials, pushes or lands.
"""
import argparse
import datetime as dt
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import time
from zoneinfo import ZoneInfo

sys.path.insert(0, str(Path(__file__).resolve().parent))
from project_common import WorkflowError, git, repository, resolve_path  # noqa: E402

SKILL = 'project-next-milestone'
SCHEMA = {
    'type': 'object',
    'properties': {
        'status': {'type': 'string', 'enum': ['success', 'handoff', 'blocked']},
        'milestone': {'type': 'string'},
        'summary': {'type': 'string'},
        'evidence': {'type': 'string'},
    },
    'required': ['status', 'milestone', 'summary', 'evidence'],
    'additionalProperties': False,
}
LIMIT_RE = re.compile(r'(?:usage|rate|session|weekly)\s+limit|limit\s+(?:reached|exceeded)|you(?:.ve| have)\s+hit\s+your\s+(?:limit|session)|too many requests|\b429\b|\blimit\b[^\n]{0,80}?\bresets?\b', re.I)
RESUME_PROMPT = ('The usage limit has reset. Continue the milestone from where you stopped: finish '
                 f'the {SKILL} skill, including its finish-milestone step and local commit, then return '
                 'the required structured status.')
STATUS_PROMPT = ('Your last reply had no valid structured status, possibly because a subagent or forked '
                 'skill returned it instead. Do not redo or extend any work. Check PLAN.md and git log, '
                 'then return the required structured status for this milestone yourself.')
CLAUDE_EFFORTS = ['low', 'medium', 'high', 'xhigh', 'max']
CODEX_EFFORTS = ['minimal', 'low', 'medium', 'high', 'xhigh']
UNIT_SECONDS = {'s': 1, 'm': 60, 'h': 3600}
# Pinned so unattended runs don't pick up a new release mid-plan.
PLAYWRIGHT_MCP = '@playwright/mcp@0.0.83'
# Distinct from any `playwright` server in the user's or project's MCP config, which would otherwise
# take precedence (Claude) or merge key by key with this one (Codex).
PLAYWRIGHT_NAME = 'plan_runner_playwright'
# Playwright MCP's default `chrome` channel looks only at these fixed install paths.
CHROME_PATHS = ('/opt/google/chrome/chrome', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome')
MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

# Roadmap headings are `## SEND — Title` (project) and `### SEND-1 — Title` (milestone).
HEADING_RE = re.compile(r'^(##|###) ([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*) — (.+)$')
STATUS_RE = re.compile(r'\*\*Status / proposed priority:\*\*\s*([a-z]+)(?:\s*/\s*P(\d))?')
DEPS_RE = re.compile(r'\*\*Dependencies / gates:\*\*\s*(.*)')
# Milestone references such as API-2, W3-M0, D2 and ranges such as UIX-1–5.
REF_RE = re.compile(r'\b([A-Z][A-Z0-9]*-[A-Z]*|[A-Z]+)(\d+)(?:\s*–\s*(\d+))?(?![\w-])')
RUNNABLE = ('active', 'proposed')
CLOSED_PROJECTS = ('blocked', 'deferred', 'done')


class Stop(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code


def now():
    return dt.datetime.now(dt.timezone.utc)


def log(msg):
    print(f'[{dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")}] {msg}', flush=True)


def references(text, known):
    found = set()
    for prefix, start, end in REF_RE.findall(text):
        last = int(end or start)
        if last - int(start) > 50:
            last = int(start)
        found.update(ident for n in range(int(start), last + 1) if (ident := f'{prefix}{n}') in known)
    return found


def parse_plan(text):
    """Return the roadmap's milestones in plan order, each with its project, status, priority and deps."""
    milestones, project, current = [], None, None
    for number, line in enumerate(text.splitlines(), 1):
        heading = HEADING_RE.match(line)
        if heading:
            level, ident, title = heading.groups()
            current = {'id': ident, 'title': title.strip(), 'line': number, 'status': None,
                       'priority': 9, 'deps': ''}
            if level == '##':
                project = current
            else:
                current['project'] = project
                milestones.append(current)
            continue
        if line.startswith(('# ', '## ', '### ')):
            current = None
            if not line.startswith('### '):
                project = None
            continue
        if current is None:
            continue
        status = STATUS_RE.search(line)
        if status and current['status'] is None:
            current['status'], current['priority'] = status.group(1), int(status.group(2) or 9)
        deps = DEPS_RE.search(line)
        if deps and not current['deps']:
            current['deps'] = deps.group(1)
    known = {m['id'] for m in milestones}
    for m in milestones:
        m['deps'] = references(m['deps'], known) - {m['id']}
    return milestones


def candidates(milestones, exclude=()):
    """Eligible milestones, best first: active ones resume first, then priority, then plan order.

    A candidate is active or proposed, outside a blocked/deferred/done project, and every milestone it
    names under Dependencies / gates is done. Deferred and blocked milestones are never eligible.
    """
    done = {m['id'] for m in milestones if m['status'] == 'done'}
    ready = [m for m in milestones
             if m['status'] in RUNNABLE and m['id'] not in exclude and m['deps'] <= done
             and (m['project'] or {}).get('status') not in CLOSED_PROJECTS]
    return sorted(ready, key=lambda m: (m['status'] != 'active', m['priority'], m['line']))


def extract_reset(text):
    """Parse ISO timestamps, clock times with an IANA zone, and relative wait intervals."""
    match = re.search(r'\b(?:reset(?:s|ting)?|try again|available)\s+(?:at|on|after)\s+(\d{4}-\d\d-\d\d[T ]\d\d:\d\d(?::\d\d)?(?:Z|[+-]\d\d:?\d\d))', text, re.I)
    if match:
        try:
            stamp = dt.datetime.fromisoformat(match.group(1).replace('Z', '+00:00'))
            if stamp.tzinfo is not None and stamp > now():
                return stamp
        except ValueError:
            pass
    # Claude reports a clock time and an IANA timezone, with a date for weekly limits, e.g.
    # 'resets 1pm (America/Vancouver)', 'resets 3:30pm (UTC)' or 'resets Oct 13, 9pm (America/Vancouver)'.
    match = re.search(r'\breset(?:s|ting)?\s+(?:at\s+)?(?:([A-Za-z]{3})[a-z]*\.?\s+(\d{1,2}),?\s+(?:at\s+)?)?'
                      r'(\d{1,2})(?::(\d{2}))?\s*(am|pm)\s*\(([A-Za-z_]+(?:/[A-Za-z0-9_+-]+)*)\)', text, re.I)
    if match:
        month_name, day, hour, minute, ampm, zone = match.groups()
        try:
            local_now = now().astimezone(ZoneInfo(zone))
            candidate = local_now.replace(hour=int(hour) % 12 + (12 if ampm.lower() == 'pm' else 0),
                                          minute=int(minute or 0), second=0, microsecond=0)
            if month_name:
                month = MONTHS.index(month_name.title()) + 1
                candidate = candidate.replace(month=month, day=int(day))
                if candidate <= local_now:
                    candidate = candidate.replace(year=candidate.year + 1)
            elif candidate <= local_now:
                candidate += dt.timedelta(days=1)
            return candidate.astimezone(dt.timezone.utc)
        except (ValueError, KeyError):
            pass
    # Codex reports local time without a zone: 'try again at 10:07 PM.' or 'try again at Oct 14th, 2026 10:07 PM.'
    match = re.search(r'\btry\s+again\s+at\s+(?:([A-Za-z]{3})[a-z]*\.?\s+(\d{1,2})(?:st|nd|rd|th)?,?\s+(\d{4})\s+)?'
                      r'(\d{1,2}):(\d{2})\s*([AP]M)\b', text, re.I)
    if match:
        month_name, day, year, hour, minute, ampm = match.groups()
        try:
            local_now = now().astimezone()
            candidate = local_now.replace(hour=int(hour) % 12 + (12 if ampm.lower() == 'pm' else 0),
                                          minute=int(minute), second=0, microsecond=0)
            if month_name:
                candidate = candidate.replace(year=int(year), month=MONTHS.index(month_name.title()) + 1, day=int(day))
            elif candidate <= local_now:
                candidate += dt.timedelta(days=1)
            if candidate > local_now:
                return candidate.astimezone(dt.timezone.utc)
        except ValueError:
            pass
    match = re.search(r'\b(?:resets?\s+in|try\s+again\s+in|retry\s+after)\s+((?:\d+\s*(?:hours?|hrs?|h|minutes?|mins?|m|seconds?|secs?|s)(?![a-z])(?:\s*,?\s*(?:and\s+)?))+)', text, re.I)
    if match:
        seconds = sum(int(n) * UNIT_SECONDS[u[0].lower()]
                      for n, u in re.findall(r'(\d+)\s*([a-z]+)', match.group(1), re.I))
        return now() + dt.timedelta(seconds=seconds)
    return None


def instructions(plan, ident, agent, playwright, continuing):
    # Appended to the system prompt (Claude) or developer instructions (Codex): the user prompt is
    # only the skill invocation, because everything after the skill name becomes its arguments.
    final = ('Only this top-level session calls StructuredOutput, once, as its final action; '
             if agent == 'claude' else 'Your final message is the structured response and nothing else; ')
    resume = (f'{ident} was started in an earlier session: continue it from its committed checkpoint; '
              'uncommitted changes in the checkout are its interrupted partial work. ' if continuing else '')
    return (f'Unattended run by tools/project-run.py. The owner started this runner on {dt.date.today()}, which '
            f'selects and authorizes exactly milestone {ident} of {plan} for local implementation; record that '
            'authorization in its checkpoint. It authorizes no other milestone, no deferred work, and no live '
            'provider access, test email, credential change, mailbox migration, dependency upgrade, push or '
            'landing: leave such gated steps unrun and recorded. ' + resume +
            f'Implement {ident} with the {SKILL} skill, including its finish-milestone step and local commit. '
            'There is no user to answer questions: decide within recorded decisions, or return status blocked '
            'instead of asking. Do not push, land, reset, rebase, switch branches or run git clean. '
            'Return status success only after the milestone is marked done and committed. '
            'Return status handoff when you hand off at a sub-step boundary under the session-boundary rules: '
            'commit the checkpoint with the milestone still active and skip the paste-ready prompt; the runner '
            'continues it in a fresh session. Return status blocked for an unmet gate, a failed validation or a '
            'material question; record the blocker under project policy and commit it, leaving no uncommitted '
            'changes. Return the required structured status, milestone ID, summary and concrete evidence. '
            + final +
            'when you start a subagent or forked skill, tell it to return its result as text instead. '
            + (f'Playwright MCP browser tools ({PLAYWRIGHT_NAME} server, headless) are available for checking web '
               'UIs; subagents without them should report browser checks for you to run. ' if playwright else ''))


def playwright_server(output_dir):
    return {'command': 'npx',
            'args': ['-y', PLAYWRIGHT_MCP, '--headless', '--isolated', '--output-dir', str(output_dir)]}


def codex_data(stdout):
    """Fold `codex exec --json` events into the fields this runner reads from Claude's JSON result."""
    data, messages, errors, completed = {}, [], [], False
    for line in stdout.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not isinstance(event, dict):
            continue
        kind, item, error = event.get('type'), event.get('item'), event.get('error')
        if kind == 'thread.started':
            data['session_id'] = event.get('thread_id')
        elif kind == 'item.completed' and isinstance(item, dict) and item.get('type') == 'agent_message':
            messages.append(item.get('text', ''))
        elif kind == 'turn.completed':
            completed = True
        elif kind == 'turn.failed':
            errors.append(str(error.get('message', '')) if isinstance(error, dict) else str(error))
        elif kind == 'error':
            errors.append(str(event.get('message', '')))
    if not data:
        return None
    # Codex emits non-fatal error events while reconnecting, so only a turn that never completed failed.
    return {**data, 'result': messages[-1] if messages else '', 'is_error': not completed, 'errors': errors}


def parse_result(data):
    if not isinstance(data, dict):
        return None
    # Some CLI versions place the schema response in result instead of structured_output.
    for obj in (data.get('structured_output'), data.get('result')):
        if isinstance(obj, str):
            try:
                obj = json.loads(obj)
            except json.JSONDecodeError:
                continue
        if (isinstance(obj, dict) and all(isinstance(obj.get(k), str) for k in SCHEMA['required'])
                and obj['status'] in SCHEMA['properties']['status']['enum']):
            return obj
    return None


def limit_text(proc, data):
    """Return the error text to scan for usage limits, or '' if the run did not fail.

    Only stderr and the CLI's result/errors fields are scanned: the full JSON holds token
    counts and timings (e.g. 31429) and model prose that could mention rate limits.
    """
    is_dict = isinstance(data, dict)
    if proc.returncode == 0 and not (is_dict and data.get('is_error')):
        return ''
    if not is_dict:
        return proc.stderr + '\n' + proc.stdout
    return '\n'.join((proc.stderr, str(data.get('result', '')), str(data.get('errors', ''))))


def pause_for_limit(raw, args, waiting_since):
    reset = extract_reset(raw)
    delay = max(60, args.retry_minutes * 60)
    if reset:
        delay = max(60, int((reset - now()).total_seconds()) + 30)
    if time.monotonic() - waiting_since + delay > args.max_wait_hours * 3600:
        when = f' (resets {reset.astimezone():%Y-%m-%d %H:%M %Z})' if reset else ''
        raise Stop(1, f'Usage limit persists beyond --max-wait-hours{when}; rerun later to resume')
    log(f'{args.agent.title()} usage limit detected. Retrying in {delay // 60}m {delay % 60}s; Ctrl+C safely stops.')
    time.sleep(delay)


def git_head(root):
    return git(root, 'rev-parse', 'HEAD').stdout.strip()


def git_dirty(root):
    return git(root, 'status', '--porcelain').stdout.strip()


def commands(args, run_dir, system_text, prompt):
    """Return the start command and a resume(session_id, text) builder for one milestone session."""
    server = playwright_server(run_dir / 'playwright') if args.playwright else None
    if args.agent == 'claude':
        cmd = ['claude', '-p', '--output-format', 'json', '--json-schema', json.dumps(SCHEMA),
               '--max-turns', str(args.max_turns), '--permission-mode', args.permission_mode]
        if args.model:
            cmd += ['--model', args.model]
        if args.effort:
            cmd += ['--effort', args.effort]
        cmd.append('--chrome' if args.chrome else '--no-chrome')
        if server:
            # The = form keeps the variadic --mcp-config from consuming the arguments after it.
            cmd.append('--mcp-config=' + json.dumps({'mcpServers': {PLAYWRIGHT_NAME: server}}))
        cmd += ['--append-system-prompt', system_text]
        return cmd + [f'/{prompt}'], lambda session_id, text: cmd + ['--resume', session_id, text]
    schema_file = run_dir / 'schema.json'
    schema_file.write_text(json.dumps(SCHEMA))
    # `codex exec resume` takes these options too, but not --sandbox, so it is set through config.
    # JSON strings are valid TOML basic strings.
    opts = ['--json', '--output-schema', str(schema_file), '-c', f'sandbox_mode="{args.sandbox}"',
            '-c', 'approval_policy="never"', '-c', 'developer_instructions=' + json.dumps(system_text)]
    if server:
        # The first npx run downloads the package, which can exceed Codex's 10s default startup timeout,
        # and a first navigation or a long wait can exceed its 60s default tool timeout.
        for key, value in {**server, 'startup_timeout_sec': 60, 'tool_timeout_sec': 180}.items():
            opts += ['-c', f'mcp_servers.{PLAYWRIGHT_NAME}.{key}=' + json.dumps(value)]
    if args.model:
        opts += ['--model', args.model]
    if args.effort:
        opts += ['-c', f'model_reasoning_effort="{args.effort}"']
    return (['codex', 'exec', *opts, f'${prompt}'],
            lambda session_id, text: ['codex', 'exec', 'resume', *opts, session_id, text])


def run_session(args, root, run_dir, plan_rel, ident, continuing):
    """Run one milestone session to a structured result, resuming it through usage limits."""
    def build(continuing):
        system_text = instructions(plan_rel, ident, args.agent, args.playwright, continuing)
        return commands(args, run_dir, system_text, f'{SKILL} {ident} {plan_rel}')
    start_cmd, resume_cmd = build(continuing)
    waiting_since = resume_id = None
    status_retry = False
    while True:
        if resume_id:
            log(f'Resuming session {resume_id} for {ident}')
            run_cmd = resume_cmd(resume_id, STATUS_PROMPT if status_retry else RESUME_PROMPT)
        else:
            log(f'Starting a {args.agent} session for {ident}')
            run_cmd = start_cmd
        output_file = run_dir / f'{dt.datetime.now().strftime("%Y%m%d-%H%M%S")}-{ident}.log'
        proc = subprocess.run(run_cmd, text=True, capture_output=True, cwd=root, stdin=subprocess.DEVNULL)
        output_file.write_text(f'EXIT: {proc.returncode}\nSTDOUT:\n{proc.stdout}\nSTDERR:\n{proc.stderr}\n')
        if args.agent == 'codex':
            data = codex_data(proc.stdout)
        else:
            try:
                data = json.loads(proc.stdout)
            except json.JSONDecodeError:
                data = None
        raw = limit_text(proc, data)
        if LIMIT_RE.search(raw):
            # Resume the interrupted session after the reset so its work and context carry over.
            resume_id = (data.get('session_id') if isinstance(data, dict) else None) or resume_id
            if waiting_since is None:
                waiting_since = time.monotonic()
            pause_for_limit(raw, args, waiting_since)
            continue
        waiting_since = None
        if resume_id and data is None:
            if status_retry:
                raise Stop(1, f'Could not resume session {resume_id} for its status; see {output_file}. Rerun to resume.')
            # e.g. 'No conversation found with session ID' when the session was not persisted.
            log(f'Could not resume session {resume_id}; see {output_file}. Starting {ident} fresh.')
            resume_id = None
            start_cmd, resume_cmd = build(True)
            continue
        resume_id = None
        if proc.returncode != 0:
            raise Stop(1, f'{args.agent.title()} returned exit {proc.returncode}; see {output_file}. Rerun to resume.')
        result = parse_result(data)
        if result is not None:
            return result, output_file
        # A subagent or forked skill can claim StructuredOutput and leave the parent without a status,
        # so ask the same session once for its own status before giving up.
        session_id = data.get('session_id') if isinstance(data, dict) else None
        if not session_id or status_retry:
            raise Stop(1, f'Missing valid structured response; see {output_file}. Rerun to resume.')
        log(f'Missing valid structured response; see {output_file}. Asking the session for its status.')
        resume_id, status_retry = session_id, True


def plan_status(plan, ident):
    return next((m['status'] for m in parse_plan(plan.read_text()) if m['id'] == ident), None)


def run_milestone(args, root, run_dir, plan, plan_rel, ident):
    """Drive one milestone through sessions and handoffs; return 'success' or 'blocked'."""
    continuing = plan_status(plan, ident) == 'active'
    for _ in range(args.max_handoffs + 1):
        head = git_head(root)
        result, output_file = run_session(args, root, run_dir, plan_rel, ident, continuing)
        log(f'{result["status"]} | {result["milestone"]} | {result["summary"]}')
        if result['milestone'].strip() != ident:
            raise Stop(1, f'Session reported milestone {result["milestone"]!r}, not {ident}; see {output_file}.')
        if git_dirty(root):
            raise Stop(2, f'{ident} {result["status"]} left uncommitted changes; see git status and {output_file}.')
        if result['status'] == 'blocked':
            log(f'Blocked: {result["evidence"]}')
            return 'blocked'
        if git_head(root) == head:
            raise Stop(1, f'{ident} reported {result["status"]} but made no commit; see {output_file}.')
        if result['status'] == 'success':
            if (status := plan_status(plan, ident)) != 'done':
                raise Stop(1, f'{ident} reported success but {plan_rel} lists it as {status}; see {output_file}.')
            return 'success'
        log(f'{ident} handed off at a sub-step boundary; continuing it in a fresh session.')
        continuing = True
    raise Stop(2, f'{ident} handed off more than --max-handoffs ({args.max_handoffs}) times; check its checkpoint.')


def next_milestone(plan, explicit, attempted):
    milestones = parse_plan(plan.read_text())
    if explicit is None:
        ranked = candidates(milestones, attempted)
        return ranked[0]['id'] if ranked else None
    status = {m['id']: m['status'] for m in milestones}
    return next((i for i in explicit if i not in attempted and status.get(i) in RUNNABLE), None)


def show_candidates(plan, plan_rel):
    ranked = candidates(parse_plan(plan.read_text()))
    if not ranked:
        print(f'No eligible milestone in {plan_rel}.')
    for n, m in enumerate(ranked, 1):
        print(f'{n}. {m["id"]} ({m["status"]}, P{m["priority"]}) — {m["title"]} [{plan_rel}:{m["line"]}]')


def parse_args(argv):
    p = argparse.ArgumentParser(description='Run eligible PLAN.md milestones unattended, one fresh session each')
    p.add_argument('--agent', choices=['claude', 'codex'], default='claude', help='CLI that runs each milestone session')
    p.add_argument('--plan', default='PLAN.md', help='Roadmap path, relative to the invocation directory')
    p.add_argument('--milestones', help='Comma-separated milestone IDs to run in this order instead of auto-selection')
    p.add_argument('--list', action='store_true', help='Print the ranked eligible milestones and exit')
    p.add_argument('--model', help='Model alias or ID; default uses the agent CLI settings')
    p.add_argument('--effort', choices=sorted(set(CLAUDE_EFFORTS + CODEX_EFFORTS), key=(CODEX_EFFORTS + ['max']).index),
                   help=f'reasoning effort for each session (Claude: {", ".join(CLAUDE_EFFORTS)}; '
                        f'Codex: {", ".join(CODEX_EFFORTS)}); default uses the agent CLI settings')
    p.add_argument('--chrome', action=argparse.BooleanOptionalAction, default=True,
                   help='Claude only: Claude in Chrome in each session, on by default '
                        '(needs Chrome open with the extension connected)')
    p.add_argument('--playwright', action=argparse.BooleanOptionalAction, default=True,
                   help=f'Playwright MCP server ({PLAYWRIGHT_MCP}, headless) in each session, on by default (needs npx)')
    p.add_argument('--max-milestones', type=int, default=25, help='Maximum milestones to complete in this run')
    p.add_argument('--max-handoffs', type=int, default=3, help='Fresh sessions allowed per milestone after a handoff')
    p.add_argument('--max-turns', type=int, default=200, help='Claude only')
    p.add_argument('--retry-minutes', type=int, default=15, help='Fallback cooldown when no parseable reset time is provided')
    p.add_argument('--max-wait-hours', type=float, default=24)
    p.add_argument('--permission-mode', choices=['acceptEdits', 'dontAsk', 'default', 'auto', 'bypassPermissions'], default='bypassPermissions',
                   help='Claude only. Default skips all permission checks; acceptEdits denies shell commands not in your allowlist')
    p.add_argument('--sandbox', choices=['read-only', 'workspace-write', 'danger-full-access'], default='danger-full-access',
                   help='Codex only, with approvals off. workspace-write keeps .git read-only, which blocks the milestone commit')
    p.add_argument('--verify-command', help='Optional independent shell verification after each completed milestone')
    args = p.parse_args(argv)
    if min(args.max_milestones, args.max_turns, args.retry_minutes) < 1 or args.max_handoffs < 0 or args.max_wait_hours <= 0:
        p.error('Milestone, turn, retry and maximum wait values must be positive')
    if args.effort and args.effort not in (CLAUDE_EFFORTS if args.agent == 'claude' else CODEX_EFFORTS):
        p.error(f'--effort {args.effort} is not supported by {args.agent}')
    return p, args


def main(argv=None):
    p, args = parse_args(argv)
    root = repository()
    plan = resolve_path(args.plan)
    if not plan.is_file() or not plan.is_relative_to(root):
        p.error('The plan file must exist within this checkout')
    plan_rel = str(plan.relative_to(root))
    if args.list:
        show_candidates(plan, plan_rel)
        return 0
    explicit = None
    if args.milestones:
        explicit = [i.strip() for i in args.milestones.split(',') if i.strip()]
        status = {m['id']: m['status'] for m in parse_plan(plan.read_text())}
        bad = [f'{i} ({status.get(i) or "not in the plan"})' for i in explicit if status.get(i) not in RUNNABLE]
        if bad:
            p.error('Only active or proposed milestones can run; reopen or fix these first: ' + ', '.join(bad))
    if not shutil.which(args.agent):
        p.error(f'{args.agent} CLI not found on PATH')
    if args.playwright and not shutil.which('npx'):
        p.error('npx not found on PATH; install Node.js or pass --no-playwright')
    if args.playwright and not any(Path(c).exists() for c in CHROME_PATHS):
        p.error('Google Chrome not found, which Playwright MCP uses; install it or pass --no-playwright')
    if not (root / '.agents/skills' / SKILL / 'SKILL.md').is_file():
        p.error(f'Missing project skill .agents/skills/{SKILL}/SKILL.md')
    branch = git(root, 'branch', '--show-current').stdout.strip()
    if branch in ('', 'main', 'master'):
        p.error('Run on a feature branch: sessions commit milestones locally and never land them')
    if git_dirty(root):
        p.error('The checkout has uncommitted changes; commit or stash them first')
    if os.name == 'nt':
        p.error('The runner needs a Unix host (fcntl)')
    import fcntl
    lock = Path(git(root, 'rev-parse', '--git-path', 'project-run.lock').stdout.strip())
    lock = (lock if lock.is_absolute() else root / lock).open('a')
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        p.error('Another runner is active in this checkout')
    # Logs and Playwright output live outside the checkout so they never count as uncommitted changes.
    cache = Path(os.environ.get('XDG_CACHE_HOME', str(Path.home() / '.cache'))) / 'mailven-project-runs'
    cache.mkdir(parents=True, exist_ok=True, mode=0o700)
    run_dir = Path(tempfile.mkdtemp(prefix=dt.datetime.now().strftime('%Y%m%d-%H%M%S-'), dir=cache))
    log(f'Plan: {plan_rel} | branch {branch} | {args.agent} | '
        f'{"milestones " + ", ".join(explicit) if explicit else "auto-selection"} | '
        f'up to {args.max_milestones} milestone{"s" * (args.max_milestones != 1)} | logs {run_dir}')
    completed, attempted, blocked = 0, set(), []
    try:
        while completed < args.max_milestones:
            ident = next_milestone(plan, explicit, attempted)
            if ident is None:
                log('No eligible milestone remains.')
                break
            attempted.add(ident)
            log(f'Selected {ident}')
            if run_milestone(args, root, run_dir, plan, plan_rel, ident) == 'blocked':
                blocked.append(ident)
                continue
            if args.verify_command:
                log(f'External verification: {args.verify_command}')
                if (check := subprocess.run(args.verify_command, shell=True, cwd=root)).returncode:
                    raise Stop(2, f'External verification failed ({check.returncode}) after {ident}.')
            completed += 1
        else:
            log(f'Maximum milestones for this run reached ({completed}). Rerun to continue.')
    except Stop as stop:
        log(stop.args[0])
        return stop.code
    log(f'{completed} milestone{"s" * (completed != 1)} completed'
        + (f'; blocked: {", ".join(blocked)}' if blocked else '') + '.')
    return 2 if blocked else 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        log('Interrupted. Rerun to resume from the plan file.')
        sys.exit(130)
    except (OSError, ValueError, WorkflowError) as exc:
        log(f'ERROR: {exc}')
        sys.exit(1)
