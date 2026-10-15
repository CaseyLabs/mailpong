// Page harness for the backend-free UI tests. tests/ui/run.mjs starts Vite and sets MAILVEN_UI_URL.
// Usage in a test file:  it('name', ui.run({ width: 900, theme: 'light' }, async (page) => { ... }))
// Each test gets a fresh browser context with fake bindings (screenshot-fakes.js data); nothing persists.
const fs = require('node:fs')
const path = require('node:path')
const { after } = require('node:test')

const root = path.resolve(__dirname, '../..')
const skill = path.join(root, '.agents/skills/run-mailven')
const { fakeBackendSource } = require(path.join(skill, 'fake-backend.js'))
const fakes = fs.readFileSync(path.join(skill, 'screenshot-fakes.js'), 'utf8')
const resultsDir = process.env.MAILVEN_UI_RESULTS || path.join(root, 'test-results/ui')
const baseURL = process.env.MAILVEN_UI_URL

let browser
const getBrowser = async () => (browser ??= await require(path.join(skill, 'pw.js')).launch())
after(async () => { await browser?.close() })

// Startup writes the backend-free mode rejects on purpose (sync, window sizing, notifications).
const expectedError = /^blocked: /

// Human-readable configuration, used in failure messages and screenshot names.
const describeConfig = (o) => `${o.theme} ${o.width}px accounts=${o.accounts} previews=${o.previews ? 'on' : 'off'}${o.style ? ` style=${o.style}` : ''}`

const defaults = { width: 1400, height: 800, theme: 'dark', accounts: 'expanded', previews: false, ready: '.chat-row' }

// Browser-side setup, after the fake backend and before the app's modules run.
const setupSource = (o) => `${fakes}
window.__installFakes(window);
{
  const A = window.go.app.App
  A.IsExtensionEnabled = async (name) => ['contacts', 'calendar'].includes(name)
  A.GetShowMessagePreview = async () => ${JSON.stringify(o.previews)}
  // Backend-free mode lifts the dev-build Dark override, so the app applies this theme itself.
  A.GetThemeMode = async () => ${JSON.stringify(o.theme)}
  const getUI = A.GetUIState
  A.GetUIState = async () => {
    const ui = await getUI()
    return { ...ui, collapsedSidebars: { ...ui.collapsedSidebars, mail: ${JSON.stringify(o.accounts === 'collapsed')} } }
  }
  // Writes a test allows resolve instead of rejecting.
  for (const name of ${JSON.stringify(o.allow ?? [])}) A[name] = async () => undefined
}
${o.beforeMount ?? ''}
{
  // Record every binding call (name and JSON-copied arguments) for assertions. Arguments JSON cannot copy
  // (cycles, BigInt) are recorded as a placeholder so the recorder never makes the call itself fail.
  window.__calls = []
  const copy = (args) => { try { return JSON.parse(JSON.stringify(args)) } catch (e) { return ['[unserializable: ' + e.message + ']'] } }
  for (const api of Object.values(window.go.app)) {
    for (const [name, fn] of Object.entries(api)) {
      api[name] = (...args) => { window.__calls.push({ name, args: copy(args) }); return fn(...args) }
    }
  }
}`

// Calls recorded in the page, optionally filtered by binding name.
exports.calls = (page, name) => page.evaluate((n) => window.__calls.filter((c) => !n || c.name === n), name ?? null)

exports.describeConfig = describeConfig

// run(options, fn) returns a node:test function that opens the app, runs fn(page, config, t) and closes it.
// options: width, height, theme ('dark'|'light'), accounts ('expanded'|'collapsed'), previews (bool),
// style (label suffix for a variant), allow (binding names that resolve), beforeMount (browser source), ready (selector), path (URL path).
exports.run = (options, fn) => async (t) => {
  if (!baseURL) throw new Error('MAILVEN_UI_URL is not set; run the suite with `make test-ui` or `node tests/ui/run.mjs`')
  const o = { ...defaults, ...options }
  const config = { ...o, label: describeConfig(o) }
  const context = await (await getBrowser()).newContext({ viewport: { width: o.width, height: o.height }, locale: 'en-GB', timezoneId: 'UTC', reducedMotion: 'reduce' })
  const page = await context.newPage()
  const errors = []
  page.on('pageerror', (e) => { if (!expectedError.test(e.message)) errors.push(e.message) })
  await page.addInitScript(fakeBackendSource() + '\n' + setupSource(o))
  try {
    await page.goto(baseURL + (o.path ?? '/'))
    if (o.ready) await page.waitForSelector(o.ready, { timeout: 20000 })
    // Final states only: no transition or animation can be sampled mid-way (hover reveals, theme fades).
    await page.addStyleTag({ content: '*,*::before,*::after{transition:none!important;animation-duration:0s!important;animation-delay:0s!important}' })
    const theme = await page.evaluate(() => document.documentElement.getAttribute('data-theme'))
    if (theme !== o.theme) throw new Error(`expected the app to apply the ${o.theme} theme, got ${theme}`)
    await fn(page, config, t)
    if (errors.length) throw new Error(`Unexpected page errors [${config.label}]:\n${[...new Set(errors)].slice(0, 5).join('\n')}`)
  } catch (error) {
    // Investigation artifact; the assertion message is the primary diagnostic.
    fs.mkdirSync(resultsDir, { recursive: true })
    const file = path.join(resultsDir, `${t.fullName ?? t.name}`.replace(/[^\w.-]+/g, '_').slice(0, 150) + '.png')
    await page.screenshot({ path: file }).catch(() => {})
    error.message += `\n[config: ${config.label}; screenshot: ${path.relative(root, file)}]`
    throw error
  } finally {
    await context.close()
  }
}
