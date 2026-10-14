# Playwright reference

Paths in commands are relative to the active checkout root; helper filenames refer to `.agents/skills/run-mailven/`. Read the core SKILL.md safeguards first.

## Playwright checks (local and cloud)

Headless Playwright needs no browser connector. `pw.js` probes the local cache and `/opt/node-tools/node_modules/playwright`; `PLAYWRIGHT_MODULE` overrides selection. It uses Playwright's bundled browser when executable, otherwise probes `/opt/pw-browsers/chromium`; `PLAYWRIGHT_CHROMIUM` overrides it. Check availability before launching; see the core prerequisites for display setup.

Set `SCRATCH` to an absolute output directory and `prefix` to a descriptive filename prefix.

```bash
# If a display is missing, start Xvfb as described in the core workflow.
.agents/skills/run-mailven/dev.sh start          # host background facility; then dev.sh wait
node .agents/skills/run-mailven/headless-shot.js "$SCRATCH"          # ss.png plus vc-<theme>-<state>.png
node .agents/skills/run-mailven/ui-matrix.js "$SCRATCH" "$prefix"     # the full matrix
```

`headless-shot.js` and `ui-matrix.js` share `fake-page.js`: it installs the
screenshot fakes before the Vite entry point mounts, including Wails
cache-busting query URLs, supplies Contacts/Calendar
visibility through fake reads, and never enables extensions in the real profile.
Its leading setup-module import runs before entry-point dependencies, so a
`beforeMount` local event dispatcher also precedes module-scope listeners.
It captures persisted UI/extension state from a bindings-only page, removes fakes
by reloading, waits for debounced saves, asserts state is unchanged, and always
closes the browser. Neither helper needs `serve-fakes.py`; the composer helpers
still use that server. `headless-shot.js` writes the README shot plus the
dark/light x Accounts-column expanded/collapsed crops. When retaking the README screenshot, copy `ss.png` to
`docs/ss.png` as is.

`ui-matrix.js <outdir> <prefix>` uses the same page setup and writes 36 PNGs:
`<prefix>-<dark|light>-<1400|900|700>-<expanded|collapsed>-<previews|nopreviews>.png`
(at 700 px the app is in overlay layout with no Accounts toggle, so only the
`expanded` file exists), plus `<prefix>-<theme>-<state>-<previews|nopreviews>-row-<hover|focus>.png`
crops at 1400 px. Compare a before and an after set without loading images:

```bash
for f in "$SCRATCH"/before/*.png; do n=$(basename "$f"); echo "$(compare -metric AE "$f" "$SCRATCH/after/${n/before/after}" null: 2>&1) $n"; done | sort -rn | head
```

Back-to-back runs on an unchanged tree differ by up to about 200 pixels per
image (relative times in the fake data, and a 9x9 px spot at the window
edge), so treat an AE under about 300 as unchanged. Before and after must
come from the same environment. The local `before` set lives in
`~/.cache/mailven-upnote-ui/before/`; the cloud container is fresh each
session, so rebuild it from the clean checkout first.

- It sets `locale: 'en-GB'` so times are 24-hour, as in the old screenshot.
- Dev builds force Dark, so the scripts set `data-theme="light"` by hand.
- ImageMagick 7: `magick` and `compare` exist; `convert` is deprecated.
- Stop only the fake server and display processes started for this check, using their recorded PIDs/session handles.
- The native window and dock icon aren't visible under Xvfb; report that.
- zsh treats an unquoted `==` as a command: quote it in `echo`.
- Expected noise in page errors: `blocked: <Binding>` from the fakes, and `Cannot read
  properties of null (reading 'nodes')` from the Svelte copy bundled in `/wails/ipc.js`.
- On a cold Vite the faked chat rows can mount more than 8 s after load; in your own
  scripts wait for `.chat-row` (`frame.waitForSelector`) rather than a fixed delay.
- When a fake UI check emits runtime events inside an iframe, override that
  iframe’s `runtime.EventsOnMultiple` and `runtime.EventsEmit` with a local
  dispatcher before the app mounts. Native Wails dispatch also reaches the parent
  app, which can open a second modal and intercept clicks.
- Finish Go edits before starting a UI check. Go hot reload replaces the page and
  detaches the iframe; recreate the fake frame if a rebuild occurs.
