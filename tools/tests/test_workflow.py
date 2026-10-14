"""Exercise logged checks and guarded landing in disposable repositories only."""
import importlib.util
import contextlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

TOOLS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(TOOLS))
from project_common import checkout_lock, tree_fingerprint


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


CHECK = load('check_helper', TOOLS / 'project-check.py')
LAND = load('land_helper', TOOLS / 'project_land.py')


class Workflow(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='mailven workflow ')
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'checkout with spaces'
        self.root.mkdir()
        self.remote = Path(self.tmp.name) / 'remote.git'
        self.git('init', '-q', '-b', 'main')
        self.git('config', 'user.name', 'Fixture')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.git('config', 'core.hooksPath', str(self.root / '.git/hooks'))
        (self.root / 'README.md').write_text('# Fixture\nBase\n')
        (self.root / 'tools').mkdir()
        for filename in ('check-doc-links.py', 'project_markdown.py'):
            shutil.copy2(TOOLS / filename, self.root / 'tools' / filename)
        self.git('add', '.')
        self.git('commit', '-qm', 'docs: initial')
        subprocess.run(['git', 'init', '--bare', '-q', str(self.remote)], check=True)
        self.git('remote', 'add', 'origin', str(self.remote))
        self.git('push', '-qu', 'origin', 'main')
        self.git('switch', '-qc', 'feature')
        (self.root / 'change.md').write_text('[Readme](README.md#fixture)\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'docs: feature')
        self.body = Path(self.tmp.name) / 'body with spaces.txt'
        self.body.write_text('Add documentation with verified checks.\n')
        self.counter = 0

    def git(self, *args, ok=True):
        r = subprocess.run(['git', '-C', str(self.root), *args], text=True, capture_output=True)
        if ok:
            self.assertEqual(r.returncode, 0, r.stderr)
        return r

    def cli(self, filename, *args):
        return subprocess.run([sys.executable, str(TOOLS / filename), *map(str, args)],
                              cwd=self.root, text=True, capture_output=True,
                              env={**os.environ, 'PYTHONDONTWRITEBYTECODE': '1'})

    def evidence(self, extra=None):
        self.counter += 1
        out = Path(self.tmp.name) / f'checks-{self.counter}'
        args = ['--base', 'main', '--run', '--output-dir', out]
        if extra:
            args += ['--extra-command', json.dumps(extra)]
        r = self.cli('project-check.py', *args)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        return out / 'evidence.json'

    def landing(self, evidence=None, execute=True):
        args = ['feature', '--title', 'docs: landed', '--body-file', self.body]
        if evidence:
            args += ['--evidence', evidence]
        if execute:
            args += ['--execute']
        return self.cli('project_land.py', *args)

    def advance_main(self, filename='other.md', text='# Other\n'):
        self.git('switch', '-q', 'main')
        (self.root / filename).write_text(text)
        self.git('add', '.')
        self.git('commit', '-qm', 'docs: advance main')
        self.git('push', '-q')
        self.git('switch', '-q', 'feature')

    def test_plan_does_not_execute_and_preserves_frontend_checks(self):
        frontend = self.root / 'frontend'
        frontend.mkdir()
        (frontend / 'component.svelte').write_text('<p>Fixture</p>')
        r = self.cli('project-check.py', '--base', 'main', '--json')
        self.assertEqual(r.returncode, 0, r.stderr)
        plan = json.loads(r.stdout)
        names = [c['name'] for c in plan['commands']]
        self.assertEqual(names, ['whitespace', 'docs', 'frontend-lint', 'frontend-check', 'frontend-build', 'ui'])
        self.assertFalse(plan['execute'])
        self.assertFalse((frontend / 'dist').exists())

    def test_scopes_are_union_not_permission_to_skip_checks(self):
        scopes, unknown = CHECK.infer_scopes(['frontend/view.svelte', 'internal/db/store.go', 'docs/guide.md', 'tools/test.py'])
        self.assertEqual(scopes, set(CHECK.SCOPES))
        self.assertEqual(unknown, [])
        self.assertEqual(CHECK.infer_scopes(['image.bin'])[1], ['image.bin'])
        self.assertEqual(CHECK.infer_scopes(['.agents/skills/test/SKILL.md'])[0], {'docs', 'tooling'})
        commands = CHECK.command_plan(self.root, {'go'}, 'HEAD')
        self.assertEqual([c['name'] for c in commands], ['whitespace', 'frontend-build', 'go'])

    def test_failed_checks_stop_and_retain_full_logs_and_nonzero_status(self):
        out = Path(self.tmp.name) / 'failed checks'
        commands = [
            {'name': 'fail', 'argv': [sys.executable, '-c', 'print("fixture failure"); raise SystemExit(7)'], 'cwd': '.'},
            {'name': 'unrun', 'argv': [sys.executable, '-c', 'raise SystemExit(0)'], 'cwd': '.'},
        ]
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code, path = CHECK.run_checks(self.root, commands, out, {'tree_fingerprint': tree_fingerprint(self.root)}, tail_lines=1)
        self.assertIn('STOP: remaining checks unrun', output.getvalue())
        self.assertEqual(code, 7)
        result = json.loads(path.read_text())
        self.assertFalse(result['passed'])
        self.assertEqual(len(result['results']), 1)
        self.assertIn('fixture failure', Path(result['results'][0]['log']).read_text())

    def test_fingerprint_covers_untracked_source_and_file_mode(self):
        old = tree_fingerprint(self.root)
        p = self.root / 'extra.py'
        p.write_text('print(1)\n')
        new = tree_fingerprint(self.root)
        self.assertNotEqual(old, new)
        p.chmod(0o755)
        self.assertNotEqual(new, tree_fingerprint(self.root))

    def test_deleted_source_evidence_remains_fresh_after_commit(self):
        path = self.root / 'obsolete.md'
        path.write_text('# Obsolete\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'docs: temporary source')
        path.unlink()
        before = tree_fingerprint(self.root)
        self.git('add', '-u')
        self.git('commit', '-qm', 'docs: delete temporary source')
        self.assertEqual(tree_fingerprint(self.root), before)

    def test_operation_lock_blocks_overlapping_checks(self):
        with checkout_lock(self.root):
            r = self.cli('project-check.py', '--run', '--output-dir', Path(self.tmp.name) / 'locked')
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('Another helper', r.stderr)

    def test_preview_is_read_only_and_execute_requires_evidence(self):
        head = self.git('rev-parse', 'HEAD').stdout
        remote = self.git('rev-parse', 'origin/main').stdout
        r = self.landing(execute=False)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn('PREVIEW ONLY', r.stdout)
        self.assertEqual(self.git('rev-parse', 'HEAD').stdout, head)
        self.assertEqual(self.git('rev-parse', 'origin/main').stdout, remote)
        self.assertNotEqual(self.landing().returncode, 0)

    def test_dirty_stale_failed_or_partial_evidence_is_refused(self):
        path = self.evidence()
        original = path.read_text()
        (self.root / 'dirty.md').write_text('# Dirty\n')
        self.assertNotEqual(self.landing(path).returncode, 0)
        (self.root / 'dirty.md').unlink()
        for change in ('stale', 'failed', 'partial', 'missing-log'):
            evidence = json.loads(original)
            if change == 'stale':
                evidence['tree_fingerprint'] = '0' * 64
            elif change == 'failed':
                evidence['passed'] = False
            elif change == 'partial':
                evidence['commands'].pop()
                evidence['results'].pop()
            else:
                evidence['results'][0]['log'] = str(Path(self.tmp.name) / 'missing.log')
            path.write_text(json.dumps(evidence))
            r = self.landing(path)
            self.assertNotEqual(r.returncode, 0, change)
            self.assertEqual(self.git('branch', '--show-current').stdout.strip(), 'feature')
        path.write_text(original)

    def test_success_uses_configured_identity_pushes_and_retains_recovery(self):
        r = self.landing(self.evidence())
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(self.git('branch', '--show-current').stdout.strip(), 'main')
        self.assertNotEqual(self.git('show-ref', '--verify', 'refs/heads/feature', ok=False).returncode, 0)
        self.assertEqual(self.git('log', '-1', '--format=%an <%ae>').stdout.strip(), 'Fixture <fixture@example.invalid>')
        self.assertEqual(self.git('rev-parse', 'main').stdout, self.git('rev-parse', 'origin/main').stdout)
        self.assertIn('recovery/project-land-', self.git('branch', '--list', 'recovery/*').stdout)
        self.assertFalse(self.git('status', '--porcelain').stdout)

    def test_rebase_reruns_recorded_checks_before_landing(self):
        marker = Path(self.tmp.name) / 'check-count.txt'
        extra = [sys.executable, '-c', 'from pathlib import Path; p=Path(__import__("sys").argv[1]); p.write_text(p.read_text()+"check\\n" if p.exists() else "check\\n")', str(marker)]
        evidence = self.evidence(extra)
        self.advance_main()
        r = self.landing(evidence)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(marker.read_text().splitlines(), ['check', 'check'])
        self.assertTrue((self.root / 'other.md').exists())

    def test_rebased_check_failure_stops_before_squash(self):
        extra=[sys.executable, '-c', 'from pathlib import Path; raise SystemExit(3 if Path("other.md").exists() else 0)']
        evidence=self.evidence(extra)
        self.advance_main()
        main_before=self.git('rev-parse','main').stdout
        result=self.landing(evidence)
        self.assertNotEqual(result.returncode,0)
        self.assertIn('Rebased checks failed',result.stderr)
        self.assertEqual(self.git('rev-parse','main').stdout,main_before)
        self.assertEqual(self.git('branch','--show-current').stdout.strip(),'feature')
        self.assertFalse(self.git('status','--porcelain').stdout)

    def test_upstream_is_required_and_main_worktree_is_respected(self):
        self.git('config','--unset','branch.main.remote')
        result=self.landing(execute=False)
        self.assertNotEqual(result.returncode,0)
        self.assertIn('configured upstream',result.stderr)
        self.git('config','branch.main.remote','origin')
        other=Path(self.tmp.name)/'main worktree'
        self.git('worktree','add','-q',str(other),'main')
        result=self.landing(execute=False)
        self.assertNotEqual(result.returncode,0)
        self.assertIn('another worktree',result.stderr)

    def test_conflict_aborts_rebase_without_landing(self):
        (self.root / 'README.md').write_text('# Fixture\nFeature edit\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'docs: conflict fixture')
        evidence = self.evidence()
        old = self.git('rev-parse', 'HEAD').stdout
        self.advance_main('README.md', '# Fixture\nMain edit\n')
        r = self.landing(evidence)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('Rebase failed and was aborted', r.stderr)
        self.assertIn('README.md', r.stderr)
        self.assertEqual(self.git('rev-parse', 'HEAD').stdout, old)
        self.assertFalse(self.git('status', '--porcelain').stdout)

    def test_commit_hook_rejection_does_not_push_or_delete_feature(self):
        evidence = self.evidence()
        hook = self.root / '.git/hooks/commit-msg'
        hook.write_text('#!/bin/sh\nexit 1\n')
        hook.chmod(0o755)
        old = self.git('rev-parse', 'main').stdout
        r = self.landing(evidence)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('Squash commit failed', r.stderr)
        self.assertEqual(self.git('rev-parse', 'main').stdout, old)
        self.git('show-ref', '--verify', 'refs/heads/feature')

    def test_push_rejection_retains_local_main_feature_and_recovery(self):
        evidence = self.evidence()
        hook = self.remote / 'hooks/pre-receive'
        hook.write_text('#!/bin/sh\nexit 1\n')
        hook.chmod(0o755)
        old = self.git('rev-parse', 'origin/main').stdout
        r = self.landing(evidence)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('Push failed', r.stderr)
        self.assertNotEqual(self.git('rev-parse', 'main').stdout, old)
        self.git('show-ref', '--verify', 'refs/heads/feature')

    def test_hook_changed_commit_tree_stops_before_push(self):
        evidence=self.evidence()
        hook=self.root/'.git/hooks/pre-commit'
        hook.write_text('#!/bin/sh\nprintf "fixture formatter change\n" >> README.md\ngit add README.md\n')
        hook.chmod(0o755)
        old=self.git('rev-parse','origin/main').stdout
        result=self.landing(evidence)
        self.assertNotEqual(result.returncode,0)
        self.assertIn('differs from the verified feature tree',result.stderr)
        self.assertEqual(self.git('rev-parse','origin/main').stdout,old)
        self.git('show-ref','--verify','refs/heads/feature')

    def test_commit_hook_source_changes_stop_before_push(self):
        evidence = self.evidence()
        hook = self.root / '.git/hooks/post-commit'
        hook.write_text('#!/bin/sh\nprintf "changed by fixture hook\\n" >> README.md\n')
        hook.chmod(0o755)
        old = self.git('rev-parse', 'origin/main').stdout
        r = self.landing(evidence)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('hook left working changes', r.stderr)
        self.assertEqual(self.git('rev-parse', 'origin/main').stdout, old)
        self.git('show-ref', '--verify', 'refs/heads/feature')


if __name__ == '__main__':
    unittest.main()
