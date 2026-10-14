#!/usr/bin/env python3
"""Print a bounded, read-only view of project planning context."""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import urlsplit

from project_markdown import links, resolve_target, slugify


def git(cwd: Path, *args: str) -> str:
    return subprocess.check_output(["git", *args], cwd=cwd, text=True, stderr=subprocess.DEVNULL).strip()


def lines_for(path: Path):
    return path.read_text(encoding="utf-8").splitlines()


def section(lines, start, end=None):
    return "\n".join(f"{i+1}: {lines[i]}" for i in range(start, end if end is not None else len(lines)))


def heading_bounds(lines, predicate, level=None):
    result = []
    for i, line in enumerate(lines):
        m = re.match(r"^(#{1,6})\s+(.+?)\s*#*\s*$", line)
        if m and (level is None or len(m.group(1)) == level) and predicate(m.group(2)):
            depth = len(m.group(1)); end = len(lines)
            for j in range(i + 1, len(lines)):
                n = re.match(r"^(#{1,6})\s+", lines[j])
                if n and len(n.group(1)) <= depth:
                    end = j; break
            result.append((i, end, m.group(2)))
    return result


def portfolio_blocks(lines):
    headings = [(i, len(m.group(1)), m.group(2).strip()) for i, line in enumerate(lines)
                if (m := re.match(r"^(#{1,6})\s+(.+?)\s*#*\s*$", line))]
    portfolios = [(i, d) for i, d, t in headings if t.lower() == "portfolio"]
    progresses = [(i, d) for i, d, t in headings if t.lower() == "progress"]
    if len(portfolios) > 1 or len(progresses) > 1:
        raise ValueError("plan has ambiguous Portfolio or Progress headings")
    p = portfolios[0] if portfolios else None
    prog = progresses[0] if progresses else None
    if p is None or prog is None or prog <= p:
        # Legacy outline style commonly uses Scope / Progress as top-level headings.
        scopes = [(i, d) for i, d, t in headings if re.match(r"Scope(?:\s*/.*)?$", t, re.I)]
        if len(scopes) > 1: raise ValueError("plan has ambiguous legacy Scope headings")
        scope = scopes[0] if scopes else None
        if scope is None or prog is None or prog[0] <= scope[0]:
            raise ValueError("plan needs unambiguous Portfolio and Progress sections (or legacy Scope and Progress)")
        p = scope
    def end_of(start, depth):
        return next((i for i, d, _ in headings if i > start and d <= depth), len(lines))
    return [(p[0], end_of(p[0], p[1]), "Portfolio"),
            (prog[0], end_of(prog[0], prog[1]), "Progress")]


def anchor_range(lines, anchor):
    """Return source bounds for an explicit HTML id or generated heading slug."""
    explicit = re.compile(r'<a\s+[^>]*id=["\']' + re.escape(anchor) + r'["\']', re.I)
    for i, line in enumerate(lines):
        if explicit.search(line):
            for j in range(i + 1, len(lines)):
                h = re.match(r"^(#{1,6})\s+", lines[j])
                if h:
                    depth = len(h.group(1))
                    return j, next((k for k in range(j + 1, len(lines)) if (m := re.match(r"^(#{1,6})\s+", lines[k])) and len(m.group(1)) <= depth), len(lines))
    slug_counts = {}
    for i, line in enumerate(lines):
        h = re.match(r"^(#{1,6})\s+(.+?)\s*#*\s*$", line)
        if h:
            depth, title = len(h.group(1)), h.group(2)
        elif i + 1 < len(lines) and re.match(r"^\s{0,3}(=+|-+)\s*$", lines[i + 1]):
            title, depth = line.strip(), (1 if lines[i + 1].strip().startswith("=") else 2)
        else:
            continue
        base = slugify(title); n = slug_counts.get(base, 0); slug_counts[base] = n + 1
        actual = base if n == 0 else f"{base}-{n}"
        if actual == anchor:
            j = i + (2 if i + 1 < len(lines) and re.match(r"^\s{0,3}(=+|-+)\s*$", lines[i + 1]) else 1)
            return i, next((k for k in range(j, len(lines)) if (m := re.match(r"^(#{1,6})\s+", lines[k])) and len(m.group(1)) <= depth), len(lines))
    return None


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("milestone", nargs="?")
    ap.add_argument("--plan", default="PLAN.md")
    ap.add_argument("--issues")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--max-lines", type=int, default=300)
    args = ap.parse_args()
    try:
        cwd = Path.cwd()
        root = Path(git(cwd, "rev-parse", "--show-toplevel"))
        plan = (cwd / args.plan).resolve()
        if not plan.is_file(): raise ValueError(f"plan not found: {args.plan}")
        if args.max_lines < 1: raise ValueError("--max-lines must be positive")
        plan_lines = lines_for(plan)
        issue_path = Path(args.issues).resolve() if args.issues else plan.parent / "ISSUES.md"
        selected = args.milestone
        sections = []
        if selected:
            matches = heading_bounds(plan_lines, lambda t: re.match(r"^" + re.escape(selected) + r"(?:\s|$|[—–:])", t, re.I))
            if len(matches) != 1:
                raise ValueError(f"milestone {selected!r} matched {len(matches)} headings; expected exactly one")
            start, end, title = matches[0]
            # Include its project intro, bounded to the nearest preceding project-level heading.
            project = None
            for i in range(start - 1, -1, -1):
                h = re.match(r"^(#{1,6})\s+(.+?)\s*#*\s*$", plan_lines[i])
                if h and len(h.group(1)) <= 2:
                    project = (i, len(h.group(1)), h.group(2)); break
            if project:
                project_depth = project[1]
                intro_end = start
                for j in range(project[0] + 1, start):
                    nested = re.match(r"^(#{1,6})\s+", plan_lines[j])
                    if nested and len(nested.group(1)) > project_depth:
                        intro_end = j; break
                selected_lines = [f"{i+1}: {plan_lines[i]}" for i in range(project[0], intro_end)]
            else:
                selected_lines = []
            selected_lines += [f"{i+1}: {plan_lines[i]}" for i in range(start, end)]
            sections += [(str(plan), f"selected project context ({title})", "\n".join(selected_lines))]
            selected_content = "\n".join(selected_lines)
            refs = []
            for _, destination in links(selected_content):
                parsed = urlsplit(destination)
                if not parsed.scheme and not parsed.netloc and parsed.path.lower().endswith("issues.md") and parsed.fragment:
                    refs.append(("issue", parsed.fragment))
                elif not parsed.scheme and not parsed.netloc and "docs/plans/" in parsed.path.lower() and parsed.path:
                    refs.append(("checkpoint", destination))
            issue_refs = sorted({fragment for kind, fragment in refs if kind == "issue"})
            if issue_refs:
                if not issue_path.is_file(): raise ValueError(f"referenced issue register is missing: {issue_path}")
                ilines = lines_for(issue_path); excerpts=[]
                for anchor in issue_refs:
                    bounds = anchor_range(ilines, anchor)
                    if bounds is None: raise ValueError(f"issue anchor #{anchor} not found in {issue_path}")
                    entry_heading = next((line for line in ilines[bounds[0]:bounds[1]] if re.match(r"^#{2,6}\s+", line)), "")
                    if not re.match(r"^#{2,6}\s+[A-Z][A-Za-z0-9-]*\d[A-Za-z0-9-]*\s+[—-]", entry_heading):
                        continue  # Ignore links to project/register overview anchors.
                    excerpts.append(section(ilines, *bounds))
                if excerpts:
                    sections.append((str(issue_path), "linked issues", "\n\n".join(excerpts)))
            checkpoint_refs = [value for kind, value in refs if kind == "checkpoint"]
            if checkpoint_refs:
                checkpoints = sorted({resolve_target(plan, value, root)[0] for value in checkpoint_refs})
                for cp in checkpoints:
                    if cp is None or not cp.is_file(): raise ValueError(f"referenced checkpoint is missing: {cp}")
                    sections.append((str(cp), "execution checkpoint", section(lines_for(cp), 0)))
            else:
                cp = plan.parent / "docs" / "plans" / f"{selected.lower()}.md"
                if not cp.exists(): cp = root / "docs" / "plans" / f"{selected.lower()}.md"
                if cp.is_file(): sections.append((str(cp), "execution checkpoint", section(lines_for(cp), 0)))
        else:
            for a,b,name in portfolio_blocks(plan_lines): sections.append((str(plan), name, section(plan_lines,a,b)))
        # Add checkout facts, never execute recorded commands.
        try: branch=git(root,"branch","--show-current") or "(detached)"
        except subprocess.CalledProcessError: branch="unknown"
        try:
            porcelain=git(root,"status","--porcelain")
            dirty=bool(porcelain)
        except subprocess.CalledProcessError: dirty=None
        head=git(root,"rev-parse","--short","HEAD")
        budget=max(0,args.max_lines-3); output=[]; remaining=budget
        for path,label,content in sections:
            rows=content.splitlines()
            take=rows[:remaining]
            output.append({"file":path,"section":label,"lines":take,"truncated":len(rows)>len(take)})
            remaining -= len(take)
            if remaining <= 0: break
        truncated=any(x["truncated"] for x in output) or len(output)<len(sections)
        result={"repository":str(root),"head":head,"branch":branch,"dirty":dirty,"milestone":selected,
                "sections":output,"truncated":truncated,"follow_up":"Rerun with a larger --max-lines value or read the cited file:line range." if truncated else None}
        if args.json: print(json.dumps(result,indent=2))
        else:
            print(f"Repository: {root} (HEAD {head}, branch {branch}, {'dirty' if dirty else 'clean' if dirty is not None else 'status unknown'})")
            for item in output:
                print(f"\n[{item['section']}] {item['file']}")
                for row in item["lines"]: print(row)
                if item["truncated"]: print("[truncated; continue reading this file]")
            if truncated: print("\nOutput bounded; rerun with a larger --max-lines or read cited files.")
        return 0
    except (ValueError, OSError, subprocess.CalledProcessError) as exc:
        print(f"project-context: {exc}", file=sys.stderr); return 2


if __name__ == "__main__": raise SystemExit(main())
