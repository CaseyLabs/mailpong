"""Small standard-library Markdown helpers shared by project workflow tools."""
from __future__ import annotations

import re
from pathlib import Path
from urllib.parse import unquote, urlsplit


def mask_code(text: str) -> str:
    """Replace fenced and inline code with spaces while preserving newlines."""
    lines = text.splitlines(keepends=True)
    in_fence = None
    out = []
    for line in lines:
        m = re.match(r"^\s{0,3}(`{3,}|~{3,})", line)
        if in_fence:
            out.append("".join("\n" if c == "\n" else " " for c in line))
            if m and m.group(1)[0] == in_fence[0] and len(m.group(1)) >= len(in_fence):
                in_fence = None
            continue
        if m:
            in_fence = m.group(1)
            out.append("".join("\n" if c == "\n" else " " for c in line))
        else:
            out.append(line)
    text = "".join(out)
    return re.sub(r"(`+)([^`\n]*?)\1", lambda m: " " * len(m.group(0)), text)


def slugify(heading: str) -> str:
    heading = re.sub(r"<[^>]+>", "", heading)
    heading = re.sub(r"`([^`]*)`", r"\1", heading)
    heading = re.sub(r"\[([^]]+)\]\([^)]*\)", r"\1", heading)
    heading = heading.lower()
    heading = re.sub(r"[^\w\-\s]", "", heading, flags=re.UNICODE)
    return re.sub(r"\s+", "-", heading).strip("-")


def anchors(text: str) -> set[str]:
    # Mask fenced blocks only: inline code is valid heading text and contributes its contents.
    cleaned_lines = []
    in_fence = None
    for line in text.splitlines():
        marker = re.match(r"^\s{0,3}(`{3,}|~{3,})", line)
        if in_fence:
            cleaned_lines.append("")
            if marker and marker.group(1)[0] == in_fence[0] and len(marker.group(1)) >= len(in_fence): in_fence = None
        elif marker:
            in_fence = marker.group(1); cleaned_lines.append("")
        else:
            cleaned_lines.append(line)
    found: set[str] = set()
    used: set[str] = set()
    counts: dict[str, int] = {}
    i = 0
    while i < len(cleaned_lines):
        line = cleaned_lines[i]
        explicit = re.findall(r'<a\s+(?:[^>]*\s)?id=["\']([^"\']+)', line, re.I)
        found.update(explicit)
        h = re.match(r"^\s{0,3}(#{1,6})\s+(.+?)\s*#*\s*$", line)
        if not h:
            if i + 1 < len(cleaned_lines):
                setext = re.match(r"^\s{0,3}(=+|-+)\s*$", cleaned_lines[i + 1])
                if setext and line.strip():
                    h = (line.strip(),)
                    i += 1
        if h:
            title = h.group(2) if isinstance(h, re.Match) else h[0]
            base = slugify(title)
            n = counts.get(base, 0)
            candidate = base if n == 0 else f"{base}-{n}"
            while candidate in used:
                n += 1; candidate = f"{base}-{n}"
            counts[base] = n + 1
            used.add(candidate); found.add(candidate)
        i += 1
    return found


def links(text: str):
    """Yield (line, destination) for inline, reference, and HTML links."""
    clean = mask_code(text)
    definitions = {}
    for i, line in enumerate(clean.splitlines(), 1):
        m = re.match(r"^\s{0,3}\[([^]]+)\]:\s*(?:<([^>]+)>|(\S+))", line)
        if m:
            definitions[m.group(1).strip().lower()] = (i, m.group(2) or m.group(3))
    for i, line in enumerate(clean.splitlines(), 1):
        if re.match(r"^\s{0,3}\[[^]]+\]:", line):
            continue
        for m in re.finditer(r"!?(?:\[[^]]*\])\((<[^>]+>|[^)]*)\)", line):
            destination = m.group(1).strip()
            if destination.startswith("<") and ">" in destination:
                destination = destination[1:destination.index(">")]
            else:
                # A quoted title may follow a destination; raw spaces in paths are allowed.
                title = re.search(r'\s+["\']', destination)
                if title:
                    destination = destination[:title.start()].rstrip()
            if destination:
                yield i, destination
        for m in re.finditer(r"(?<!!)\[([^]]+)\](?:\[([^]]*)\])?", line):
            key = (m.group(2) or m.group(1)).strip().lower()
            if key in definitions:
                yield i, definitions[key][1]
        for m in re.finditer(r"(?:href|src)=[\"']([^\"']+)[\"']", line, re.I):
            yield i, m.group(1)


def resolve_target(source: Path, target: str, repo: Path):
    """Resolve a local Markdown link; return (path, anchor), or (None, None)."""
    if target.startswith("//"):
        return None, None
    parsed = urlsplit(target)
    if parsed.scheme or parsed.netloc:
        return None, None
    path = unquote(parsed.path)
    anchor = unquote(parsed.fragment)
    if path.startswith("/docs"):
        route = path[len("/docs"):].lstrip("/")
        if route == "user-guide" or route.startswith("user-guide/"):
            route = route.removeprefix("user-guide").lstrip("/")
        candidate = repo / "docs" / "user-guide" / route
    elif path.startswith("/"):
        candidate = repo / path.lstrip("/")
    elif path:
        candidate = source.parent / path
    else:
        candidate = source
    candidate = candidate.resolve()
    # Site routes require a real page. Ordinary relative links may browse a
    # repository directory (e.g. tools/) and need no index.md.
    if path.startswith("/docs") and not candidate.suffix:
        choices = [candidate.with_suffix(".md"), candidate.with_suffix(".mdx"), candidate / "index.md"]
        candidate = next((p for p in choices if p.is_file()), choices[0])
    return candidate, anchor
