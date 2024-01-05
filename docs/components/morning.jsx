'use client'

// The first screen of the landing page: an isometric desk built from CSS 3D
// boxes, and a morning that plays on it as the page scrolls. The script only
// measures the scroll and the pointer and writes them into custom properties
// (--p1…--p5, --day, --tx, --ty, --s); every movement is CSS in landing.css.
import Link from 'next/link'
import { useEffect, useRef, useState } from 'react'

const STEPS = [
  { time: '07:30', title: 'Pour a coffee.', text: 'pulse is already running in the background. Six trending lists are in the air, repeating each other and yesterday.' },
  { time: '07:32', title: 'Open the paper.', text: 'The six lists fold into one front page. A repository that is in four of them is one card, and what you have seen stays out.' },
  { time: '07:35', title: 'Read a story.', text: 'Click a card and it rises beside the paper: the stars, the topics and the README, in the same window.' },
  { time: '07:40', title: 'Keep what matters.', text: 'One click on Interesting, a note in your own words, and it is a line in your notebook.' },
  { time: '07:41', title: 'Close the tab.', text: 'Everything else is marked as seen. The paper is empty until tomorrow, and the coffee is still warm.' },
]

// Where each list floats before it lands on the desk: an offset from the
// paper, a height above the desk and a turn.
const SHEETS = [
  { label: 'trendshift · daily', dx: -210, dy: -130, dz: 190, r: -16, chips: 'a-b-' },
  { label: 'trendshift · weekly', dx: -40, dy: -250, dz: 130, r: 9, chips: '-a-n' },
  { label: 'trendshift · monthly', dx: 150, dy: -190, dz: 235, r: 22, chips: '--an' },
  { label: 'github · daily', dx: -230, dy: 90, dz: 105, r: 13, chips: 'b-c-' },
  { label: 'github · weekly', dx: 40, dy: 10, dz: 165, r: -7, chips: '-c-n' },
  { label: 'github · monthly', dx: 215, dy: 120, dz: 80, r: -21, chips: 'a-n-' },
]

const CARDS = ['GO · 2 LISTS', 'RUST · 3 LISTS', 'PYTHON', 'ZIG · 2 LISTS', 'TYPESCRIPT', 'C · 2 LISTS']

const clamp = v => Math.min(1, Math.max(0, v))
const ease = v => v * v * (3 - 2 * v)

function Box({ className = '', x, y, z = 0, w, d, h, front, children }) {
  return (
    <div className={`iso-box ${className}`} style={{ '--x': `${x}px`, '--y': `${y}px`, '--z': `${z}px`, '--h': `${h}px`, width: w, height: d }}>
      <i className="iso-left" />
      <div className="iso-front">{front}</div>
      <div className="iso-top">{children}</div>
    </div>
  )
}

function Disc({ r, z, className = '' }) {
  return <i className={`iso-disc ${className}`} style={{ '--r': `${r}px`, '--z': `${z}px` }} />
}

function Cup() {
  const wall = []
  for (let i = 0; i < 36; i++) wall.push(<Disc key={i} className="is-wall" r={25 + i * 0.22 + Math.sin(i / 11) * 2.2} z={7 + i} />)
  return (
    <div className="iso-cup">
      <i className="iso-shadow is-round" style={{ '--w': '150px', '--d': '150px' }} />
      <Disc className="is-saucer" r={56} z={1} />
      <Disc className="is-saucer" r={58} z={2.5} />
      <Disc className="is-saucer is-rim" r={59} z={4} />
      <Disc className="is-base" r={24} z={5} />
      <Disc className="is-base" r={25} z={6} />
      {wall}
      <Disc className="is-coffee" r={28} z={0} />
      <i className="iso-handle" />
      <div className="iso-steam" aria-hidden="true"><i /><i /><i /></div>
    </div>
  )
}

function Glasses() {
  return (
    <div className="iso-glasses">
      <i className="iso-shadow" style={{ '--w': '140px', '--d': '90px', '--sx': '-14px', '--sy': '-64px' }} />
      <i className="iso-lens" style={{ '--x': '0px' }} />
      <i className="iso-lens" style={{ '--x': '60px' }} />
      <i className="iso-bridge" />
      <Box className="is-temple" x={-2} y={-78} z={30} w={4} d={78} h={4} />
      <Box className="is-temple" x={110} y={-78} z={30} w={4} d={78} h={4} />
    </div>
  )
}

function Notebook() {
  const rings = []
  for (let i = 0; i < 9; i++) rings.push(<i key={i} className="iso-ring" style={{ '--y': `${16 + i * 20}px` }} />)
  return (
    <div className="iso-notebook">
      <i className="iso-shadow" style={{ '--w': '176px', '--d': '214px', '--sx': '-12px', '--sy': '4px' }} />
      <Box className="is-cover" x={0} y={0} w={156} d={196} h={3} />
      <Box className="is-pages" x={3} y={3} z={3} w={150} d={190} h={9}>
        <div className="iso-page">
          <span className="is-date">Fri, Nov 20</span>
          <span className="is-old">uv — for the monorepo</span>
          <span className="is-old">ghostty — its configs</span>
          <span className="is-new">LoopForge — for the CI</span>
          <span className="is-new is-second">review of the agents</span>
        </div>
      </Box>
      {rings}
      <i className="iso-ribbon" />
    </div>
  )
}

function Paper() {
  return (
    <Box className="is-paper" x={-160} y={-196} w={260} d={384} h={6}>
      <div className="iso-news">
        <b className="iso-masthead">The Pulse</b>
        <p className="iso-dateline"><span>FRIDAY, NOVEMBER 20</span><span>7 NEW · 31 SEEN</span></p>
        <div className="iso-lead">
          <small>FRONT PAGE · IN 4 LISTS</small>
          <b>owner/first</b>
          <i /><i /><i />
        </div>
        <div className="iso-cards">
          {CARDS.map((kicker, i) => (
            <div key={kicker} className={i === 0 ? 'is-kept' : ''} style={{ '--i': i }}>
              <small>{kicker}</small><i /><i /><i />
            </div>
          ))}
        </div>
        <div className="iso-stamp"><b>Until tomorrow.</b><span>ALL MARKED AS SEEN</span></div>
      </div>
    </Box>
  )
}

function Scene({ step }) {
  return (
    <div className="iso-stage" aria-hidden="true">
      <div className="iso-glow" />
      <div className="iso-world">
        <i className="iso-floor" />
        <Box className="is-desk" x={-330} y={-250} z={-22} w={660} d={500} h={22}>
          <i className="iso-light" />
        </Box>

        <Paper />
        <i className="iso-shadow is-paper-shadow" style={{ '--w': '280px', '--d': '400px', '--sx': '-178px', '--sy': '-196px' }} />

        {SHEETS.map((sheet, i) => (
          <i key={`s${sheet.label}`} className="iso-sheet-shadow" style={{ '--i': i, '--dx': `${sheet.dx}px`, '--dy': `${sheet.dy}px`, '--r': `${sheet.r}deg` }} />
        ))}
        {SHEETS.map((sheet, i) => (
          <div key={sheet.label} className="iso-sheet" style={{ '--i': i, '--dx': `${sheet.dx}px`, '--dy': `${sheet.dy}px`, '--dz': `${sheet.dz}px`, '--r': `${sheet.r}deg` }}>
            <div className="iso-sheet-in">
              <small>{sheet.label}</small>
              {[0, 1, 2, 3, 4, 5].map(row => {
                const kind = sheet.chips[(row + i) % sheet.chips.length]
                return <i key={row} className={kind === '-' ? 'is-seen' : kind === 'n' ? '' : `is-${kind}`} />
              })}
            </div>
          </div>
        ))}

        <Cup />
        <Notebook />
        <Glasses />

        <div className="iso-pencil">
          <i className="iso-shadow" style={{ '--w': '150px', '--d': '20px', '--sx': '-12px', '--sy': '0px' }} />
          <Box className="is-eraser" x={0} y={0} w={12} d={8} h={8} />
          <Box className="is-wood" x={12} y={0} w={112} d={8} h={8} />
          <i className="iso-tip" />
        </div>

        <Box className="is-clock" x={-312} y={168} w={92} d={34} h={50} front={<span>{step ? STEPS[step - 1].time : '07:29'}</span>} />
        <i className="iso-shadow" style={{ '--w': '112px', '--d': '48px', '--sx': '-326px', '--sy': '170px' }} />

        <div className="iso-reader">
          <small>GO · IN 2 LISTS</small>
          <b>Tencent/LoopForge</b>
          <em>Self-hosted workflows for coding-agent delivery.</em>
          <p><span>5,109<i>STARS</i></span><span>708<i>FORKS</i></span><span>MIT<i>LICENSE</i></span></p>
          <div><u>Interesting</u><s>Skip</s></div>
        </div>
      </div>
    </div>
  )
}

export function Morning() {
  const root = useRef(null)
  const [step, setStep] = useState(0)

  useEffect(() => {
    const node = root.current
    const stage = node.querySelector('.iso-stage')
    const panels = [...node.querySelectorAll('[data-step]')]
    const still = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const target = panels.map(() => 0)
    const now = panels.map(() => 0)
    const tilt = { x: 0, y: 0, tx: 0, ty: 0 }
    let frame = 0
    let shown = -1

    const draw = () => {
      frame = 0
      let moving = false
      let day = 0
      now.forEach((value, i) => {
        const gap = target[i] - value
        if (still || Math.abs(gap) < 0.001) now[i] = target[i]
        else { now[i] = value + gap * 0.14; moving = true }
        day += now[i]
        node.style.setProperty(`--p${i + 1}`, ease(now[i]).toFixed(4))
      })
      node.style.setProperty('--day', (day / now.length).toFixed(4))
      for (const axis of ['x', 'y']) {
        const gap = tilt[`t${axis}`] - tilt[axis]
        if (Math.abs(gap) < 0.002) tilt[axis] = tilt[`t${axis}`]
        else { tilt[axis] += gap * 0.08; moving = true }
        node.style.setProperty(`--t${axis}`, tilt[axis].toFixed(3))
      }
      const active = now.filter(value => value > 0.5).length
      if (active !== shown) { shown = active; setStep(active) }
      if (moving) frame = requestAnimationFrame(draw)
    }
    const schedule = () => { if (!frame) frame = requestAnimationFrame(draw) }

    const measure = () => {
      const view = window.innerHeight
      panels.forEach((panel, i) => {
        target[i] = clamp((view * 0.82 - panel.getBoundingClientRect().top) / (view * 0.5))
      })
      schedule()
    }
    const point = event => {
      if (still) return
      tilt.tx = (event.clientX / window.innerWidth - 0.5) * 2
      tilt.ty = (event.clientY / window.innerHeight - 0.5) * 2
      schedule()
    }
    const fit = () => {
      node.style.setProperty('--s', Math.min(1.25, stage.clientWidth / 800, stage.clientHeight / 640).toFixed(4))
      measure()
    }

    const sizes = new ResizeObserver(fit)
    sizes.observe(stage)
    window.addEventListener('scroll', measure, { passive: true })
    window.addEventListener('pointermove', point, { passive: true })
    fit()
    node.classList.add('is-live')
    return () => {
      sizes.disconnect()
      window.removeEventListener('scroll', measure)
      window.removeEventListener('pointermove', point)
      cancelAnimationFrame(frame)
    }
  }, [])

  return (
    <section className="morning" ref={root} data-s={step}>
      <div className="morning-hero">
        <Link className="release-pill" href="/changelog/v0.1.1/">Latest release · v0.1.1 <span aria-hidden="true">↗</span></Link>
        <h1>Your morning, with a hand on <em>the&nbsp;pulse.</em></h1>
        <p className="pulse-lead">Six trending lists repeat each other and yesterday. pulse folds them into one quiet paper that holds only what you have not seen. Read it with your coffee, keep what matters in one click, and close the tab knowing you missed nothing.</p>
        <div className="pulse-actions">
          <a className="button-primary" href="#get-pulse">Get pulse <span aria-hidden="true">↓</span></a>
          <Link className="button-text" href="/trend-watch/">Read the guide <span aria-hidden="true">→</span></Link>
        </div>
        <p className="hero-note">v0.1.1 · macOS & Linux · runs on your computer · free and open source</p>
        <p className="morning-hint" aria-hidden="true"><i />Scroll to play the morning</p>
      </div>

      <div className="morning-view">
        <Scene step={step} />
        <ol className="morning-rail" aria-hidden="true">
          {STEPS.map((item, i) => <li key={item.time} className={step === i + 1 ? 'is-now' : step > i + 1 ? 'is-done' : ''}>{item.time}</li>)}
        </ol>
      </div>

      {STEPS.map((item, i) => (
        <article key={item.time} className={`morning-step${step === i + 1 ? ' is-now' : ''}`} data-step={i + 1}>
          <time>{item.time}</time>
          <h2>{item.title}</h2>
          <p>{item.text}</p>
        </article>
      ))}
    </section>
  )
}
