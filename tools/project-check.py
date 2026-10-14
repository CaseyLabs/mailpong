#!/usr/bin/env python3
"""Plan or run serial project checks, with compact logs and content-bound evidence."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import time

sys.dont_write_bytecode = True
from project_common import WorkflowError, changed_paths, checkout_lock, git, repository, resolve_path, tree_fingerprint

SCOPES = ('docs', 'tooling', 'go', 'frontend')


def infer_scopes(paths):
    scopes, unknown = set(), []
    for name in paths:
        if name.endswith(('.md', '.mdx')):
            scopes.add('docs')
            if name.startswith(('.agents/', '.claude/', '.codex/')):
                scopes.add('tooling')
        elif name.startswith(('frontend/', 'tests/ui/')):
            scopes.add('frontend')
        elif name.endswith('.go') or name in ('go.mod', 'go.sum', 'Makefile'):
            scopes.add('go')
        elif name.startswith(('tools/', '.agents/', '.claude/', '.codex/', '.githooks/')) or name in ('.gitignore', 'CLAUDE.md'):
            scopes.add('tooling')
        else:
            unknown.append(name)
    return scopes, unknown


def command_plan(repo, scopes, base, extra=(), go_build=None):
    if go_build is None:
        go_build = 'go' in scopes and not (repo / 'frontend/dist').is_dir()
    commands = [{'name': 'whitespace', 'argv': ['git', 'diff', '--check', base, '--'], 'cwd': '.'}]
    if 'docs' in scopes:
        commands.append({'name': 'docs', 'argv': ['python3', 'tools/check-doc-links.py', '--changed', '--base', base], 'cwd': '.'})
    if 'tooling' in scopes:
        commands.extend([
            {'name': 'tooling', 'argv': ['python3', '-m', 'unittest', 'discover', '-s', 'tools/tests', '-p', 'test_*.py'], 'cwd': '.'},
            {'name': 'skill-paths', 'argv': ['python3', '.agents/skills/project-next-milestone/tests/paths.py'], 'cwd': '.'},
            {'name': 'brand', 'argv': ['make', 'lint-brand'], 'cwd': '.'},
        ])
    if 'frontend' in scopes or go_build:
        if 'frontend' in scopes:
            commands.extend([
                {'name': 'frontend-lint', 'argv': ['npm', 'run', 'lint'], 'cwd': 'frontend'},
                {'name': 'frontend-check', 'argv': ['npm', 'run', 'check'], 'cwd': 'frontend'},
            ])
        commands.append({'name': 'frontend-build', 'argv': ['npm', 'run', 'build'], 'cwd': 'frontend'})
    if 'go' in scopes:
        commands.append({'name': 'go', 'argv': ['make', 'test'], 'cwd': '.'})
    if 'frontend' in scopes:
        commands.append({'name': 'ui', 'argv': ['make', 'test-ui'], 'cwd': '.'})
    commands.extend({'name': f'extra-{i + 1}', 'argv': argv, 'cwd': '.'} for i, argv in enumerate(extra))
    return commands


def run_checks(repo, commands, output_dir, metadata, tail_lines=6):
    output_dir.mkdir(parents=True, exist_ok=False, mode=0o700)
    evidence = {**metadata, 'schema_version': 1, 'commands': commands, 'results': [], 'passed': False}
    evidence_file = output_dir / 'evidence.json'
    environment = {**os.environ, 'PYTHONDONTWRITEBYTECODE': '1'}
    for index, command in enumerate(commands):
        log = output_dir / f'{index + 1:02d}-{command["name"]}.log'
        start = time.monotonic()
        print(f'RUN {command["name"]}: {shlex.join(command["argv"])} (cwd={command["cwd"]})', flush=True)
        with log.open('w', encoding='utf-8') as stream:
            try:
                result = subprocess.run(command['argv'], cwd=repo / command['cwd'],
                                        stdout=stream, stderr=subprocess.STDOUT, env=environment)
                code = result.returncode
            except OSError as error:
                stream.write(str(error) + '\n')
                code = 127
        elapsed = round(time.monotonic() - start, 3)
        evidence['results'].append({'name': command['name'], 'exit_code': code, 'seconds': elapsed, 'log': str(log)})
        print(f'{"PASS" if code == 0 else "FAIL"} {command["name"]} ({elapsed}s); log={log}', flush=True)
        # Bound returned context; keep the complete diagnostic on disk.
        from collections import deque
        with log.open(errors='replace') as stream:
            for line in deque(stream, maxlen=tail_lines):
                print('  ' + line.rstrip()[:500])
        evidence_file.write_text(json.dumps(evidence, indent=2) + '\n')
        if code:
            print(f'STOP: remaining checks unrun; evidence={evidence_file}')
            return code if code > 0 else 1, evidence_file
    evidence['final_fingerprint'] = tree_fingerprint(repo)
    evidence['passed'] = evidence['final_fingerprint'] == evidence['tree_fingerprint']
    if not evidence['passed']:
        print('FAIL: checks changed source files; review the changes and rerun checks.')
    evidence_file.write_text(json.dumps(evidence, indent=2) + '\n')
    print(f'Evidence: {evidence_file}')
    return (0 if evidence['passed'] else 1), evidence_file


def parse_extra(values):
    commands = []
    for value in values:
        try:
            command = json.loads(value)
        except ValueError as error:
            raise WorkflowError('--extra-command must be a JSON argv array') from error
        if not isinstance(command, list) or not command or not all(isinstance(x, str) and x and '\0' not in x for x in command):
            raise WorkflowError('--extra-command needs a nonempty array of nonempty strings')
        commands.append(command)
    return commands


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--scope', action='append', choices=SCOPES, default=[])
    parser.add_argument('--base', default='HEAD', help='Changed-source baseline (use milestone base or main for landing evidence)')
    parser.add_argument('--run', action='store_true', help='Execute the displayed checks; default only plans them')
    parser.add_argument('--output-dir', help='New private log directory; relative to invocation cwd')
    parser.add_argument('--tail-lines', type=int, default=6)
    parser.add_argument('--extra-command', action='append', default=[], help='Additional required command as JSON argv, cwd=repo; no shell')
    parser.add_argument('--json', action='store_true', help='JSON plan (not combined with --run)')
    args = parser.parse_args(argv)
    if args.json and args.run:
        parser.error('--json is for plans; --run already writes evidence.json')
    if not 0 <= args.tail_lines <= 30:
        parser.error('--tail-lines must be between 0 and 30')
    try:
        repo = repository()
        base = git(repo, 'rev-parse', '--verify', args.base + '^{commit}').stdout.strip()
        paths = changed_paths(repo, base)
        inferred, unknown = infer_scopes(paths)
        if unknown:
            raise WorkflowError('Unclassified changed paths; select their validation manually: ' + ', '.join(unknown[:10]))
        scopes = inferred | set(args.scope)
        go_build = 'go' in scopes and not (repo / 'frontend/dist').is_dir()
        commands = command_plan(repo, scopes, base, parse_extra(args.extra_command), go_build=go_build)
        metadata = {'repo': str(repo), 'head': git(repo, 'rev-parse', 'HEAD').stdout.strip(),
                    'base': base, 'go_build': go_build, 'scopes': sorted(scopes), 'changed_paths': paths,
                    'tree_fingerprint': tree_fingerprint(repo),
                    'created_at': datetime.now(timezone.utc).isoformat()}
        if not args.run:
            report = {**metadata, 'commands': commands, 'execute': False}
            if args.json:
                print(json.dumps(report, indent=2))
            else:
                print(f'PLAN ONLY: {repo}; scopes={",".join(sorted(scopes)) or "whitespace"}; changed={len(paths)}')
                for command in commands:
                    print(f'  {command["name"]}: {shlex.join(command["argv"])} (cwd={command["cwd"]})')
                print('Add plan-specific checks with --extra-command; use --run to execute. Native/live cases remain separate.')
            return 0
        if args.output_dir:
            output = resolve_path(args.output_dir)
        else:
            cache = Path(os.environ.get('XDG_CACHE_HOME', str(Path.home() / '.cache'))) / 'mailven-project-checks'
            cache.mkdir(parents=True, exist_ok=True, mode=0o700)
            # Reserve a unique name, then let run_checks create it exclusively.
            output = Path(tempfile.mkdtemp(prefix='run-', dir=cache))
            output.rmdir()
        if output == repo or repo in output.parents:
            raise WorkflowError('Keep check logs outside the checkout so evidence cannot alter its source fingerprint')
        with checkout_lock(repo):
            # Inputs may have changed while the user reviewed the plan.
            if tree_fingerprint(repo) != metadata['tree_fingerprint']:
                raise WorkflowError('Source changed before checks started; regenerate the plan')
            code, _ = run_checks(repo, commands, output, metadata, args.tail_lines)
        return code
    except (WorkflowError, OSError) as error:
        print(f'ERROR: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
