#!/usr/bin/env bash
# Brand check: fail on any "Aerion" mention that is not covered by
# tools/brand-check.allow. Each allowlist line is
#   <path glob> :: <ERE matched against the line text> :: <reason>
# A match is allowed when its file matches the glob and its text matches the
# regex. Use '.' as the regex to allow a whole file.
set -euo pipefail
cd "$(dirname "$0")/.."

allow=tools/brand-check.allow
status=0

# Every entry needs three fields, the last being a non-empty reason.
bad=$(grep -v -E '^\s*(#|$)' "$allow" | awk -F' :: ' 'NF != 3 || $3 == "" {print}')
if [ -n "$bad" ]; then
	echo "brand-check: allowlist entries without a reason:" >&2
	echo "$bad" >&2
	exit 1
fi

# Binary files are skipped (-I); archive/ and history are attribution only, and
# this script and its allowlist necessarily spell the name out. Exit 1 means no
# match; any other failure (not a git checkout, bad pathspec) must not pass.
rc=0
matches=$(git grep -n -I -i 'aerion' -- . ':!archive' ':!docs/history' \
	':!tools/brand-check.sh' ':!tools/brand-check.allow') || rc=$?
if [ "$rc" -gt 1 ]; then
	echo "brand-check: git grep failed (exit $rc)" >&2
	exit 1
fi

unallowed=$(printf '%s\n' "$matches" | awk -v allow="$allow" '
	BEGIN {
		n = 0
		while ((getline line < allow) > 0) {
			if (line ~ /^[ \t]*(#|$)/) continue
			split(line, f, " :: ")
			glob[n] = f[1]; re[n] = f[2]; n++
		}
		for (i = 0; i < n; i++) {
			g = glob[i]
			gsub(/[.+^$()|{}\\[\\]?]/, "\\\\&", g)
			gsub(/\*\*/, "\001", g)
			gsub(/\*/, "[^/]*", g)
			gsub(/\001/, ".*", g)
			rx[i] = "^" g "$"
		}
	}
	NF == 0 { next }
	{
		p1 = index($0, ":"); file = substr($0, 1, p1 - 1)
		rest = substr($0, p1 + 1); p2 = index(rest, ":"); text = substr(rest, p2 + 1)
		for (i = 0; i < n; i++)
			if (file ~ rx[i] && text ~ re[i]) next
		print
	}')

if [ -n "$unallowed" ]; then
	echo "brand-check: unallowed 'Aerion' mentions (rename them or add a reasoned entry to $allow):" >&2
	printf '%s\n' "$unallowed" | cut -c1-200 >&2
	status=1
fi
exit $status
