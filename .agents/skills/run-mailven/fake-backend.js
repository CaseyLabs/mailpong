// Backend-free mode: serve the frontend with plain Vite and fake every Wails binding.
// No Go process, display, profile or account is involved, so nothing real can be read or written.
// Bindings come from the generated frontend/wailsjs/go/app/App.d.ts: reads (Get/List/Search/Is/Has/Find)
// resolve to the fresh-store setting value or an empty value of their declared type; every other binding rejects with 'blocked: <name>'.
// screenshot-fakes.js and per-check beforeMount code then replace the reads they need.
const fs = require('node:fs')
const path = require('node:path')
const net = require('node:net')
const os = require('node:os')
const { spawn } = require('node:child_process')

const root = path.resolve(__dirname, '../../..')
const frontend = path.join(root, 'frontend')

// name -> empty value of the declared return type, for one generated bindings file.
const bindingDefaults = (file) => {
  const src = fs.readFileSync(path.join(frontend, 'wailsjs/go/app', file), 'utf8')
  const out = {}
  for (const [, name, type] of src.matchAll(/^export function (\w+)\([^)]*\):Promise<(.+)>;$/gm)) {
    out[name] = /^Array</.test(type) ? [] : /^Record</.test(type) ? {} : type === 'boolean' ? false
      : type === 'number' ? 0 : type === 'string' ? '' : null
  }
  return out
}

// Fresh-store setting values, kept current by internal/settings TestFrontendFakeDefaults.
const settingDefaults = () => ({
  ...JSON.parse(fs.readFileSync(path.join(__dirname, 'settings-defaults.json'), 'utf8')),
  GetTermsAccepted: true,
})

// Browser-side source that installs window.go and window.runtime. Runs before the app's entry point.
exports.fakeBackendSource = () => `(() => {
  const reads = /^(Get|List|Search|Is|Has|Find)/
  const make = (defaults) => {
    const api = {}
    for (const [name, value] of Object.entries(defaults)) {
      api[name] = reads.test(name.replace(/^[A-Za-z]+_/, ''))
        ? async () => structuredClone(value)
        : async () => { throw new Error('blocked: ' + name) }
    }
    return api
  }
  const app = make(${JSON.stringify({ ...bindingDefaults('App.d.ts'), ...settingDefaults() })})
  app.IsReady = async () => true
  window.go = { app: { App: app, ComposerApp: make(${JSON.stringify(bindingDefaults('ComposerApp.d.ts'))}) } }
  // Wails runtime: a local event bus; window, clipboard and notification calls are inert.
  const handlers = {}
  const runtime = {
    EventsOnMultiple: (name, cb, max) => {
      const entry = { cb, left: max > 0 ? max : Infinity }
      ;(handlers[name] ??= []).push(entry)
      return () => { handlers[name] = (handlers[name] ?? []).filter((x) => x !== entry) }
    },
    EventsEmit: (name, ...data) => {
      for (const entry of [...(handlers[name] ?? [])]) {
        if (--entry.left <= 0) handlers[name] = handlers[name].filter((x) => x !== entry)
        entry.cb(...data)
      }
    },
    EventsOff: (...names) => names.forEach((n) => delete handlers[n]),
    EventsOffAll: () => Object.keys(handlers).forEach((n) => delete handlers[n]),
    Environment: async () => ({ buildType: 'dev', platform: 'linux', arch: 'amd64' }),
    ClipboardGetText: async () => '',
    ScreenGetAll: async () => [],
    WindowGetSize: async () => ({ w: innerWidth, h: innerHeight }),
    WindowGetPosition: async () => ({ x: 0, y: 0 }),
  }
  window.runtime = new Proxy(runtime, { get: (t, k) => t[k] ?? (() => Promise.resolve()) })
  window.__fakeBackend = true
})();`

const free = (port) => new Promise((resolve) => {
  const s = net.createServer().once('error', () => resolve(false)).once('listening', () => s.close(() => resolve(true)))
  s.listen(port, '127.0.0.1')
})

const reachable = async (url) => {
  try { return (await fetch(url)).ok } catch { return false }
}

// Start Vite on a free port (default 34116) and resolve to { url, stop }. Reuses MAILVEN_UI_URL when set.
exports.startVite = async ({ port = Number(process.env.MAILVEN_VITE_PORT) || 34116 } = {}) => {
  if (process.env.MAILVEN_UI_URL) return { url: process.env.MAILVEN_UI_URL, stop: async () => {} }
  while (!(await free(port))) port++
  const vite = path.join(frontend, 'node_modules/.bin/vite')
  if (!fs.existsSync(vite)) throw new Error('frontend dependencies missing: run `cd frontend && npm ci`')
  if (!fs.existsSync(path.join(frontend, 'public/spellcheck'))) {
    await new Promise((resolve, reject) => spawn(process.execPath, ['scripts/copy-dictionaries.mjs'], { cwd: frontend, stdio: 'ignore' })
      .once('exit', (code) => (code ? reject(new Error('copy-dictionaries failed')) : resolve())))
  }
  const child = spawn(vite, ['--host', '127.0.0.1', '--port', String(port), '--strictPort', '--clearScreen', 'false'],
    { cwd: frontend, stdio: ['ignore', 'pipe', 'pipe'], detached: true })
  let log = ''
  child.stdout.on('data', (d) => { log = (log + d).slice(-4000) })
  child.stderr.on('data', (d) => { log = (log + d).slice(-4000) })
  const url = `http://127.0.0.1:${port}`
  // 'exit' fires once whether Vite exits normally or is killed by a signal (exitCode stays null then).
  let running = true
  const exited = new Promise((resolve) => child.once('exit', () => { running = false; resolve() }))
  // Vite runs in its own process group, so a terminal Ctrl-C or a crash of this process never reaches it:
  // kill the group on this process's exit and on SIGINT/SIGTERM/SIGHUP.
  const kill = () => { if (running) try { process.kill(-child.pid, 'SIGTERM') } catch { /* already gone */ } }
  const signals = ['SIGINT', 'SIGTERM', 'SIGHUP']
  const onSignal = (sig) => { kill(); process.exit(128 + os.constants.signals[sig]) }
  process.once('exit', kill)
  for (const sig of signals) process.once(sig, onSignal)
  const stop = async () => {
    process.off('exit', kill)
    for (const sig of signals) process.off(sig, onSignal)
    kill()
    await exited
  }
  for (let i = 0; i < 300 && running; i++) {
    if (await reachable(url + '/src/main.ts')) return { url, stop }
    await new Promise((r) => setTimeout(r, 100))
  }
  await stop()
  throw new Error(`Vite did not start on ${url}:\n${log}`)
}
