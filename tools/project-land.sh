#!/bin/sh
# Resolve sibling code independently of the caller's cwd, including path spaces.
set -eu
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec python3 "$script_dir/project_land.py" "$@"
