# MAINT-8 — Closed responsive viewer keyboard isolation

Owner selected MAINT-8 ([UI4](../../ISSUES.md#ui4)) on 2026-10-15. [Roadmap and inline checkpoint](../../PLAN.md#maint-8); pre-commit base `efc42fb`, branch `fix/ui4-viewer-focus`.

## Result

Mail's conversation viewer (`App.svelte`, `<main data-pane="viewer">`) and the shared kit `DetailPane` set `inert` while they are a closed responsive overlay (medium ≤1024 px and narrow ≤767 px). Their descendants, including Back, leave keyboard navigation and the accessibility tree while staying mounted. Open viewers, thread/message focus mode (which shows the viewer overlay at any width) and the full layout are unchanged.

Closing hands focus back to the list. Mail moves the focused pane from `viewer` to `messageList` and, if DOM focus was in the viewer, focuses the list container. `DetailPane` focuses the list through a new optional `PaneNavTarget.focus`, which kit `ListPane` registers. This is a generic, extension-agnostic kit addition under `docs/EXT_RULES.md`.

The hand-off matters because ChatView handles Tab through a window listener whenever the focused pane is `viewer`, wherever DOM focus actually is. Clicking Back sets that pane, so once the viewer was inert, Tab targeted unfocusable messages, called `preventDefault()` and stopped moving. The full UI audit caught this at 700 px ("Tab reached only 0 … focusable controls"). Before this milestone the same handler sent Tab into the hidden viewer's messages.

## Verification

- `node tests/ui/run.mjs viewer-focus`: 4 passed, 0 failed/skipped. Mail and Contacts (through the real kit `DetailPane`, with one synthetic contact) at 700 and 900 px. Each checks that an open viewer accepts focus; that after Back focus is outside the viewer, Tab never enters it and reaches a visible control, and every mounted viewer control rejects focus; that reopening (row click) accepts focus again; and that 1400 px accepts focus. Mail also covers thread focus mode (`f`, then Escape) from a closed viewer.
- Regression proof: without `inert`, all 4 fail ("closed viewer must reject focus on every control"). Without Mail's hand-off, the Mail cases fail ("Tab after closing the viewer reaches a visible control"). Without `DetailPane`'s hand-off, the Contacts cases fail ("closing the viewer hands focus to the list").
- `python3 tools/project-check.py --base main --run`: whitespace, documentation links, frontend lint, Svelte/TypeScript check, production build and full UI suite pass; UI result 99 passed, 0 failed, 0 skipped. The audit matrix covers both themes, narrow/medium/full widths, Mail Accounts column expanded/collapsed and previews on/off.
- Direct diff review found no correctness defect; a simplify review found no useful change. The independent read-only acceptance check passed all four criteria and found no new stubs; it relied on the tests above rather than reading the diff.
- The MAINT-6 sidebar does not have the stuck-Tab problem: a probe that closed the narrow sidebar and pressed Tab reached "Toggle sidebar".

## Limits

Backend-free Chromium with synthetic data only. Native WebKitGTK, screen-reader operation and live accounts were not exercised.
