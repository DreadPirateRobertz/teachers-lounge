#!/usr/bin/env node
/**
 * gen-testing-doc.mjs — Living testing doc generator.
 *
 * Reads Playwright JSON test results from e2e/test-results/results.json
 * and screenshots from e2e/test-results/screenshots/, then regenerates
 * docs/testing-master.html with real pass/fail status and screenshots.
 *
 * Usage:
 *   node scripts/gen-testing-doc.mjs [--results <path>] [--screenshots <dir>]
 *
 * Options:
 *   --results     Path to Playwright JSON results (default: e2e/test-results/results.json)
 *   --screenshots Dir with screenshots sub-dirs per feature (default: e2e/test-results/screenshots)
 *   --out         Output HTML path (default: docs/testing-master.html)
 */

import { readFileSync, writeFileSync, existsSync, readdirSync } from 'fs'
import { join, resolve, relative, extname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))
const repoRoot = resolve(__dirname, '..')

// ── CLI arg parsing ───────────────────────────────────────────────────────────

const args = process.argv.slice(2)
const getArg = (flag) => {
  const idx = args.indexOf(flag)
  return idx !== -1 ? args[idx + 1] : null
}

const RESULTS_PATH = getArg('--results') ?? join(repoRoot, 'e2e/test-results/results.json')
const SCREENSHOT_BASE = getArg('--screenshots') ?? join(repoRoot, 'e2e/test-results/screenshots')
const OUT_PATH = getArg('--out') ?? join(repoRoot, 'docs/testing-master.html')
const DOCS_DIR = join(repoRoot, 'docs')

// ── Load test results ─────────────────────────────────────────────────────────

/**
 * @typedef {{ title: string; status: 'passed'|'failed'|'skipped'; duration: number; error?: string }} TestCase
 * @typedef {{ title: string; tests: TestCase[] }} TestSuite
 */

/** @type {{ suites: TestSuite[], stats: { passed: number, failed: number, skipped: number, expected: number } } | null} */
let results = null
let runTimestamp = new Date().toISOString()

if (existsSync(RESULTS_PATH)) {
  try {
    results = JSON.parse(readFileSync(RESULTS_PATH, 'utf-8'))
    // Playwright JSON root may have `startTime`
    if (results?.startTime) {
      runTimestamp = new Date(results.startTime).toISOString()
    }
  } catch (e) {
    console.warn(`Warning: could not parse ${RESULTS_PATH}: ${e.message}`)
  }
} else {
  console.warn(`Warning: results file not found at ${RESULTS_PATH} — generating doc with no-data state`)
}

// ── Flatten test cases ────────────────────────────────────────────────────────

/**
 * Recursively walk Playwright JSON suites and collect all test cases.
 *
 * @param {object[]} suites
 * @param {string[]} parentPath
 * @returns {{ suitePath: string[]; title: string; status: string; duration: number; error?: string }[]}
 */
function flattenSuites(suites, parentPath = []) {
  const out = []
  for (const suite of suites ?? []) {
    const suitePath = [...parentPath, suite.title].filter(Boolean)
    for (const spec of suite.specs ?? []) {
      for (const test of spec.tests ?? []) {
        const result = test.results?.[0] ?? {}
        out.push({
          suitePath,
          title: spec.title,
          status: result.status ?? 'unknown',
          duration: result.duration ?? 0,
          error: result.error?.message ?? undefined,
        })
      }
    }
    // Recurse into nested suites
    out.push(...flattenSuites(suite.suites, suitePath))
  }
  return out
}

const allTests = flattenSuites(results?.suites ?? [])

// Group by top-level suite name (maps to service: auth, tutoring, gaming, notifications)
/** @type {Map<string, typeof allTests>} */
const byService = new Map()
for (const t of allTests) {
  const svc = t.suitePath[0] ?? 'Other'
  if (!byService.has(svc)) byService.set(svc, [])
  byService.get(svc).push(t)
}

// ── Screenshot resolution ─────────────────────────────────────────────────────

/**
 * Return a relative path (from docs/) to the screenshot, or empty string if
 * not found.  Screenshots are looked up by a normalized name within the
 * service subdirectory.
 *
 * @param {string} service - e.g. 'auth', 'gaming'
 * @param {string} name    - e.g. 'register-form'
 * @returns {string}
 */
function screenshotSrc(service, name) {
  const svcDir = join(SCREENSHOT_BASE, service)
  const candidates = existsSync(svcDir) ? readdirSync(svcDir) : []
  const match = candidates.find((f) => {
    const stem = f.replace(extname(f), '')
    return stem === name || stem.includes(name)
  })
  if (!match) return ''
  const absPath = join(svcDir, match)
  // Path relative to docs/
  return relative(DOCS_DIR, absPath).replace(/\\/g, '/')
}

// ── Stats ─────────────────────────────────────────────────────────────────────

const passed = allTests.filter((t) => t.status === 'passed').length
const failed = allTests.filter((t) => t.status === 'failed').length
const skipped = allTests.filter((t) => t.status === 'skipped' || t.status === 'pending').length
const total = allTests.length

// ── HTML helpers ─────────────────────────────────────────────────────────────

/**
 * Render a badge for a test status.
 *
 * @param {'passed'|'failed'|'skipped'|string} status
 */
function badge(status) {
  const map = {
    passed: ['green', '✅ PASSED'],
    failed: ['red', '❌ FAILED'],
    skipped: ['yellow', '⏭ SKIPPED'],
    pending: ['yellow', '⏳ PENDING'],
    unknown: ['muted', '? UNKNOWN'],
  }
  const [cls, label] = map[status] ?? ['muted', status.toUpperCase()]
  return `<span class="badge ${cls}">${label}</span>`
}

/**
 * Render a screenshot <img> or a placeholder box.
 *
 * @param {string} src - Relative path or empty string.
 * @param {string} alt
 */
function screenshotEl(src, alt) {
  if (src) {
    return `<img class="screenshot" src="${src}" alt="${alt}" loading="lazy">`
  }
  return `<div class="screenshot-placeholder">📸 Screenshot captured on next test run</div>`
}

/**
 * Render all tests in a service as <li> items.
 *
 * @param {string} svc
 */
function renderTests(svc) {
  const tests = byService.get(svc) ?? []
  if (tests.length === 0) {
    return `<p class="muted-text">No test results yet for this feature.</p>`
  }
  return `<ul class="test-list">
    ${tests
      .map(
        (t) => `
      <li class="test-item ${t.status}">
        <div class="test-header">
          ${badge(t.status)}
          <span class="test-title">${escHtml(t.suitePath.slice(1).join(' › '))} — ${escHtml(t.title)}</span>
          <span class="test-duration">${(t.duration / 1000).toFixed(1)}s</span>
        </div>
        ${t.error ? `<pre class="test-error">${escHtml(t.error)}</pre>` : ''}
      </li>`,
      )
      .join('\n')}
  </ul>`
}

/** HTML-escape a string. @param {string} s */
function escHtml(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

// ── Section definitions ───────────────────────────────────────────────────────

/**
 * @typedef {{ id: string; title: string; service: string; features: FeatureDef[] }} SectionDef
 * @typedef {{ id: string; title: string; description: string; screenshot: string }} FeatureDef
 */

/** @type {SectionDef[]} */
const SECTIONS = [
  {
    id: 'auth',
    title: 'Auth / User Service',
    service: 'Auth',
    features: [
      {
        id: 'register',
        title: 'Register new user',
        description: 'Registration form submission → success landing on dashboard.',
        screenshot: screenshotSrc('auth', 'register-success'),
      },
      {
        id: 'login',
        title: 'Login',
        description: 'Login with valid credentials → dashboard visible.',
        screenshot: screenshotSrc('auth', 'login-success-dashboard'),
      },
      {
        id: 'jwt-refresh',
        title: 'JWT refresh',
        description: 'POST /api/user/auth/refresh returns a new token without re-login.',
        screenshot: '',
      },
      {
        id: 'gdpr-export',
        title: 'GDPR data export',
        description: 'GET /api/user/profile/export returns user data download.',
        screenshot: screenshotSrc('auth', 'gdpr-export-confirmation'),
      },
    ],
  },
  {
    id: 'tutoring',
    title: 'Tutoring Service',
    service: 'Tutoring',
    features: [
      {
        id: 'start-session',
        title: 'Start tutoring session',
        description: 'Chat panel renders with empty thread ready for input.',
        screenshot: screenshotSrc('tutoring', 'chat-empty'),
      },
      {
        id: 'send-receive',
        title: 'Send message, receive tutor response',
        description: 'User sends a message; Prof. Nova replies via SSE stream.',
        screenshot: screenshotSrc('tutoring', 'chat-active'),
      },
      {
        id: 'context-summarisation',
        title: 'Context summarisation at 40+ messages',
        description: 'POST /api/chat/summarise is reachable and returns a summary.',
        screenshot: '',
      },
      {
        id: 'adaptive-dashboard',
        title: 'Adaptive dashboard',
        description: 'Mastery heatmap renders after study events.',
        screenshot: screenshotSrc('tutoring', 'adaptive-dashboard'),
      },
    ],
  },
  {
    id: 'gaming',
    title: 'Gaming Service',
    service: 'Gaming',
    features: [
      {
        id: 'streak',
        title: 'Streak tracking',
        description: 'Study event increments streak counter.',
        screenshot: screenshotSrc('gaming', 'streak-counter'),
      },
      {
        id: 'streak-freeze',
        title: 'Streak freeze purchase (50 gems)',
        description: 'POST /api/gaming/streak/freeze activates freeze.',
        screenshot: screenshotSrc('gaming', 'streak-freeze-active'),
      },
      {
        id: 'boss-entry',
        title: 'Boss battle entry',
        description: '/boss-battle/1 renders title card with name and stats.',
        screenshot: screenshotSrc('gaming', 'boss-title-card'),
      },
      {
        id: 'boss-active',
        title: 'Boss battle active',
        description: 'Three.js canvas mounts with HP bars; WS receives damage events.',
        screenshot: screenshotSrc('gaming', 'boss-battle-active'),
      },
      {
        id: 'boss-victory',
        title: 'Boss battle victory',
        description: 'Loot reveal modal appears after battle end.',
        screenshot: screenshotSrc('gaming', 'boss-victory-loot'),
      },
      {
        id: 'leaderboard',
        title: 'Leaderboard',
        description: 'Rankings tab renders leaderboard with 5+ entries.',
        screenshot: screenshotSrc('gaming', 'leaderboard'),
      },
      {
        id: 'shop',
        title: 'Shop catalog + purchase',
        description: '/shop renders catalog; purchase API is reachable.',
        screenshot: screenshotSrc('gaming', 'shop-catalog'),
      },
    ],
  },
  {
    id: 'notifications',
    title: 'Notification Service',
    service: 'Notifications',
    features: [
      {
        id: 'streak-risk',
        title: 'Streak-risk push notification',
        description:
          'Notification service sends push when streak is at 20–24 h (verified via API).',
        screenshot: '',
      },
      {
        id: 'rag-search',
        title: 'Search / RAG curriculum retrieval',
        description:
          'GET /api/search returns curriculum results; RAG context appears in tutor response.',
        screenshot: screenshotSrc('notifications', 'rag-context-in-response'),
      },
    ],
  },
]

// ── Render HTML ───────────────────────────────────────────────────────────────

/**
 * Build the complete testing-master.html content.
 *
 * @returns {string}
 */
function renderHtml() {
  const overallStatus = total === 0 ? 'no-data' : failed > 0 ? 'failing' : 'passing'
  const statusLabel =
    total === 0
      ? '⚪ No data'
      : failed > 0
        ? `❌ ${failed} failing`
        : `✅ All ${passed} passing`

  const sections = SECTIONS.map(
    (sec) => `
    <section id="${sec.id}">
      <h2>${escHtml(sec.title)}</h2>
      ${sec.features
        .map(
          (feat) => `
        <div class="feature-card">
          <div class="feature-header">
            <h3 id="${feat.id}">${escHtml(feat.title)}</h3>
          </div>
          <p>${escHtml(feat.description)}</p>
          ${screenshotEl(feat.screenshot, feat.title)}
        </div>`,
        )
        .join('\n')}
      <div class="test-results">
        <h4>Test results</h4>
        ${renderTests(sec.service)}
      </div>
    </section>`,
  ).join('\n')

  const navLinks = SECTIONS.map(
    (s) => `<a href="#${s.id}">${escHtml(s.title)}</a>`,
  ).join('\n    ')

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>TeachersLounge — Living Test Report</title>
  <style>
    :root {
      --bg: #0d1117;
      --surface: #161b22;
      --border: #30363d;
      --text: #e6edf3;
      --muted: #8b949e;
      --accent: #58a6ff;
      --green: #3fb950;
      --yellow: #d29922;
      --red: #f85149;
      --purple: #bc8cff;
      --orange: #ffa657;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; line-height: 1.6; }
    header { background: var(--surface); border-bottom: 1px solid var(--border); padding: 1.5rem 2rem; position: sticky; top: 0; z-index: 100; }
    header h1 { font-size: 1.4rem; color: var(--accent); }
    header p { color: var(--muted); font-size: 0.85rem; margin-top: 0.25rem; }
    .header-meta { display: flex; align-items: center; gap: 1.5rem; margin-top: 0.5rem; flex-wrap: wrap; }
    .status-pill { padding: 0.2rem 0.75rem; border-radius: 12px; font-size: 0.78rem; font-weight: 700; }
    .status-pill.passing { background: rgba(63,185,80,0.15); color: var(--green); border: 1px solid rgba(63,185,80,0.3); }
    .status-pill.failing { background: rgba(248,81,73,0.15); color: var(--red); border: 1px solid rgba(248,81,73,0.3); }
    .status-pill.no-data { background: rgba(139,148,158,0.15); color: var(--muted); border: 1px solid rgba(139,148,158,0.3); }
    .stats { font-size: 0.82rem; color: var(--muted); }
    nav { display: flex; gap: 1rem; margin-top: 0.75rem; flex-wrap: wrap; }
    nav a { color: var(--accent); text-decoration: none; font-size: 0.82rem; opacity: 0.8; }
    nav a:hover { opacity: 1; }
    main { max-width: 1100px; margin: 0 auto; padding: 2rem; }
    section { margin-bottom: 3rem; }
    h2 { color: var(--accent); font-size: 1.2rem; margin: 2.5rem 0 1rem; padding-bottom: 0.5rem; border-bottom: 1px solid var(--border); }
    h3 { color: var(--text); font-size: 1rem; margin-bottom: 0.25rem; }
    h4 { color: var(--muted); font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.06em; margin: 1.25rem 0 0.5rem; }
    p { color: var(--muted); font-size: 0.88rem; margin-bottom: 0.5rem; }
    .muted-text { color: var(--muted); font-size: 0.85rem; font-style: italic; }
    .feature-card { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 1.25rem; margin: 1rem 0; }
    .feature-header { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 0.5rem; }
    .screenshot { max-width: 100%; border: 1px solid var(--border); border-radius: 6px; margin-top: 0.75rem; display: block; }
    .screenshot-placeholder { background: #0a0e13; border: 1px dashed var(--border); border-radius: 6px; padding: 2rem 1.5rem; text-align: center; color: var(--muted); font-size: 0.82rem; margin-top: 0.75rem; }
    .test-results { margin-top: 1rem; }
    .test-list { list-style: none; }
    .test-item { border-left: 3px solid var(--border); padding: 0.5rem 0.75rem; margin: 0.4rem 0; background: var(--surface); border-radius: 0 6px 6px 0; }
    .test-item.passed { border-left-color: var(--green); }
    .test-item.failed { border-left-color: var(--red); }
    .test-item.skipped, .test-item.pending { border-left-color: var(--yellow); }
    .test-header { display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap; }
    .test-title { color: var(--text); font-size: 0.85rem; flex: 1; }
    .test-duration { color: var(--muted); font-size: 0.78rem; white-space: nowrap; }
    .test-error { background: #1a0000; border: 1px solid var(--red); border-radius: 4px; padding: 0.5rem 0.75rem; font-family: monospace; font-size: 0.78rem; color: var(--red); margin-top: 0.4rem; white-space: pre-wrap; overflow-x: auto; }
    .badge { display: inline-flex; align-items: center; padding: 0.15rem 0.5rem; border-radius: 10px; font-size: 0.72rem; font-weight: 700; white-space: nowrap; }
    .badge.green { background: rgba(63,185,80,0.15); color: var(--green); border: 1px solid rgba(63,185,80,0.3); }
    .badge.red { background: rgba(248,81,73,0.15); color: var(--red); border: 1px solid rgba(248,81,73,0.3); }
    .badge.yellow { background: rgba(210,153,34,0.15); color: var(--yellow); border: 1px solid rgba(210,153,34,0.3); }
    .badge.muted { background: rgba(139,148,158,0.1); color: var(--muted); border: 1px solid rgba(139,148,158,0.2); }
    footer { border-top: 1px solid var(--border); padding: 1.5rem 2rem; color: var(--muted); font-size: 0.8rem; text-align: center; margin-top: 3rem; }
  </style>
</head>
<body>
  <header>
    <h1>TeachersLounge — Living Test Report</h1>
    <p>Auto-generated by <code>scripts/gen-testing-doc.mjs</code> · Last run: ${escHtml(runTimestamp)}</p>
    <div class="header-meta">
      <span class="status-pill ${overallStatus}">${statusLabel}</span>
      ${total > 0 ? `<span class="stats">${passed} passed · ${failed} failed · ${skipped} skipped · ${total} total</span>` : ''}
    </div>
    <nav>
      ${navLinks}
    </nav>
  </header>
  <main>
    ${sections}
  </main>
  <footer>
    Generated by <code>scripts/gen-testing-doc.mjs</code> &mdash;
    run <code>cd e2e &amp;&amp; npm test &amp;&amp; cd .. &amp;&amp; node scripts/gen-testing-doc.mjs</code> to refresh.
  </footer>
</body>
</html>`
}

// ── Write output ──────────────────────────────────────────────────────────────

const html = renderHtml()
writeFileSync(OUT_PATH, html, 'utf-8')
console.log(`✅ testing-master.html written → ${OUT_PATH}`)
console.log(`   ${passed} passed · ${failed} failed · ${skipped} skipped · ${total} total`)
if (total === 0) {
  console.log(`   (no test results found — run 'cd e2e && npm test' first)`)
}
