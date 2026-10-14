"""Small Git/path primitives for the project workflow entrypoints."""
import hashlib
import os
from pathlib import Path
import subprocess


class WorkflowError(Exception):
    pass


def git(repo, *args, check=True):
    result = subprocess.run(['git', '-C', str(repo), *args], capture_output=True,
                            text=True, errors='replace')
    if check and result.returncode:
        raise WorkflowError(result.stderr.strip() or f'git {args[0]} failed ({result.returncode})')
    return result


def repository(cwd=None):
    return Path(git(cwd or Path.cwd(), 'rev-parse', '--show-toplevel').stdout.strip()).resolve()


def changed_paths(repo, base='HEAD'):
    tracked = git(repo, 'diff', '--name-only', '-z', base, '--').stdout.split('\0')
    untracked = git(repo, 'ls-files', '--others', '--exclude-standard', '-z').stdout.split('\0')
    return sorted(set(p for p in tracked + untracked if p))


def tree_fingerprint(repo):
    """Hash tracked + nonignored untracked working files, independent of commit ID.

    Checks run before a commit can therefore prove the identical committed tree.
    Ignored build/cache output isn't a source change; symlinks aren't followed.
    """
    names = git(repo, 'ls-files', '--cached', '--others', '--exclude-standard', '-z').stdout
    digest = hashlib.sha256()
    for name in sorted(set(p for p in names.split('\0') if p)):
        path = repo / name
        if not path.exists() and not path.is_symlink():
            continue  # Deleted sources must fingerprint identically before/after their commit.
        digest.update(os.fsencode(name) + b'\0')
        if path.is_symlink():
            digest.update(b'link\0' + os.fsencode(os.readlink(path)))
        elif path.is_file():
            digest.update(b'exec\0' if path.stat().st_mode & 0o111 else b'file\0')
            with path.open('rb') as stream:
                for block in iter(lambda: stream.read(1024 * 1024), b''):
                    digest.update(block)
        else:
            raise WorkflowError(f'Unsupported source path: {name}')
        digest.update(b'\0')
    return digest.hexdigest()


def resolve_path(value, cwd=None):
    path = Path(value).expanduser()
    return (path if path.is_absolute() else (cwd or Path.cwd()) / path).resolve()


def checkout_lock(repo):
    """Serialize helper checks and landing in this checkout; retain lock inode."""
    import contextlib

    @contextlib.contextmanager
    def held():
        path = Path(git(repo, 'rev-parse', '--git-path', 'project-workflow.lock').stdout.strip())
        if not path.is_absolute():
            path = repo / path
        with path.open('a+b') as stream:
            if os.name == 'nt':
                import msvcrt
                if stream.tell() == 0:
                    stream.write(b'\0')
                    stream.flush()
                stream.seek(0)
                try:
                    msvcrt.locking(stream.fileno(), msvcrt.LK_NBLCK, 1)
                except OSError as error:
                    raise WorkflowError('Another helper check/landing is running in this checkout') from error
                try:
                    yield
                finally:
                    stream.seek(0)
                    msvcrt.locking(stream.fileno(), msvcrt.LK_UNLCK, 1)
            else:
                import fcntl
                try:
                    fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
                except BlockingIOError as error:
                    raise WorkflowError('Another helper check/landing is running in this checkout') from error
                try:
                    yield
                finally:
                    fcntl.flock(stream, fcntl.LOCK_UN)
    return held()
