// Shared Playwright launcher for the run-mailpong scripts. See "Playwright checks" in SKILL.md.
// Picks the cloud or local install; $PLAYWRIGHT_MODULE / $PLAYWRIGHT_CHROMIUM override both paths.
const fs = require('fs')
const os = require('os')
const path = require('path')

const cloud = process.env.CLAUDE_CODE_REMOTE === 'true' || fs.existsSync('/opt/node-tools/node_modules/playwright')
const mod =
  process.env.PLAYWRIGHT_MODULE ||
  (cloud ? '/opt/node-tools/node_modules/playwright' : path.join(os.homedir(), '.cache/mailpong-upnote-ui/node_modules/playwright'))
const chromiumPath = process.env.PLAYWRIGHT_CHROMIUM || (cloud ? '/opt/pw-browsers/chromium' : undefined)

if (!fs.existsSync(mod)) {
  console.error(`Playwright not found at ${mod}. Reinstall (never into frontend/package.json):`)
  console.error('  npm install --prefix ~/.cache/mailpong-upnote-ui --no-save playwright@1.63.0')
  process.exit(1)
}

const { chromium } = require(mod)

exports.cloud = cloud
exports.launch = () => chromium.launch(chromiumPath ? { executablePath: chromiumPath } : {})
