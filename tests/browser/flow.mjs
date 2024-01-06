// Checks flow mode in headless Chrome against a pulse built from this
// checkout: the keys, the swipes, a failed write, the end of the flow, the
// reader, and the answers that stay in place for a short and a long card.
//
//   PLAYWRIGHT_CORE=/path/to/playwright-core/index.js node tests/browser/flow.mjs
//
// With SCREENSHOTS naming a directory, it saves pictures of the flow there.
//
// playwright-core is not a dependency of the repository, as for
// docs/scripts/og.mjs. The script builds pulse into a temporary directory,
// runs it on a free loopback port with a data directory of its own, whose
// snapshots are fresh so that nothing is loaded from the network, and stops
// it at the end. It exits 1 on the first check that fails.
import { execFileSync, spawn } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync, writeFileSync, mkdirSync } from 'node:fs'
import { createServer } from 'node:net'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const core = await import(process.env.PLAYWRIGHT_CORE || 'playwright-core')
const { chromium } = core.default ?? core

const root = fileURLToPath(new URL('../..', import.meta.url))
const work = mkdtempSync(join(tmpdir(), 'pulse-flow-'))
const data = join(work, 'data')
const config = join(work, 'config.toml')
writeFileSync(config, '')

// The cards of the flow, in the order of the page: in one list each, so the
// front page takes the first and the daily section the rest by rank.
const long = 'A long description that wraps over several lines of the card, '.repeat(6).trim()
const repos = [
  ['first', 'short', 'Short.'],
  ['second', 'long', long],
  ['third', 'later', 'Left for later.'],
  ['fourth', 'fails', 'Its write fails once.'],
  ['fifth', 'swiped', 'Swiped left.'],
  ['sixth', 'last', 'The last card.'],
]
const now = new Date()
const stamp = now.toISOString().replace(/:/g, '-')
const lists = [['trendshift', 'daily'], ['trendshift', 'weekly'], ['trendshift', 'monthly'],
  ['github', 'daily'], ['github', 'weekly'], ['github', 'monthly']]
const attempts = []
for (const [source, period] of lists) {
  const dir = join(data, 'snapshots', source, period)
  mkdirSync(dir, { recursive: true })
  const entries = source === 'trendshift' && period === 'daily'
    ? repos.map(([owner, name, description], i) => ({ rank: i + 1, repo: { owner, name }, description, language: 'Go' }))
    : [{ rank: 1, repo: { owner: 'other', name: `${source}-${period}` } }]
  const snapshot = { list: { source, period }, request: 'https://example.invalid/', marker: 'test', fetched_at: now.toISOString(), entries }
  writeFileSync(join(dir, `${stamp}.json`), JSON.stringify(snapshot))
  attempts.push(JSON.stringify({ time: now.toISOString(), list: `${source}/${period}`, ok: true }))
}
writeFileSync(join(data, 'attempts.jsonl'), attempts.join('\n') + '\n')
// The other lists' repositories are seen, so the flow holds only the six.
const marks = lists.filter(([s, p]) => !(s === 'trendshift' && p === 'daily'))
  .map(([source, period]) => JSON.stringify({ time: now.toISOString(), repo: { owner: 'other', name: `${source}-${period}` }, action: 'seen', lists: [] }))
writeFileSync(join(data, 'marks.jsonl'), marks.join('\n') + '\n')

const port = await new Promise((resolve) => {
  const probe = createServer().listen(0, '127.0.0.1', () => {
    const { port } = probe.address()
    probe.close(() => resolve(port))
  })
})
const binary = join(work, 'pulse')
execFileSync('go', ['build', '-o', binary, './cmd/pulse'], { cwd: root, stdio: 'inherit' })
const service = spawn(binary, ['--listen', `127.0.0.1:${port}`, '--data-dir', data, '--config', config], { stdio: ['ignore', 'pipe', 'inherit'] })
await new Promise((resolve, reject) => {
  service.stdout.on('data', (chunk) => { if (String(chunk).includes('pulse listens on')) resolve() })
  service.on('exit', (code) => reject(new Error(`pulse exited with ${code}`)))
})
const origin = `http://127.0.0.1:${port}`

let failed = false
function check (ok, what) {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${what}`)
  if (!ok) failed = true
}
function markLines () {
  return readFileSync(join(data, 'marks.jsonl'), 'utf8').trim().split('\n').map((line) => JSON.parse(line)).slice(marks.length)
}

let browser
try {
  browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({ viewport: { width: 1100, height: 800 }, hasTouch: true })
  const page = await context.newPage()
  await page.goto(`${origin}/flow/`)

  const visible = () => page.locator('.flow-card:not([hidden])').getAttribute('data-repo')
  const boxes = async () => Promise.all(['skip', 'later', 'interesting'].map(async (answer) => {
    const box = await page.locator(`[data-answer="${answer}"]`).boundingBox()
    return `${Math.round(box.x)},${Math.round(box.y)},${Math.round(box.width)}`
  }))

  check(await visible() === 'first/short', 'the flow starts with the first card of the page')
  check((await page.locator('#flow-count').textContent()).replace(/\s+/g, ' ').includes('1 of 6 · 6 left'), 'it says the place and how many are left')
  if (process.env.SCREENSHOTS) await page.screenshot({ path: join(process.env.SCREENSHOTS, 'flow-short.png') })
  const short = await boxes()

  await page.keyboard.press('ArrowLeft')
  await page.waitForFunction(() => document.querySelector('.flow-card:not([hidden])')?.dataset.repo === 'second/long')
  check(markLines().at(-1)?.action === 'skip' && markLines().at(-1)?.repo.name === 'short', 'left marks the repository as skipped')
  check(JSON.stringify(await boxes()) === JSON.stringify(short), 'the answers stay in place for a long description')
  if (process.env.SCREENSHOTS) await page.screenshot({ path: join(process.env.SCREENSHOTS, 'flow-long.png') })

  await page.fill('[name="comment"]', 'read it later')
  await page.keyboard.press('Escape')
  await page.keyboard.press('ArrowRight')
  await page.waitForFunction(() => document.querySelector('.flow-card:not([hidden])')?.dataset.repo === 'third/later')
  const interesting = markLines().at(-1)
  check(interesting?.action === 'interesting' && interesting?.comment === 'read it later', 'right marks it as interesting with the comment')
  check(await page.inputValue('[name="comment"]') === '', 'the comment field is empty for the next card')

  const before = markLines().length
  // A held key repeats: only the first press answers.
  await page.keyboard.down('ArrowDown')
  await page.keyboard.down('ArrowDown')
  await page.keyboard.down('ArrowDown')
  await page.keyboard.up('ArrowDown')
  check(await visible() === 'fourth/fails' && markLines().length === before, 'down marks nothing and shows the next card, once for a held key')

  await page.route('**/api/marks', (route) => route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"the disk is full"}' }))
  await page.keyboard.press('ArrowLeft')
  await page.locator('#flow-error:not([hidden])').waitFor()
  check(await visible() === 'fourth/fails', 'a failed write keeps the card on the screen')
  check((await page.locator('#flow-error').innerText()).includes('the disk is full'), 'and shows the error')
  await page.unroute('**/api/marks')
  await page.keyboard.press('ArrowLeft')
  await page.waitForFunction(() => document.querySelector('.flow-card:not([hidden])')?.dataset.repo === 'fifth/swiped')

  await page.locator('.flow-card:not([hidden]) [data-action="read"]').first().click()
  await page.locator('#reader:not([hidden]) .story').waitFor()
  check(await page.locator('#reader .story').getAttribute('data-repo') === 'fifth/swiped', 'the reader opens from the card')
  await page.keyboard.press('Escape')
  check(await page.locator('#reader').isHidden() && await visible() === 'fifth/swiped', 'and returns to the same card')

  await page.evaluate(() => {
    const stage = document.getElementById('flow-stage')
    const at = (type, x) => stage.dispatchEvent(new PointerEvent(type, { bubbles: true, pointerType: 'touch', clientX: x, clientY: 300 }))
    at('pointerdown', 600)
    at('pointerup', 400)
  })
  await page.waitForFunction(() => document.querySelector('.flow-card:not([hidden])')?.dataset.repo === 'sixth/last')
  check(markLines().at(-1)?.repo.name === 'swiped' && markLines().at(-1)?.action === 'skip', 'a swipe to the left skips')

  await page.evaluate(() => {
    const stage = document.getElementById('flow-stage')
    const at = (type, x) => stage.dispatchEvent(new PointerEvent(type, { bubbles: true, pointerType: 'touch', clientX: x, clientY: 300 }))
    at('pointerdown', 300)
    at('pointerup', 520)
  })
  await page.locator('#flow-end:not([hidden])').waitFor()
  check(markLines().at(-1)?.repo.name === 'last' && markLines().at(-1)?.action === 'interesting', 'a swipe to the right is interesting')
  check(await page.locator('[data-flow="page"]').getAttribute('href') === '/', 'the last answer shows the end with a link to the page')

  await page.goto(`${origin}/flow/`)
  check(await visible() === 'third/later' && await page.locator('.flow-card').count() === 1, 'a new flow holds the card left for later')

  await page.setViewportSize({ width: 390, height: 760 })
  await page.goto(`${origin}/flow/`)
  const narrow = await boxes()
  if (process.env.SCREENSHOTS) await page.screenshot({ path: join(process.env.SCREENSHOTS, 'flow-phone.png') })
  await page.keyboard.press('ArrowDown')
  check(await page.locator('#flow-end').isVisible(), 'on a phone too, down goes to the end')
  check(narrow.every((box) => Number(box.split(',')[1]) > 600), 'the answers sit at the bottom of a phone screen')
} finally {
  await browser?.close()
  service.kill('SIGTERM')
  rmSync(work, { recursive: true, force: true })
}
process.exit(failed ? 1 : 0)
