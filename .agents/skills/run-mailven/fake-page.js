// Mount screenshot fakes before the Vite entry point runs; never mount a real UI.
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { launch } = require('./pw')
// Wails adds a cache-busting query after a backend hot reload.
const mainEntry = /\/src\/main\.ts(?:\?.*)?$/

exports.withFakePage = async (viewport, check, options = {}) => {
  const browser = await launch()
  let state, before, page
  const snapshot = () => state.evaluate(async () => {
    const A = window.go.app.App
    return {
      ui: await A.GetUIState(),
      contacts: await A.IsExtensionEnabled('contacts'),
      calendar: await A.IsExtensionEnabled('calendar'),
    }
  })
  try {
    state = await browser.newPage()
    await state.route(mainEntry, route => route.abort())
    await state.goto('http://localhost:34115')
    await state.waitForFunction(() => window.go?.app?.App)
    before = await snapshot()
    page = await browser.newPage({ viewport, locale: 'en-GB' })
    const errors = []
    page.on('pageerror', error => {
      if (!/blocked: |reading 'nodes'/.test(error.message)) errors.push(error.message)
    })
    const fakes = fs.readFileSync(path.join(__dirname, 'screenshot-fakes.js'), 'utf8')
    await page.route(mainEntry, async route => {
      const response = await route.fetch()
      const setup = `${fakes}
window.__installFakes(window);
const realIsEnabled = window.go.app.App.IsExtensionEnabled;
window.go.app.App.IsExtensionEnabled = name => ['contacts', 'calendar'].includes(name) ? Promise.resolve(true) : realIsEnabled(name);
${options.beforeMount || ''}`
      // A leading import executes before the entry point's other dependencies.
      // Prepending ordinary statements runs too late for EventsOn at module scope.
      const initModule = 'data:text/javascript;base64,' + Buffer.from(setup).toString('base64')
      await route.fulfill({ response, body: `import ${JSON.stringify(initModule)};\n${await response.text()}` })
    })
    await page.goto('http://localhost:34115')
    await page.waitForSelector('.chat-row', { timeout: 30000 })
    await check(page)
    assert.deepEqual(errors, [], 'Unexpected page errors')
  } finally {
    try {
      if (page) {
        await page.unroute(mainEntry)
        await page.route(mainEntry, route => route.abort())
        await page.reload()
        await page.waitForTimeout(1700)
        assert.equal(await page.evaluate(() => typeof window.__installFakes), 'undefined')
      }
      if (before) {
        assert.deepEqual(await snapshot(), before, 'Persisted UI or extension settings changed')
        console.log('PASS fakes removed; persisted UI and extension settings unchanged')
      }
    } finally {
      await browser.close()
    }
  }
}
