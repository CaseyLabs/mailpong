# Composer reference

Paths in commands are relative to the active checkout root; helper filenames refer to `.agents/skills/run-mailven/`. Read the core SKILL.md safeguards first.

### Composer and undo-send checks

```bash
node .agents/skills/run-mailven/undo-send-check.js "$SCRATCH"   # dev server and serve-fakes.py running; about a minute
```

It prints PASS/FAIL for chat reply undo, the main composer's Undo toast, and
the detached composer's countdown (Undo, Send now), each with the fake binding
log, and exits non-zero on a failure. Chat screenshots cover the Accounts
column both ways.

For your own composer checks, use `composer-fakes.js`. Its header gives both
uses: `__installComposerFakes(w)` after `__installFakes(w)` in the main
window, or `page.addInitScript({ path })` before `/composer.html`, which in
a browser has no `window.go` or `window.runtime` of its own. Every faked call
lands in `w.__log`. Queued sends fire after `w.__sendDelay` ms (default
10000) through `send:finished`. Nothing is sent or saved.
