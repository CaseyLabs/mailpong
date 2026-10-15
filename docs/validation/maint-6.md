# MAINT-6 — Closed sidebar keyboard isolation

Owner selected [UI2](../../ISSUES.md#ui2) on 2026-10-14. [Roadmap and inline checkpoint](../../PLAN.md#maint-6); pre-commit base `2c25557`, branch `fix/ui2-sidebar-focus`.

## Result

Mail's sidebar and the shared kit `SidebarFrame` set `inert` while narrow and closed. Descendant controls leave keyboard navigation and the accessibility tree while the sidebar remains mounted. Opening clears inert; medium/full layouts retain their existing behavior. No extension architecture boundary changes were needed; the shared primitive remains extension-agnostic under `docs/EXT_RULES.md`.

## Verification

- `node tests/ui/run.mjs sidebar-focus`: 2 passed, 0 failed/skipped. Mail and Contacts (through the real shared kit frame) reject focus on mounted controls while closed, accept focus when open, reject it after closing again, and accept focus after resizing to 900 and 1400 px.
- `python3 tools/project-check.py --base main --run`: whitespace, documentation links, frontend lint, Svelte/TypeScript check (0 errors/warnings), production build and full UI suite pass. UI result: 58 passed, 0 failed, 0 skipped. The UI2 audit allowlist entry is removed. The matrix covers both themes, narrow/medium/full widths, Mail Accounts expanded/collapsed and previews on/off.
- Initial regression development found two test defects (Mail's close button uses “Close sidebar”, and the test needed to wait for the close update). Both were corrected before the successful checks. No baseline product-check failures were observed.
- Direct correctness review and independent read-only acceptance review found no correctness defect. A behavior-preserving simplify review found no useful further change.
- Full logs are retained privately under `~/.cache/mailven-project-checks/`; the landing runner requires final source-matching evidence and rechecks after any rebase.

## Limits and follow-up

Backend-free Chromium with synthetic data only; native WebKitGTK, screen-reader operation and live accounts were not exercised. The tests do not claim native verification. No application servers were left running by the test runner.

Static review confirmed separate hidden responsive viewer controls remain focusable; recorded as [UI4](../../ISSUES.md#ui4), unselected and unfixed. UI1, UI3 and CH1 remain outside this task. The existing Gmail investigation is preserved on `docs/send-2-gmail-verification`; it was not included in this fix branch.
