// Shared launcher. Overrides are explicit paths; otherwise probe installed capabilities.
const fs = require('fs')
const os = require('os')
const path = require('path')

const candidates = [
  path.join(os.homedir(), '.cache/mailven-upnote-ui/node_modules/playwright'),
  '/opt/node-tools/node_modules/playwright',
]
const mod = process.env.PLAYWRIGHT_MODULE || candidates.find(p => fs.existsSync(p))
if (!mod || !fs.existsSync(mod)) {
  throw new Error(`Playwright not found${mod ? ` at ${mod}` : ''}. Install outside frontend/package.json: npm install --prefix ~/.cache/mailven-upnote-ui --no-save playwright@1.63.0`)
}
const { chromium } = require(mod)
const executable = p => {
  try { fs.accessSync(p, fs.constants.X_OK); return true } catch { return false }
}
const chromiumPath = process.env.PLAYWRIGHT_CHROMIUM ||
  [chromium.executablePath(), '/opt/pw-browsers/chromium'].find(executable)
if (!chromiumPath || !executable(chromiumPath)) {
  throw new Error(`Chromium executable not found${chromiumPath ? ` at ${chromiumPath}` : ''}; install the Playwright browser or set PLAYWRIGHT_CHROMIUM.`)
}
exports.launch = () => chromium.launch({ executablePath: chromiumPath })
