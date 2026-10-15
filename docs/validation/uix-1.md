# UIX-1 — Nord retune and radius token

Owner selected UIX-1 on 2026-10-14. [Roadmap and inline checkpoint](../../PLAN.md#uix-1); [design](../plans/ui-refresh.md#palette-and-type-uix-1); pre-commit base `d5d3bec`, branch `feat/uix-1-nord-retune`.

## Result

- Nord surface, divider, selected, bubble and muted tokens in `frontend/src/themes/built-in.css` are near-neutral greys (10-14 % saturation) in both themes; Frost and Aurora accents are unchanged.
- Light `--primary` and `--ring` are 213 40% 40%, so Compose (`--primary-foreground` on primary) and the selected Inbox label (primary on `--selected`) reach at least 4.5:1 (UI1). The UI1 allowlist entry is deleted, so `tests/ui/ui-audit-allow.cjs` is empty and guards the fix.
- `--radius: 0.625rem` is now defined in `frontend/src/app.css`. It had been absent since `162e5a2` (theme reduction), which left `rounded-lg`, `-md` and `-sm` computing to no radius; they now render as 10, 8 and 6 px. This is the "softer radius" and also restores corners across about 70 files.
- [UI_TOKENS](../UI_TOKENS.md) lists the new values and the radius token.

## Deviation

Base text size and line height are unchanged. Row, header and calendar pixel budgets are computed from them (see UI_TOKENS), so enlarging them is a separate, riskier change; no later UIX milestone depends on it.

## Verification

- `node tools/contrast-check.mjs`: all text tiers pass on every surface in both themes (lowest 4.70:1, `--fg-tertiary` on dark `--selected`).
- `python3 tools/project-check.py --base main --run`: whitespace, documentation links, frontend lint, Svelte/TypeScript check (0 errors/warnings), production build and full UI suite pass. UI result: 58 passed, 0 failed, 0 skipped, including the UI quality audit across both themes, 700/900/1400 px and Accounts expanded/collapsed/overlay with an empty allowlist.
- `make lint` passes (golangci-lint, ESLint, brand check, contrast check).

## Limits

Chromium with synthetic data only. No screenshot review of the new look, native WebKitGTK rendering (fonts, radius) or real-backend run was done; these remain unrun.
