#!/bin/sh
# Print a plan file's heading outline (with line numbers) and its Progress section,
# so a session can read only the sections it needs instead of the whole plan.
# Relative paths (including default PLAN.md) resolve from the caller’s cwd.
plan="${1:-PLAN.md}"
if [ ! -f "$plan" ]; then
	echo "No plan file at $plan (cwd: $(pwd))."
	exit 1
fi
echo "== Outline of $plan ($(wc -l <"$plan") lines). Read sections by line range."
grep -n '^#' "$plan"
echo
echo "== Progress section"
awk '/^## Progress/{p=1; print; next} p && /^## /{exit} p' "$plan"
exit 0
