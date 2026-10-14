#!/usr/bin/env python3
"""Preview or execute repository-policy squash landing with verified check evidence."""
import argparse
from datetime import datetime, timezone
import importlib.util
import json
from pathlib import Path
import re
import sys
import tempfile

sys.dont_write_bytecode = True
from project_common import WorkflowError, checkout_lock, git, repository, resolve_path, tree_fingerprint

spec = importlib.util.spec_from_file_location('project_check', Path(__file__).with_name('project-check.py'))
checks = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checks)


def branch_ref(repo, branch):
    if branch.startswith('-') or git(repo, 'check-ref-format', '--branch', branch, check=False).returncode:
        raise WorkflowError('Invalid branch name')
    ref = 'refs/heads/' + branch
    git(repo, 'rev-parse', '--verify', ref + '^{commit}')
    return ref


def validate_evidence(repo, path, required_scopes):
    try:
        evidence = json.loads(path.read_text())
    except (OSError, ValueError) as error:
        raise WorkflowError(f'Cannot read check evidence: {path}') from error
    if evidence.get('schema_version') != 1 or evidence.get('passed') is not True:
        raise WorkflowError('Checks are failed or incomplete')
    fingerprint = tree_fingerprint(repo)
    if evidence.get('tree_fingerprint') != fingerprint or evidence.get('final_fingerprint') != fingerprint:
        raise WorkflowError('Check evidence is stale; rerun checks for the current source tree')
    if evidence.get('repo') != str(repo):
        raise WorkflowError('Check evidence belongs to another checkout')
    scopes = evidence.get('scopes', [])
    if not isinstance(scopes, list) or not set(scopes).issubset(checks.SCOPES) or not required_scopes.issubset(scopes):
        raise WorkflowError('Check evidence does not cover the changed scopes')
    base = evidence.get('base', '')
    if not isinstance(base, str) or not re.fullmatch(r'[0-9a-f]{40,64}', base):
        raise WorkflowError('Invalid evidence baseline')
    if git(repo, 'merge-base', '--is-ancestor', base, 'HEAD', check=False).returncode:
        raise WorkflowError('Check evidence baseline is not an ancestor of this branch')
    commands = evidence.get('commands', [])
    results = evidence.get('results', [])
    if not isinstance(commands, list) or not isinstance(results, list) or len(commands) != len(results):
        raise WorkflowError('Check results are incomplete')
    go_build = evidence.get('go_build', False)
    if not isinstance(go_build, bool):
        raise WorkflowError('Invalid build prerequisite evidence')
    required = checks.command_plan(repo, set(scopes), base, go_build=go_build)
    if commands[:len(required)] != required:
        raise WorkflowError('Evidence omits or changes required check commands')
    extra = []
    for i, command in enumerate(commands[len(required):], 1):
        if not isinstance(command, dict) or command.get('name') != f'extra-{i}' or command.get('cwd') != '.':
            raise WorkflowError('Invalid additional check command')
        extra.extend(checks.parse_extra([json.dumps(command.get('argv'))]))
    for command, result in zip(commands, results):
        if not isinstance(result, dict) or result.get('name') != command.get('name') or result.get('exit_code') != 0:
            raise WorkflowError('A required check is failed or unrun')
        log = result.get('log')
        if not isinstance(log, str) or not Path(log).is_file():
            raise WorkflowError('A required check log is missing')
    return evidence, extra


def preflight(repo, branch, main):
    feature = branch_ref(repo, branch)
    target = branch_ref(repo, main)
    if branch == main:
        raise WorkflowError('Feature branch must differ from main')
    if git(repo, 'symbolic-ref', '--short', 'HEAD', check=False).stdout.strip() != branch:
        raise WorkflowError('Run landing from the feature branch being landed')
    if git(repo, 'status', '--porcelain').stdout:
        raise WorkflowError('Working tree/index must be clean before landing')
    for marker in ('rebase-merge', 'rebase-apply', 'MERGE_HEAD', 'CHERRY_PICK_HEAD'):
        path = Path(git(repo, 'rev-parse', '--git-path', marker).stdout.strip())
        if not path.is_absolute():
            path = repo / path
        if path.exists():
            raise WorkflowError('Finish or abort the existing Git operation first')
    changes = git(repo, 'diff', '--name-only', '-z', target + '...' + feature, '--').stdout.split('\0')
    scopes, unknown = checks.infer_scopes([p for p in changes if p])
    if unknown:
        raise WorkflowError('Unclassified changed paths need manual validation before automated landing: ' + ', '.join(unknown[:10]))
    if not any(changes):
        raise WorkflowError('No feature changes to squash; inspect whether it is already landed')
    remote = git(repo, 'config', '--get', f'branch.{main}.remote', check=False).stdout.strip()
    merge = git(repo, 'config', '--get', f'branch.{main}.merge', check=False).stdout.strip()
    if not remote or not merge.startswith('refs/heads/'):
        raise WorkflowError(f'{main} needs a configured upstream remote and branch')
    upstream = git(repo, 'rev-parse', '--verify', main + '@{upstream}', check=False)
    if upstream.returncode or git(repo, 'merge-base', '--is-ancestor', upstream.stdout.strip(), target, check=False).returncode:
        raise WorkflowError('Local main lacks known upstream commits; reconcile it before landing')
    # Git worktrees cannot check out a branch already used by another checkout.
    for chunk in git(repo, 'worktree', 'list', '--porcelain').stdout.split('\n\n'):
        if f'branch {target}\n' in chunk + '\n':
            worktree = next((line[9:] for line in chunk.splitlines() if line.startswith('worktree ')), '')
            if Path(worktree).resolve() != repo:
                raise WorkflowError(f'{main} is checked out in another worktree; land from a checkout where it is available')
    return scopes, remote, merge


def land(repo, args, evidence, extra, remote, merge):
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%f')
    recovery = 'recovery/project-land-' + stamp
    git(repo, 'branch', recovery + '-feature', 'HEAD')
    git(repo, 'branch', recovery + '-main', args.main)
    print(f'Recovery branches: {recovery}-feature, {recovery}-main', flush=True)
    # Never rewrite a branch before retaining its original local head.
    ahead = int(git(repo, 'rev-list', '--count', args.branch + '..' + args.main).stdout.strip())
    if ahead:
        result = git(repo, 'rebase', args.main, check=False)
        if result.returncode:
            conflicts = git(repo, 'diff', '--name-only', '--diff-filter=U').stdout.strip()
            git(repo, 'rebase', '--abort')
            raise WorkflowError('Rebase failed and was aborted; conflicting files: ' + (conflicts or '(see Git diagnostic)') + '\n' + result.stderr[-1500:])
        print('Rebased onto main; rerunning the recorded required checks.', flush=True)
        base = git(repo, 'rev-parse', args.main).stdout.strip()
        scopes = set(evidence['scopes'])
        go_build = 'go' in scopes and not (repo / 'frontend/dist').is_dir()
        fingerprint = tree_fingerprint(repo)
        metadata = {**evidence, 'head': git(repo, 'rev-parse', 'HEAD').stdout.strip(), 'base': base, 'go_build': go_build,
                    'tree_fingerprint': fingerprint, 'created_at': datetime.now(timezone.utc).isoformat()}
        cache = args.evidence.parent
        output = Path(tempfile.mkdtemp(prefix='rebased-', dir=cache))
        output.rmdir()
        code, path = checks.run_checks(repo, checks.command_plan(repo, scopes, base, extra, go_build=go_build), output, metadata)
        if code:
            raise WorkflowError(f'Rebased checks failed; feature retained and nothing landed. Evidence: {path}')
        validate_evidence(repo, path, scopes)
    if git(repo, 'status', '--porcelain').stdout:
        raise WorkflowError('Checks left source/index changes; review and commit them before landing')
    git(repo, 'switch', args.main)
    result = git(repo, 'merge', '--squash', args.branch, check=False)
    if result.returncode:
        raise WorkflowError('Squash failed; feature and recovery branches retained. Resolve the Git state before retrying.\n' + result.stderr[-1500:])
    with tempfile.NamedTemporaryFile(mode='w', encoding='utf-8', prefix='squash-', suffix='.txt', dir=args.evidence.parent, delete=False) as message:
        message.write(args.title + '\n\n' + args.body_file.read_text())
        message_path = Path(message.name)
    try:
        result = git(repo, 'commit', '-F', str(message_path), check=False)
    finally:
        message_path.unlink(missing_ok=True)
    if result.returncode:
        raise WorkflowError('Squash commit failed; staged changes and feature retained. Fix the failure without bypassing hooks.\n' + result.stderr[-1500:])
    if git(repo, 'status', '--porcelain').stdout:
        raise WorkflowError('Commit hook left working changes; feature/recovery branches retained and push skipped')
    if git(repo, 'diff', '--quiet', args.main, args.branch, '--', check=False).returncode:
        raise WorkflowError('Squash commit differs from the verified feature tree; push skipped, feature retained')
    head = git(repo, 'rev-parse', '--short', 'HEAD').stdout.strip()
    print(f'Committed {head}; pushing {args.main} to its configured upstream.', flush=True)
    result = git(repo, 'push', '--', remote, f'HEAD:{merge}', check=False)
    if result.returncode:
        raise WorkflowError(f'Push failed; local main commit {head}, feature and recovery branches retained.\n' + result.stderr[-1500:])
    if git(repo, 'diff', '--quiet', args.main, args.branch, '--', check=False).returncode:
        raise WorkflowError('Main and feature trees differ; feature retained')
    git(repo, 'branch', '-D', args.branch)
    print(f'LANDED {head}; feature deleted after equal-tree check; recovery branches retained.')
    return 0


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('branch')
    parser.add_argument('--main', default='main')
    parser.add_argument('--evidence', type=resolve_path, help='Successful project-check evidence matching current source')
    parser.add_argument('--title', help='Descriptive squash commit title')
    parser.add_argument('--body-file', type=resolve_path, help='Reviewed commit body file, relative to invocation cwd')
    parser.add_argument('--execute', action='store_true', help='Perform landing; default is read-only preview')
    args = parser.parse_args(argv)
    try:
        repo = repository()
        scopes, remote, merge = preflight(repo, args.branch, args.main)
        evidence, extra = (None, [])
        if args.evidence:
            evidence, extra = validate_evidence(repo, args.evidence, scopes)
        if not args.execute:
            print(f'PREVIEW ONLY: {args.branch} -> {args.main}; scopes={",".join(sorted(scopes)) or "none"}')
            print(f'Rebase if needed, rerun checks after rebase, retain recovery branches, squash/commit, push {remote}:{merge}, compare trees, then delete feature.')
            print('Check evidence: ' + ('valid' if evidence else 'required for --execute'))
            print('Review/acceptance and authorization must be completed by the caller before --execute.')
            return 0
        if evidence is None or not args.title or args.body_file is None:
            raise WorkflowError('--execute requires --evidence, --title and --body-file')
        if not re.match(r'^(?:feat|fix|docs|refactor|test|chore):\s+\S', args.title):
            raise WorkflowError('Use a descriptive prefixed commit title')
        if not args.body_file.is_file() or not args.body_file.read_text().strip():
            raise WorkflowError('Provide a nonempty reviewed commit body file')
        with checkout_lock(repo):
            # Recheck all source/Git gates after acquiring the shared operation lock.
            scopes, remote, merge = preflight(repo, args.branch, args.main)
            evidence, extra = validate_evidence(repo, args.evidence, scopes)
            return land(repo, args, evidence, extra, remote, merge)
    except (WorkflowError, OSError, ValueError, TypeError) as error:
        print(f'ERROR: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
