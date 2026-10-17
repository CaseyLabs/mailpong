"""Exercise the unattended milestone runner with fixture plans and a fake agent CLI only."""
import datetime as dt
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

TOOLS = Path(__file__).resolve().parents[1]


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


RUN = load('run_helper', TOOLS / 'project-run.py')

PLAN = """# Project roadmap

## Portfolio

| Project | Status |
| --- | --- |
| [ALPHA](#alpha) | proposed |

<a id="alpha"></a>

## ALPHA — Alpha project

- **Status / proposed priority:** proposed / P2.

### ALPHA-1 — Done base

- **Status / proposed priority:** done / P2. **Dependencies / gates:** none.

### ALPHA-2 — Depends on base

- **Status / proposed priority:** proposed / P3. **Dependencies / gates:** [ALPHA-1](#alpha-1).

### ALPHA-3 — Waits on ALPHA-4

- **Status / proposed priority:** proposed / P1. **Dependencies / gates:** ALPHA-1–2 and ALPHA-4.

### ALPHA-4 — Urgent

- **Status / proposed priority:** proposed / P1. **Dependencies / gates:** none.

### ALPHA-5 — Deferred

- **Status / proposed priority:** deferred / P1. **Dependencies / gates:** owner decision.

### ALPHA-6 — Gated

- **Status / proposed priority:** blocked / P1. **Dependencies / gates:** live authorization.

## BETA — Deferred project

- **Status / proposed priority:** deferred / P3.

### BETA-1 — Inside a deferred project

- **Status / proposed priority:** proposed / P1. **Dependencies / gates:** none.

## GAMMA — Resumed work

- **Status / proposed priority:** proposed / P3.

### GAMMA-1 — Already active

- **Status / proposed priority:** active / P3. **Dependencies / gates:** none.

## Progress

No milestone here.
"""

# Marks the selected milestone per FAKE_PLAN ({id: [status per session]}) and returns that status.
FAKE_CLAUDE = r'''#!/usr/bin/env python3
import json, os, re, subprocess, sys
from pathlib import Path
prompt = sys.argv[-1]
ident = prompt.split()[1]
counter = Path(os.environ['FAKE_STATE']) / ident
calls = int(counter.read_text()) if counter.exists() else 0
counter.write_text(str(calls + 1))
Path(os.environ['FAKE_STATE'], 'prompts').open('a').write(prompt + '\n')
outcomes = json.loads(os.environ['FAKE_PLAN']).get(ident, ['success'])
status = outcomes[min(calls, len(outcomes) - 1)]
plan = Path('PLAN.md')
if status in ('success', 'handoff', 'nodone'):
    new = 'done' if status == 'success' else 'active'
    text = re.sub(rf'(### {re.escape(ident)} — .*\n\n- \*\*Status / proposed priority:\*\* )[a-z]+',
                  rf'\g<1>{new}', plan.read_text())
    plan.write_text(text)
    subprocess.run(['git', 'commit', '--allow-empty', '-qam', f'feat: {ident} {status}'], check=True)
if status == 'dirty':
    plan.write_text(plan.read_text() + 'stray\n')
    status = 'blocked'
if status == 'nodone':
    status = 'success'
print(json.dumps({'session_id': 'fake', 'is_error': False, 'structured_output': {
    'status': status, 'milestone': ident, 'summary': f'{ident} {status}', 'evidence': 'fixture'}}))
'''


class Selection(unittest.TestCase):
    def setUp(self):
        self.milestones = RUN.parse_plan(PLAN)

    def ids(self, exclude=()):
        return [m['id'] for m in RUN.candidates(self.milestones, exclude)]

    def test_ranks_active_then_priority_then_order(self):
        self.assertEqual(self.ids(), ['GAMMA-1', 'ALPHA-4', 'ALPHA-2'])

    def test_dependencies_ranges_and_links(self):
        deps = {m['id']: m['deps'] for m in self.milestones}
        self.assertEqual(deps['ALPHA-2'], {'ALPHA-1'})
        self.assertEqual(deps['ALPHA-3'], {'ALPHA-1', 'ALPHA-2', 'ALPHA-4'})

    def test_exclusions_and_unlocking(self):
        self.assertEqual(self.ids({'GAMMA-1', 'ALPHA-4'}), ['ALPHA-2'])
        for m in self.milestones:
            if m['id'] in ('ALPHA-2', 'ALPHA-4'):
                m['status'] = 'done'
        self.assertEqual(self.ids({'GAMMA-1'}), ['ALPHA-3'])

    def test_never_selects_closed_work(self):
        never = {'ALPHA-1', 'ALPHA-5', 'ALPHA-6', 'BETA-1'}
        for m in self.milestones:
            if m['status'] == 'proposed' and m['id'] not in never:
                m['status'] = 'done'
        self.assertEqual(self.ids({'GAMMA-1'}), [])


class Parsing(unittest.TestCase):
    def test_reset_formats(self):
        future = (RUN.now() + dt.timedelta(hours=2)).replace(microsecond=0)
        self.assertEqual(RUN.extract_reset(f'resets at {future.isoformat()}'), future)
        zoned = RUN.extract_reset('resets 3:30pm (UTC)')
        self.assertEqual((zoned.hour, zoned.minute), (15, 30))
        self.assertGreater(zoned, RUN.now())
        relative = RUN.extract_reset('try again in 2h 30m') - RUN.now()
        self.assertAlmostEqual(relative.total_seconds(), 9000, delta=5)
        self.assertIsNone(RUN.extract_reset('no reset here'))

    def test_limit_text_ignores_successful_runs(self):
        ok = subprocess.CompletedProcess([], 0, '{"usage": 31429}', '')
        self.assertEqual(RUN.limit_text(ok, {'is_error': False}), '')
        failed = subprocess.CompletedProcess([], 1, '', "You've hit your session limit · resets 3pm (UTC)")
        self.assertRegex(RUN.limit_text(failed, None), RUN.LIMIT_RE)

    def test_results(self):
        body = {'status': 'handoff', 'milestone': 'A-1', 'summary': 's', 'evidence': 'e'}
        self.assertEqual(RUN.parse_result({'structured_output': body}), body)
        self.assertEqual(RUN.parse_result({'result': json.dumps(body)}), body)
        self.assertIsNone(RUN.parse_result({'result': json.dumps({**body, 'status': 'all_done'})}))
        events = '\n'.join(json.dumps(e) for e in (
            {'type': 'thread.started', 'thread_id': 't1'},
            {'type': 'error', 'message': 'reconnecting'},
            {'type': 'item.completed', 'item': {'type': 'agent_message', 'text': json.dumps(body)}},
            {'type': 'turn.completed'}))
        data = RUN.codex_data(events)
        self.assertEqual((data['session_id'], data['is_error']), ('t1', False))
        self.assertEqual(RUN.parse_result(data), body)

# Stand-ins for the checkout's check and landing helpers, used by --land.
FAKE_CHECK = r'''#!/usr/bin/env python3
import os, sys
from pathlib import Path
out = Path(sys.argv[sys.argv.index('--output-dir') + 1])
out.mkdir(parents=True)
(out / 'evidence.json').write_text('{}')
sys.exit(1 if os.environ.get('FAKE_CHECK_FAIL') else 0)
'''
FAKE_LAND = r'''#!/bin/sh
set -eu
echo "$1 $5" >> "$FAKE_STATE/landed"
git switch -q main
git merge -q --squash "$1"
git commit -qm "$5"
git branch -qD "$1"
'''


class Runner(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix='mailven run ')
        self.addCleanup(tmp.cleanup)
        self.tmp = Path(tmp.name)
        self.root = self.tmp / 'checkout with spaces'
        skill = self.root / '.agents/skills/project-next-milestone'
        skill.mkdir(parents=True)
        (skill / 'SKILL.md').write_text('fixture\n')
        (self.root / 'PLAN.md').write_text(PLAN)
        for args in (('init', '-q', '-b', 'main'), ('config', 'user.name', 'Fixture'),
                     ('config', 'user.email', 'fixture@example.invalid'),
                     ('config', 'core.hooksPath', str(self.root / '.git/hooks')),
                     ('add', '.'), ('commit', '-qm', 'docs: initial'), ('switch', '-qc', 'feature')):
            self.git(*args)
        bin_dir = self.tmp / 'bin'
        bin_dir.mkdir()
        (bin_dir / 'claude').write_text(FAKE_CLAUDE)
        (bin_dir / 'claude').chmod(0o755)
        self.state = self.tmp / 'state'
        self.state.mkdir()
        self.env = {**os.environ, 'PATH': f'{bin_dir}{os.pathsep}{os.environ["PATH"]}',
                    'XDG_CACHE_HOME': str(self.tmp / 'cache'), 'FAKE_STATE': str(self.state)}

    def git(self, *args):
        return subprocess.run(['git', '-C', str(self.root), *args], check=True,
                              capture_output=True, text=True).stdout

    def run_tool(self, *args, plan=None):
        env = {**self.env, 'FAKE_PLAN': json.dumps(plan or {})}
        return subprocess.run([sys.executable, str(TOOLS / 'project-run.py'), '--no-playwright', '--no-chrome', *args],
                              cwd=self.root, env=env, capture_output=True, text=True, timeout=60)

    def prompts(self):
        path = self.state / 'prompts'
        return [line.split()[1] for line in path.read_text().splitlines()] if path.exists() else []

    def test_runs_ranked_milestones_and_unlocks_dependents(self):
        proc = self.run_tool()
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.assertEqual(self.prompts(), ['GAMMA-1', 'ALPHA-4', 'ALPHA-2', 'ALPHA-3'])
        self.assertIn('4 milestones completed', proc.stdout)
        self.assertEqual(self.git('status', '--porcelain'), '')

    def test_blocked_is_skipped_and_reported(self):
        proc = self.run_tool('--max-milestones', '2', plan={'GAMMA-1': ['blocked']})
        self.assertEqual(proc.returncode, 2, proc.stdout + proc.stderr)
        self.assertEqual(self.prompts(), ['GAMMA-1', 'ALPHA-4', 'ALPHA-2'])
        self.assertIn('blocked: GAMMA-1', proc.stdout)

    def test_handoff_continues_in_fresh_session(self):
        proc = self.run_tool('--milestones', 'ALPHA-4', plan={'ALPHA-4': ['handoff', 'success']})
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.assertEqual(self.prompts(), ['ALPHA-4', 'ALPHA-4'])
        proc = self.run_tool('--milestones', 'ALPHA-2', '--max-handoffs', '1', plan={'ALPHA-2': ['handoff']})
        self.assertEqual(proc.returncode, 2)
        self.assertIn('more than --max-handoffs', proc.stdout)

    def test_inconsistent_sessions_stop(self):
        proc = self.run_tool('--milestones', 'ALPHA-4', plan={'ALPHA-4': ['nodone']})
        self.assertEqual(proc.returncode, 1)
        self.assertIn('lists it as active', proc.stdout)
        proc = self.run_tool('--milestones', 'ALPHA-2', plan={'ALPHA-2': ['dirty']})
        self.assertEqual(proc.returncode, 2)
        self.assertIn('left uncommitted changes', proc.stdout)

    def land_fixture(self):
        tools = self.root / 'tools'
        tools.mkdir()
        (tools / 'project-check.py').write_text(FAKE_CHECK)
        (tools / 'project-land.sh').write_text(FAKE_LAND)
        (tools / 'project-land.sh').chmod(0o755)
        self.git('switch', '-q', 'main')
        self.git('add', 'tools')
        self.git('commit', '-qm', 'test: landing fixtures')

    def test_land_lands_each_milestone_from_main(self):
        self.land_fixture()
        proc = self.run_tool('--land', '--milestones', 'ALPHA-4,ALPHA-2')
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.assertEqual((self.state / 'landed').read_text().splitlines(),
                         ['milestone/alpha-4 feat: ALPHA-4 success', 'milestone/alpha-2 feat: ALPHA-2 success'])
        self.assertEqual(self.git('branch', '--show-current').strip(), 'main')
        self.assertEqual(self.git('branch', '--list', 'milestone/*'), '')
        self.assertEqual(self.git('log', '-2', '--format=%s').splitlines(),
                         ['feat: ALPHA-2 success', 'feat: ALPHA-4 success'])

    def test_land_stops_without_landing_on_failed_checks_or_blocked(self):
        self.land_fixture()
        proc = self.run_tool('--land', '--milestones', 'ALPHA-4', plan={})
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.env['FAKE_CHECK_FAIL'] = '1'
        proc = self.run_tool('--land', '--milestones', 'ALPHA-2')
        self.assertEqual(proc.returncode, 2, proc.stdout + proc.stderr)
        self.assertIn('Checks failed before landing ALPHA-2', proc.stdout)
        self.assertEqual(self.git('branch', '--show-current').strip(), 'milestone/alpha-2')
        self.assertEqual(len((self.state / 'landed').read_text().splitlines()), 1)
        self.git('switch', '-q', 'main')
        proc = self.run_tool('--land', '--milestones', 'GAMMA-1', plan={'GAMMA-1': ['blocked']})
        self.assertEqual(proc.returncode, 2, proc.stdout + proc.stderr)
        self.assertIn('GAMMA-1 is blocked', proc.stdout)
        self.assertEqual(self.git('branch', '--show-current').strip(), 'milestone/gamma-1')

    def test_preconditions(self):
        proc = self.run_tool('--milestones', 'ALPHA-5,BETA-9')
        self.assertEqual(proc.returncode, 2)
        self.assertIn('ALPHA-5 (deferred), BETA-9 (not in the plan)', proc.stderr)
        (self.root / 'stray.txt').write_text('x\n')
        self.assertIn('uncommitted changes', self.run_tool().stderr)
        (self.root / 'stray.txt').unlink()
        self.git('switch', '-q', 'main')
        self.assertIn('feature branch', self.run_tool().stderr)
        listing = self.run_tool('--list')
        self.assertEqual(listing.returncode, 0)
        self.assertEqual(listing.stdout.splitlines()[0], '1. GAMMA-1 (active, P3) — Already active [PLAN.md:51]')
        self.assertEqual(self.prompts(), [])


if __name__ == '__main__':
    unittest.main()
