# Screenshot reference

Paths in commands are relative to the active checkout root; helper filenames refer to `.agents/skills/run-mailven/`. Read the core SKILL.md safeguards first.

## Retaking the README screenshots

The README has three shots, all of made-up mail only:

- `docs/ss.png`: 1040×660, Dark theme, mail sidebar collapsed.
- `docs/ss-light.png`: 1040×690, Light theme, Accounts column expanded, chat scrolled to its subject heading.
- `docs/ss-narrow.png`: 480×750, single-pane layout, chat scrolled to its subject heading.

The quickest route is headless: `node .agents/skills/run-mailven/headless-shot.js <outdir>`
writes all three (see [Playwright](playwright.md)). The heights of the last two end the
chat between messages; recheck them after changing the fake conversation. The browser
steps below retake `docs/ss.png` only.

`screenshot-fakes.js` fakes the read bindings: two accounts, a chat list
and a short group chat. It also blocks every write binding, saves nothing, and
pins the in-app title bar on so the shots don't depend on the dev profile.
Edit the data there, not in the page.

1. Start the dev server and `serve-fakes.py` (both with the host background facility).
   Open a browser tab, then navigate to `http://localhost:34115`.
   The viewport must be at least 660 px tall; resize the Chrome window if it isn't.
2. Run this in the available page-evaluation tool. The page becomes a 1040×660 iframe, and the
   fakes go in before the app mounts, so its first load reads them:

   ```js
   await new Promise((r) => setTimeout(r, 2500))
   window.__ui0 = JSON.stringify(await window.go.app.App.GetUIState())
   ;(0, eval)(await (await fetch('http://127.0.0.1:8765/screenshot-fakes.js')).text())
   document.body.innerHTML = ''; document.body.style.margin = '0'
   const f = document.createElement('iframe')
   f.style.cssText = 'width:1040px;height:660px;border:0;position:fixed;left:0;top:0'
   document.body.appendChild(f); f.src = location.href
   const poll = setInterval(() => { try { const w = f.contentWindow
     if (w.location.href !== 'about:blank' && w.go?.app?.App && !w.__faked) { w.__faked = true; __installFakes(w); clearInterval(poll) } } catch {} }, 1)
   await new Promise((r) => setTimeout(r, 8000))
   ```
3. Hover outside the iframe (for example at x=1300) so no cursor shows, then
   save a browser screenshot crop of region `[0, 0, 1040, 660]` using the available API.
   Inspect its dimensions; if scaling is needed, set `saved_png` to its path and run:
   `magick "$saved_png" -alpha off -filter Lanczos -resize 1040x660\! -strip PNG24:docs/ss.png`.
4. Check that `JSON.stringify(await window.go.app.App.GetUIState())` still equals
   `__ui0`. Then close the tab and stop both servers.

The fake data has to match what the components read, or the image looks wrong:

- Every message needs `chat: { text, hasQuoted: false, isRich: false }`.
  Without it the bubbles render in full mode and look like emails.
- `GetIdentities` must return my address, or "me" counts as a recipient.
- Keep the width at 1025 px or more. Below that the app uses its overlay
  layout and shows only one pane.
