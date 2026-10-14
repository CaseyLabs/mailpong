#!/usr/bin/env python3
"""Check local links in changed Markdown files (or explicitly supplied paths)."""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

from project_markdown import anchors, links, resolve_target


def git(root: Path, *args: str):
    raw = subprocess.check_output(["git", *args], cwd=root, stderr=subprocess.DEVNULL)
    return [os.fsdecode(part) for part in raw.split(b"\0") if part]


def changed_paths(root: Path, base: str | None):
    args = ["diff", "--name-only", "-z", "--diff-filter=ACMRTUXB"]
    args += [base, "--"] if base else ["HEAD", "--"]
    paths=set(git(root,*args))
    if not base:
        paths.update(git(root,"diff","--cached","--name-only","-z","--diff-filter=ACMRTUXB"))
    paths.update(git(root,"ls-files","--others","--exclude-standard","-z"))
    return {p for p in paths if p.lower().endswith((".md",".mdx"))}


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument("paths",nargs="*")
    ap.add_argument("--changed",action="store_true")
    ap.add_argument("--base")
    ap.add_argument("--json",action="store_true")
    ap.add_argument("--json-output", help="write the complete JSON report to this path")
    ap.add_argument("--max-errors",type=int,default=50)
    args=ap.parse_args()
    try:
        root=Path(subprocess.check_output(["git","rev-parse","--show-toplevel"],text=True,stderr=subprocess.DEVNULL).strip())
        if args.max_errors < 0: raise ValueError("--max-errors must be nonnegative")
        explicit=set()
        for value in args.paths:
            path=(Path.cwd()/value).resolve()
            explicit.add(str(path.relative_to(root)) if path.is_relative_to(root) else str(path))
        names=set(explicit)
        if args.changed or not args.paths: names.update(changed_paths(root,args.base))
        errors=[]
        # A base selects committed changes against that ref; otherwise staged and unstaged plus untracked.
        for name in sorted(names):
            source=(root/name).resolve()
            if source.suffix.lower() not in (".md",".mdx"): continue
            if not source.exists():
                if name in explicit: errors.append({"file":name,"line":1,"message":"explicit Markdown source does not exist"})
                continue
            try: body=source.read_text(encoding="utf-8")
            except (OSError,UnicodeError) as e:
                errors.append({"file":name,"line":1,"message":f"cannot read Markdown: {e}"}); continue
            for line,dest in links(body):
                target,anchor=resolve_target(source,dest,root)
                if target is None: continue
                if not target.exists():
                    errors.append({"file":name,"line":line,"message":f"missing link target: {dest}"}); continue
                if anchor and target.suffix.lower() in (".md",".mdx"):
                    try: target_anchors=anchors(target.read_text(encoding="utf-8"))
                    except (OSError,UnicodeError): target_anchors=set()
                    if anchor not in target_anchors:
                        errors.append({"file":name,"line":line,"message":f"missing anchor #{anchor} in {target.relative_to(root) if target.is_relative_to(root) else target}"})
        report={"root":str(root),"checked":sorted(names),"error_count":len(errors),"errors":errors}
        if args.json_output:
            with Path(args.json_output).open("x",encoding="utf-8") as stream:
                stream.write(json.dumps(report,indent=2)+"\n")
        shown=errors[:args.max_errors]
        if args.json: print(json.dumps({**report,"errors":shown,"errors_truncated":len(shown)<len(errors)},indent=2))
        else:
            for e in shown: print(f"{e['file']}:{e['line']}: {e['message']}")
            if len(shown)<len(errors): print(f"... {len(errors)-len(shown)} additional errors; see --json-output report.")
            print(f"Checked {len(names)} Markdown path(s); {len(errors)} error(s).")
        return 1 if errors else 0
    except (subprocess.CalledProcessError,OSError,ValueError) as e:
        print(f"check-doc-links: {e}",file=sys.stderr); return 2


if __name__=="__main__": raise SystemExit(main())
