"""Portable path smoke checks; no application server or account access."""
import pathlib
import subprocess
import tempfile
import shutil

repo = pathlib.Path(__file__).resolve().parents[4]
helper = repo / '.agents/skills/project-next-milestone/plan-outline.sh'
def run(*args, cwd=repo, ok=True):
    result = subprocess.run(args, cwd=cwd, capture_output=True, text=True)
    assert (result.returncode == 0) == ok, result.stderr + result.stdout
    return result.stdout
with tempfile.TemporaryDirectory(prefix='mailven paths ') as tmp:
    tmp = pathlib.Path(tmp)
    content = '# Fixture\n## Scope\nSmall\n## Progress\n- [ ] Task\n## Decision Log\nHidden\n'
    (tmp / 'PLAN.md').write_text(content)
    (tmp / 'space plan.md').write_text(content)
    for args in [[], ['PLAN.md'], ['space plan.md'], [str(tmp / 'space plan.md')]]:
        out = run('sh', str(helper), *args, cwd=tmp)
        assert '- [ ] Task' in out and 'Hidden' not in out
    assert 'No plan file' in run('sh', str(helper), 'missing.md', cwd=tmp, ok=False)
    worktree = tmp / 'work tree'
    run('git', 'worktree', 'add', '--detach', str(worktree), 'HEAD')
    try:
        # Test the current edited resources, not only HEAD's earlier versions.
        shutil.copytree(repo / '.agents/skills', worktree / '.agents/skills', dirs_exist_ok=True)
        for checkout in [repo, worktree]:
            for cwd in [checkout, checkout / 'frontend']:
                for tree in ['.agents/skills', '.claude/skills']:
                    base = checkout / tree
                    assert run('git', '-C', str(base), 'rev-parse', '--show-toplevel', cwd=cwd).strip() == str(checkout)
                    for item in (checkout / '.agents/skills').rglob('*'):
                        if item.is_file():
                            assert item.samefile(base / item.relative_to(checkout / '.agents/skills'))
                    assert '## Progress' in run('sh', str(base / 'project-next-milestone/plan-outline.sh'), str(checkout / 'PLAN.md'), cwd=cwd)
    finally:
        run('git', 'worktree', 'remove', '--force', str(worktree))
print('PASS default/explicit/missing/spaced plans; root/subdirectory/worktree resource identity')
