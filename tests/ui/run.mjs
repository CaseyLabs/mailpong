// Backend-free frontend test runner: starts Vite once, then runs tests/ui/*.test.cjs with node:test.
// Usage: node tests/ui/run.mjs [file-or-name-filter...]      (make test-ui)
// Writes test-results/ui/{junit.xml,results.json} and failure screenshots; on GitHub Actions also
// emits ::error annotations and a step summary. Exit code 1 on any failure.
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'
import { run } from 'node:test'
import { spec, junit } from 'node:test/reporters'

const here = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(here, '../..')
const require = createRequire(import.meta.url)
const { startVite } = require(path.join(root, '.agents/skills/run-mailven/fake-backend.js'))

const resultsDir = process.env.MAILVEN_UI_RESULTS ||= path.join(root, 'test-results/ui')
fs.rmSync(resultsDir, { recursive: true, force: true })
fs.mkdirSync(resultsDir, { recursive: true })

const filters = process.argv.slice(2)
const files = fs.readdirSync(here).filter((f) => f.endsWith('.test.cjs'))
  .filter((f) => !filters.length || filters.some((x) => f.includes(path.basename(x))))
  .map((f) => path.join(here, f))
if (!files.length) {
  console.error(`no test files match ${filters.join(' ')}`)
  process.exit(2)
}

const vite = await startVite()
process.env.MAILVEN_UI_URL = vite.url
const rel = (f) => (f ? path.relative(root, f.replace(/^file:\/\//, '')) : undefined)

// The first stack frame inside a test file locates a failure more precisely than the test's own line.
const failureLocation = (data) => {
  const stack = String(data.details?.error?.cause?.stack ?? data.details?.error?.stack ?? '')
  const m = stack.match(/(\/[^\s():]+\/tests\/ui\/[^\s():]+\.(?:test\.)?cjs):(\d+)/)
  return m ? { file: rel(m[1]), line: Number(m[2]) } : { file: rel(data.file), line: data.line }
}
const message = (data) => {
  const err = data.details?.error
  const text = String(err?.cause?.message ?? err?.message ?? err ?? 'failed')
  // Drop Playwright call logs and ANSI codes; keep the assertion message.
  return text.replace(/\x1b\[[0-9;]*m/g, '').replace(/\nCall log:[\s\S]*?(?=\n\[config:|$)/, '').slice(0, 1500)
}

const results = []
try {
  const stream = run({ files, concurrency: Number(process.env.MAILVEN_UI_CONCURRENCY) || 2, timeout: 180000 })
  stream.on('test:pass', (data) => {
    if (data.details?.type === 'suite') return
    results.push({ name: data.name, file: rel(data.file), line: data.line, status: data.skip !== undefined ? 'skipped' : data.todo !== undefined ? 'todo' : 'passed', durationMs: Math.round(data.details.duration_ms) })
  })
  stream.on('test:fail', (data) => {
    // A suite fails because a child failed; the child carries the diagnostic. File-level failures have nesting 0 and no type.
    if (data.details?.type === 'suite') return
    results.push({ name: data.name, ...failureLocation(data), status: 'failed', durationMs: Math.round(data.details.duration_ms), error: message(data) })
  })
  const junitOut = fs.createWriteStream(path.join(resultsDir, 'junit.xml'))
  stream.compose(junit).pipe(junitOut)
  await new Promise((resolve) => stream.compose(spec).on('end', resolve).pipe(process.stdout))
  await new Promise((resolve) => junitOut.on('close', resolve))
} finally {
  await vite.stop()
}

const count = (s) => results.filter((r) => r.status === s).length
const summary = { passed: count('passed'), failed: count('failed'), skipped: count('skipped'), todo: count('todo') }
fs.writeFileSync(path.join(resultsDir, 'results.json'), JSON.stringify({ summary, results }, null, 2) + '\n')

const failed = results.filter((r) => r.status === 'failed')
if (process.env.GITHUB_ACTIONS) {
  const esc = (s) => String(s).replace(/%/g, '%25').replace(/\r/g, '%0D').replace(/\n/g, '%0A')
  for (const f of failed.slice(0, 10)) {
    console.log(`::error file=${f.file},line=${f.line},title=${esc(f.name).replace(/[,:]/g, ' ')}::${esc(f.error)}`)
  }
  if (process.env.GITHUB_STEP_SUMMARY) {
    // First message line plus, for audit failures, the first finding (the first line only counts them).
    const cell = (s) => String(s).split('\n').slice(0, 2).map((l) => l.trim()).filter((l) => !l.startsWith('[config:'))
      .join(' ').replace(/\|/g, '\\|').slice(0, 300)
    const lines = [`### UI tests: ${summary.passed} passed, ${summary.failed} failed, ${summary.skipped} skipped`, '']
    if (failed.length) {
      lines.push('| Test | Location | Error |', '| --- | --- | --- |')
      for (const f of failed.slice(0, 30)) lines.push(`| ${cell(f.name)} | ${f.file}:${f.line} | ${cell(f.error)} |`)
      lines.push('', 'Screenshots and junit.xml are in the `ui-test-results` artifact.')
    }
    fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, lines.join('\n') + '\n')
  }
}
console.log(`\nUI tests: ${summary.passed} passed, ${summary.failed} failed, ${summary.skipped} skipped; results in ${path.relative(root, resultsDir)}/`)
for (const f of failed) console.log(`FAIL ${f.file}:${f.line} ${f.name}\n  ${f.error.split('\n').join('\n  ')}`)
process.exitCode = failed.length || !results.length ? 1 : 0
