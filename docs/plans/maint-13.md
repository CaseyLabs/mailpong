# MAINT-13 — Split oversized components

Authorized by the owner's selection in session on 2026-10-16 (local scope only; no live checks). Base commit: 054800c on branch `maint-13-split-components`. Issue: UI6.

## Goal
Every tracked Svelte component (excluding `archive/`) under 500 lines, behavior unchanged. Calendar components follow `docs/EXT_RULES.md`. One commit per component split.

## Progress
Listing: `git ls-files '*.svelte' | grep -v '^archive/' | xargs wc -l | sort -n | awk '$1>500'`

- [x] composer/Composer.svelte (2487 → 175; state class + 14 modules + 5 child components; lint, check, full UI suite 107/107 pass; send/draft/attachment/PGP flows not covered by the suite, native unrun)
- [x] settings/AccountForm.svelte (1742 → 104; state class, 5 modules, 8 child components in settings/account/; lint, check, full UI suite 107/107 pass; the suite does not open the account form, native unrun)
- [x] settings/account/AccountSecurityTab.svelte (1152 → 35; state class, 3 modules, 3 child components; lint, check, full UI suite 107/107 pass; no UI test covers this tab, native unrun)
- [x] calendar EventComposerDialog.svelte (905 → 397; eventComposerLogic.ts + eventComposerState.svelte.ts 491; EXT_RULES R21/R26/R29 respected; lint, check, full UI suite 107/107 pass; no UI test covers it, native unrun)
- [x] calendar views/TimelineView.svelte (894 → 373; timelineLayout.ts + timelineDrag.svelte.ts; EXT_RULES respected; lint, check, full UI suite 107/107 pass; no UI test covers it, native unrun)
- [x] settings/account/AccountServerTab.svelte (885 → 407; 4 self-contained child components + label helper; ConfirmDialog moved into a child but portals, so DOM unchanged; lint, check, full UI suite 107/107 pass; no UI test covers it, native unrun)
- [x] settings/ContactSourceDialog.svelte (881 → 99; state class, actions + effects modules, CardDAV/OAuth/sync-interval components; lint, check, full UI suite 107/107 pass; no UI test covers it, native unrun)
- [x] calendar CalendarSettingsDialog.svelte (872 → 295; Local/Sources/Defaults section components, dialogs kept in parent outside the focus trap; EXT_RULES respected; lint, check, full UI suite 107/107 pass; no UI test covers it, native unrun)
- [x] viewer/EmailBody.svelte (865 → 481; emailBodyHtml.ts, emailBodyIframe.ts, EmailBodyImageBanner.svelte; CSP, sandbox, image blocking, source check and every replace() line verified identical against HEAD; lint, check, full UI suite 107/107 pass; rendering not covered by UI tests, native unrun)
- [x] settings/account/SignatureEditor.svelte (705 → 465; editor setup + helpers modules, color and font-size pickers; lint, check, full UI suite 107/107 pass; no UI test covers it, native unrun)
- [x] sidebar/Sidebar.svelte (671 → 479; sidebarNav.ts, SidebarSyncFooter, SidebarEmptyState; covered by sidebar and responsive suites; lint, check, full UI suite 107/107 pass)
- [x] calendar EventDetail.svelte (609 → 489; eventDetailRRule.ts + EventDetailAbout.svelte, host-sanitized descriptionHTML passed unchanged; lint, check, full UI suite 107/107 pass; no UI test covers it, native unrun)
- [x] settings/AccountDialog.svelte (603 → 480; accountDialogActions.ts, setup-hooks step, tab list, edit actions; no UI test covers it; lint, check, full UI suite 107/107 pass)
- [x] composer/EditorToolbar.svelte (551 → 477; hint badge, color and font-size menus; lint, check, full UI suite 107/107 pass)
- [x] settings/GeneralTab.svelte (512 → 430; background and startup sections; lint, check, full UI suite 107/107 pass)

## Checkpoint
Done 2026-10-16. Base 054800c; one commit per component, then a close-out commit (PLAN, ISSUES, this file, the `project-check.py` frontend classification of `extensions/<name>/frontend/` with a test, and whitespace fixes to `emailBodyIframe.ts` blank lines and `signatureEditorHelpers.ts` EOF).

Results: the UI6 listing prints only the total (largest component 493 lines, untouched `chat/ChatList.svelte`). `python3 tools/project-check.py --base 054800c --run`: whitespace, docs, tooling, skill-paths, brand, frontend-lint, frontend-check, frontend-build and ui (107 passed, 0 failed, 0 skipped) all PASS.

## Decisions
- Splits were delegated one or two components at a time to `project-worker` and reviewed here: state kinds ($state, $derived, plain) compared against the base for the state-class splits; no class method is passed unbound as a callback; EmailBody's CSP, sandbox, image blocking, message-source check and every `replace()` line compared identical to the base; calendar files stay in `extensions/calendar/frontend/` (EXT_RULES R21, R26, R29).
- Accepted structural moves: `ConfirmDialog` inside `AccountServerTrustedCerts` (it portals); the duplicated contact-source sync-interval select became one component; `calendarCountLabel` is duplicated in two calendar sections to keep reactive `$_`.

## Unrun / limits
The UI suite opens the composer and sidebar (page errors fail it) but not the account settings, signature editor, email body or calendar components; send, draft, attachment, encryption, account edit, remote-image and calendar drag flows are verified by type checks and review only. Native WebKitGTK and real-backend checks unrun.
