// Exercise selection without launching browsers or depending on installed modules.
const assert = require('assert/strict')
const fs = require('fs')
const path = require('path')
const vm = require('vm')
const source = fs.readFileSync(path.join(__dirname, '../pw.js'), 'utf8')
function check(env, modules, executables) {
  const exports = {}
  vm.runInNewContext(source, {
    exports, process: { env },
    require(name) {
      if (name === 'fs') return {
        constants: { X_OK: 1 }, existsSync: p => modules.includes(p),
        accessSync(p) { if (!executables.includes(p)) throw Error('missing') },
      }
      if (name === 'os') return { homedir: () => '/fixture' }
      if (name === 'path') return path
      assert(modules.includes(name))
      return { chromium: { executablePath: () => '/bundled', launch: opts => opts.executablePath } }
    },
  })
  return exports.launch()
}
const local = '/fixture/.cache/mailven-upnote-ui/node_modules/playwright'
const cloud = '/opt/node-tools/node_modules/playwright'
assert.equal(check({}, [local], ['/bundled']), '/bundled')
assert.equal(check({ CLAUDE_CODE_REMOTE: 'true' }, [local], ['/bundled']), '/bundled')
assert.equal(check({}, [cloud], ['/opt/pw-browsers/chromium']), '/opt/pw-browsers/chromium')
assert.equal(check({ PLAYWRIGHT_MODULE: '/custom module', PLAYWRIGHT_CHROMIUM: '/custom browser' }, ['/custom module'], ['/custom browser']), '/custom browser')
assert.throws(() => check({ PLAYWRIGHT_MODULE: '/missing' }, [local], ['/bundled']), /Playwright not found/)
assert.throws(() => check({}, [], []), /Playwright not found/)
assert.throws(() => check({}, [local], []), /Chromium executable not found/)
assert.throws(() => check({ PLAYWRIGHT_CHROMIUM: '/missing' }, [local], ['/bundled']), /Chromium executable not found/)
console.log('PASS launcher capability selection, overrides and missing dependencies')
