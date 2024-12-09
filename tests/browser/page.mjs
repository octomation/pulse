// Checks the grid of the main page in headless Chrome against a pulse built
// from this checkout: one grid for every section at three widths, the parts
// of a row on one line, cards without answers, and the reader that answers
// them from the keyboard.
//
//   PLAYWRIGHT_CORE=/path/to/playwright-core/index.js node tests/browser/page.mjs
//
// With SCREENSHOTS naming a directory, it saves pictures of the page there.
//
// As tests/browser/flow.mjs: playwright-core is not a dependency, pulse runs
// on a free loopback port with a data directory of its own whose snapshots
// are fresh, and the script exits 1 on the first check that fails. It needs
// Node 21 or later (Object.groupBy).
import { execFileSync, spawn } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync, writeFileSync, mkdirSync } from 'node:fs'
import { createServer } from 'node:net'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const core = await import(process.env.PLAYWRIGHT_CORE || 'playwright-core')
const { chromium } = core.default ?? core

const root = fileURLToPath(new URL('../..', import.meta.url))
const work = mkdtempSync(join(tmpdir(), 'pulse-page-'))
const data = join(work, 'data')
const config = join(work, 'config.toml')
writeFileSync(config, '')

const long = 'A long description that wraps over many lines of the card, '.repeat(8).trim()
const repo = (owner, name, description = 'Short.') => ({ owner, name, description })
// Three repositories in two lists make the front page: the lead and two
// beside it. The daily section holds five, the weekly one.
const front = [repo('lead', 'story', long), repo('second', 'story'), repo('third', 'story', long)]
const daily = [repo('day', 'one', long), repo('day', 'two'), repo('day', 'three'), repo('day', 'four', long), repo('day', 'five')]
const weekly = [repo('week', 'one')]
const content = {
  'trendshift/daily': [...front, ...daily],
  'github/weekly': [...front, ...weekly],
  'trendshift/weekly': [], 'trendshift/monthly': [], 'github/daily': [], 'github/monthly': [],
}
const now = new Date()
const stamp = now.toISOString().replace(/:/g, '-')
const attempts = []
const seen = []
for (const [key, repos] of Object.entries(content)) {
  const [source, period] = key.split('/')
  const dir = join(data, 'snapshots', source, period)
  mkdirSync(dir, { recursive: true })
  // A list cannot be empty: its filler is marked as seen.
  const all = repos.length ? repos : [repo('filler', `${source}-${period}`)]
  if (!repos.length) seen.push(all[0])
  const entries = all.map((r, i) => ({ rank: i + 1, repo: { owner: r.owner, name: r.name }, description: r.description, language: 'Go' }))
  writeFileSync(join(dir, `${stamp}.json`), JSON.stringify({ list: { source, period }, request: 'https://example.invalid/', marker: 'test', fetched_at: now.toISOString(), entries }))
  attempts.push(JSON.stringify({ time: now.toISOString(), list: key, ok: true }))
}
// Seven past days hold every count the grid must fill: a front page of one
// to seven cards, and sections of 1, 2, 4 and 50 cards.
const sizes = [1, 2, 4, 50]
const pastDays = []
for (let d = 1; d <= 7; d++) {
  const at = new Date(now.getTime() - d * 24 * 3600 * 1000)
  const day = `${at.getFullYear()}-${String(at.getMonth() + 1).padStart(2, '0')}-${String(at.getDate()).padStart(2, '0')}`
  pastDays.push({ day, front: d })
  const both = Array.from({ length: d }, (_, i) => repo(`d${d}`, `front-${i}`, i % 2 ? long : 'Short.'))
  const daily = Array.from({ length: sizes[(d - 1) % 4] }, (_, i) => repo(`d${d}`, `daily-${i}`, i % 3 ? 'Short.' : long))
  const weekly = Array.from({ length: sizes[(d + 1) % 4] }, (_, i) => repo(`d${d}`, `weekly-${i}`, i % 2 ? long : 'Short.'))
  const lists = { 'trendshift/daily': both, 'github/daily': daily, 'github/weekly': [...both, ...weekly] }
  const dayStamp = at.toISOString().replace(/:/g, '-')
  for (const key of Object.keys(content)) {
    const [source, period] = key.split('/')
    const all = lists[key]?.length ? lists[key] : [repo('filler', `${source}-${period}`)]
    const entries = all.map((r, i) => ({ rank: i + 1, repo: { owner: r.owner, name: r.name }, description: r.description, language: 'Go' }))
    writeFileSync(join(data, 'snapshots', source, period, `${dayStamp}.json`), JSON.stringify({ list: { source, period }, request: 'https://example.invalid/', marker: 'test', fetched_at: at.toISOString(), entries }))
  }
}
writeFileSync(join(data, 'attempts.jsonl'), attempts.join('\n') + '\n')
const marks = seen.map((r) => JSON.stringify({ time: now.toISOString(), repo: { owner: r.owner, name: r.name }, action: 'seen', lists: [] }))
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

// layout returns, for every card of the page, its section, its left edge and
// the tops of its parts.
function layout (page) {
  return page.evaluate(() => Array.from(document.querySelectorAll('#shown .card')).map((card, _, all) => {
    const top = (selector) => Math.round(card.querySelector(selector).getBoundingClientRect().top)
    const box = card.getBoundingClientRect()
    return {
      key: card.dataset.repo,
      index: Array.prototype.indexOf.call(card.parentNode.children, card),
      lead: card.classList.contains('is-lead'),
      right: Math.round(box.right),
      top: Math.round(box.top),
      bottom: Math.round(box.bottom),
      section: card.closest('.section').dataset.section,
      left: Math.round(box.left),
      width: Math.round(box.width),
      kicker: top('.kicker'),
      name: top('.card-name'),
      description: top('.description'),
      badges: top('.badges'),
      lines: Math.round(card.querySelector('.description').getBoundingClientRect().height / parseFloat(getComputedStyle(card.querySelector('.description')).lineHeight)),
      controls: card.querySelectorAll('button, input, form').length,
    }
  }))
}

let browser
try {
  browser = await chromium.launch({ headless: true })
  const page = await browser.newPage({ viewport: { width: 1200, height: 900 } })

  // grid checks a page at a width: the cards stand on the same columns in
  // every section, the rows are full in the order of the cards and their
  // parts are on one line, no card overlaps another, and no card holds an
  // answer.
  // grid checks a page at a width against the cards it must hold: the
  // count of every section and the lead first; the cards beside the lead in
  // the third column; full rows of the column count whose cards stand side
  // by side with their parts on one line; one set of columns for every
  // section; no overlap; no answers on a card; the clamps.
  async function grid (path, width, columns, what, expect) {
    await page.setViewportSize({ width, height: 900 })
    await page.goto(`${origin}${path}`)
    const cards = await layout(page)
    const ok = []
    const count = (section) => cards.filter((card) => card.section === section).length
    ok.push([Object.entries(expect).every(([section, n]) => count(section) === n) && cards.length === Object.values(expect).reduce((a, b) => a + b, 0),
      `${expect.front} on the front page, ${expect.daily} daily, ${expect.weekly} weekly`])
    const front = cards.filter((card) => card.section === 'front')
    const lead = front[0]
    ok.push([lead?.lead === true && front.slice(1).every((card) => !card.lead), 'the lead first'])
    const plain = cards.filter((card) => !card.lead)
    const edges = [...new Set(plain.map((card) => card.left))].sort((a, b) => a - b)
    ok.push([edges.length <= columns, 'the same columns in every section'])
    const near = (a, b) => Math.abs(a - b) <= 1
    if (columns === 3 && front.length > 1) {
      const beside = front.slice(1, 3)
      const after = front.slice(3)
      const third = Math.max(...beside.map((card) => card.left))
      ok.push([beside.every((card) => card.left > lead.right) && near(beside[0].top, lead.top) &&
        (beside.length < 2 || (near(beside[1].left, beside[0].left) && beside[1].top >= beside[0].bottom - 1)) &&
        (front.length === 2 || near(third, edges.at(-1))),
      'the next two beside the lead, one above the other'])
      const floor = Math.max(lead.bottom, ...beside.map((card) => card.bottom))
      ok.push([after.every((card) => card.top >= floor - 1), 'the rows of the front page start below the lead'])
    } else if (front.length > 1) {
      ok.push([front.slice(1).every((card) => card.top >= lead.bottom - 1), 'the lead takes its own row'])
    }
    // Rows of a section: the cards of a row stand side by side at one top,
    // and only the last row of a section may be short.
    const rows = []
    for (const section of ['daily', 'weekly']) {
      const list = cards.filter((card) => card.section === section)
      for (let i = 0; i < list.length; i += columns) rows.push(list.slice(i, i + columns))
    }
    if (columns === 3) {
      const after = front.slice(3)
      for (let i = 0; i < after.length; i += 3) rows.push(after.slice(i, i + 3))
    } else {
      const after = front.slice(1)
      for (let i = 0; i < after.length; i += columns) rows.push(after.slice(i, i + columns))
    }
    ok.push([rows.every((row) => new Set(row.map((card) => card.left)).size === row.length && ['top', 'kicker', 'name', 'description', 'badges'].every((part) => new Set(row.map((card) => card[part])).size === 1)),
      'full rows side by side, their parts on one line'])
    const overlap = cards.some((a, i) => cards.some((b, j) => i < j && a.left < b.right - 1 && b.left < a.right - 1 && a.top < b.bottom - 1 && b.top < a.bottom - 1))
    ok.push([!overlap, 'no card overlaps another'])
    ok.push([cards.every((card) => card.controls === 0), 'no answers on a card'])
    ok.push([plain.every((card) => card.lines <= 3) && (!lead || lead.lines <= 5), 'three lines of a description, five on the lead'])
    const bad = ok.filter(([pass]) => !pass).map(([, name]) => name)
    check(bad.length === 0, `${what} at ${width}px${bad.length ? ': ' + bad.join(', ') : ''}`)
    return cards
  }

  for (const [width, columns] of [[1200, 3], [900, 2], [390, 1]]) {
    const cards = await grid('/', width, columns, 'today', { front: 3, daily: 5, weekly: 1 })
    if (process.env.SCREENSHOTS) await page.screenshot({ path: join(process.env.SCREENSHOTS, `page-${width}.png`), fullPage: true })
    const lead = cards.find((card) => card.key === 'lead/story')
    const plain = cards.filter((card) => !card.lead)
    check(width === 1200 ? lead.width > plain[0].width * 1.8 : lead.width >= plain[0].width, `${width}px: the lead is wider than a card`)
    for (const { day, front } of pastDays) {
      const expect = { front, daily: sizes[(front - 1) % 4], weekly: sizes[(front + 1) % 4] }
      await grid(`/days/${day}/`, width, columns, `a front page of ${front}, sections of ${expect.daily} and ${expect.weekly}`, expect)
    }
  }

  await page.setViewportSize({ width: 1200, height: 900 })
  await page.goto(`${origin}/`)
  const order = await page.evaluate(() => Array.from(document.querySelectorAll('#shown .card')).map((card) => card.dataset.repo))
  check(JSON.stringify(order) === JSON.stringify(['lead/story', 'second/story', 'third/story', 'day/one', 'day/two', 'day/three', 'day/four', 'day/five', 'week/one']),
    'the cards are in the order of the ranks')

  await page.locator('#shown .card [data-action="read"]').first().focus()
  await page.keyboard.press('Enter')
  await page.locator('#reader:not([hidden]) .story').waitFor()
  const story = () => page.locator('#reader .story').getAttribute('data-repo')
  check(await story() === 'lead/story', 'Enter on a name opens the reader')
  await page.keyboard.press('j')
  await page.waitForFunction(() => document.querySelector('#reader .story')?.dataset.repo === 'second/story')
  await page.keyboard.press('j')
  await page.waitForFunction(() => document.querySelector('#reader .story')?.dataset.repo === 'third/story')
  check(true, 'j walks the cards in the order of the page')

  await page.fill('#reader [name="comment"]', 'from the reader')
  await page.locator('#reader [data-action="interesting"]').click()
  await page.waitForFunction(() => !document.querySelector('#shown [data-repo="third/story"]'))
  check(markLines().at(-1)?.action === 'interesting' && markLines().at(-1)?.comment === 'from the reader', 'the reader answers Interesting with the comment')
  await page.waitForFunction(() => document.querySelector('#reader .story')?.dataset.repo !== 'third/story')
  check(await story() === 'day/one', 'and moves to the next card')

  await page.route('**/api/marks', (route) => route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"the disk is full"}' }))
  await page.locator('#reader [data-action="skip"]').click()
  await page.locator('#reader .card-error:not([hidden])').waitFor()
  check((await page.locator('#reader .card-error').innerText()).includes('the disk is full') &&
    await page.locator('#shown [data-repo="day/one"]').count() === 1, 'a failed answer keeps the card and the reader shows the error')
  await page.unroute('**/api/marks')
  await page.keyboard.press('Escape')

  const left = await page.locator('#shown .card').count()
  const before = markLines().length
  await page.locator('[data-action="seen-all"]').click()
  await page.waitForFunction(() => !document.querySelector('#shown .card'))
  const written = markLines().slice(before)
  check(left === 8 && written.length === left && written.every((line) => line.action === 'seen'), 'Mark all shown as seen clears the page')

  check(await page.locator('#notice [data-action="undo-seen"]').isVisible(), 'and offers Undo')
  await page.locator('#notice [data-action="undo-seen"]').click()
  await page.waitForFunction((count) => document.querySelectorAll('#shown .card').length === count, left)
  check(markLines().slice(before + left).length === left && markLines().slice(before + left).every((line) => line.action === 'clear'),
    'Undo takes back exactly the marks it wrote')
  await page.locator('[data-action="seen-all"]').click()
  await page.waitForFunction(() => !document.querySelector('#shown .card'))

  // In the hidden view an answer moves the card to its group, and the
  // reader stays on it with its new answer.
  await page.goto(`${origin}/hidden/`)
  check(await page.locator('[data-group="skipped"] [data-repo="week/one"] [data-stamp="seen"]').count() === 1,
    'the hidden view groups the cards under their stamps')
  await page.locator('#hidden [data-repo="week/one"] [data-action="read"]').first().click()
  await page.locator('#reader:not([hidden]) .story[data-repo="week/one"]').waitFor()
  await page.fill('#reader [name="comment"]', 'kept from the hidden view')
  await page.locator('#reader [data-action="interesting"]').click()
  await page.locator('#reader:not([hidden]) .story[data-repo="week/one"] [data-stamp="interesting"]').waitFor()
  const moved = page.locator('[data-group="interesting"] [data-repo="week/one"]')
  check(await moved.count() === 1 && (await moved.textContent()).includes('kept from the hidden view') &&
    await moved.locator('button, input').count() === 0,
  'an answer in the hidden view moves the card to its group, still without answers, and the reader stays on it')
  await page.locator('#reader [data-action="clear"]').click()
  await page.locator('#shown [data-repo="week/one"]').waitFor()
  check(markLines().at(-1)?.action === 'clear' && markLines().at(-1)?.repo.name === 'one', 'Take back makes it new again')

  // The queue lists the interest without a note, and takes it back in one
  // click.
  await page.goto(`${origin}/queue/`)
  const queued = page.locator('.queue-item[data-repo="third/story"]')
  check(await queued.count() === 1 && (await queued.textContent()).includes('from the reader'), 'the queue lists a bookmark with its comment')
  await queued.locator('[data-action="clear"]').click()
  await page.waitForFunction(() => !document.querySelector('.queue-item[data-repo="third/story"]'))
  check(markLines().at(-1)?.action === 'clear' && markLines().at(-1)?.repo.name === 'story', 'Take back in the queue removes it')

  // Answering the lead of a past day of five leaves four on its front page,
  // the first three a plain row: side by side, at one top, without a rule.
  const five = pastDays.find((past) => past.front === 5)
  await page.setViewportSize({ width: 1200, height: 900 })
  await page.goto(`${origin}/days/${five.day}/`)
  await page.locator('#shown .is-front .card.is-lead [data-action="read"]').first().click()
  await page.locator('#reader:not([hidden]) .story[data-repo="d5/front-0"]').waitFor()
  await page.locator('#reader [data-action="skip"]').click()
  await page.waitForFunction(() => !document.querySelector('#shown [data-repo="d5/front-0"]'))
  await page.keyboard.press('Escape')
  const rest = await page.evaluate(() => Array.from(document.querySelectorAll('.is-front .card')).map((card) => {
    const box = card.getBoundingClientRect()
    return [Math.round(box.top), Math.round(box.left), getComputedStyle(card).borderTopWidth, card.classList.contains('is-lead')]
  }))
  const row = rest.slice(0, 3)
  check(rest.length === 4 && rest.every(([, , , lead]) => !lead) && new Set(row.map(([top]) => top)).size === 1 &&
    new Set(row.map(([, left]) => left)).size === 3 && row.every(([, , border]) => border === '0px'),
  'without the lead the front page starts with a plain row')
} finally {
  await browser?.close()
  service.kill('SIGTERM')
  rmSync(work, { recursive: true, force: true })
}
process.exit(failed ? 1 : 0)
