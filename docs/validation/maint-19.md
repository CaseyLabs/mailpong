# MAINT-19 — Binding generation leaves no file-mode diff

Owner authorization: the unattended `tools/project-run.py` run started 2026-10-16 selects this milestone only. [Roadmap](../../PLAN.md#maint-19); base `0d79461`, branch `milestone/maint-19`.

## Result

- The `generate` target in the `Makefile` runs `chmod 644 frontend/wailsjs/runtime/*` after `wails generate module`, which writes those files as 0755.
- [Regenerating Wails bindings](../../CONTRIBUTING.md#regenerating-wails-bindings) names `make generate`, `wails dev` and `wails build` as generator paths and says to chmod after a direct `wails generate module`.
- MA9 is resolved.

## Verification

- Mailven stopped; `make generate`, then `git status --short frontend/wailsjs` printed nothing; runtime files are 0644.
- `make lint`: 0 issues, contrast check passes.
- `tools/project-check.py --base 0d79461 --run`: whitespace, doc links and `make test` passed.

## Limits

Review and acceptance were checked directly (no subagent) on a three-file change. Only `make generate` was run; `wails dev` and `wails build` were not re-run.
