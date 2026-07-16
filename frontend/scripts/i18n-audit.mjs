#!/usr/bin/env node
/**
 * i18n hardcoded Chinese audit.
 *
 * Scans .vue templates under src/ for user-facing CJK text that should live in
 * src/i18n/locales/{en,zh}.ts. Existing legacy findings can be pinned in
 * scripts/i18n-audit-baseline.json so CI blocks new debt without requiring a
 * full translation cleanup in the same change.
 */

import {
  existsSync,
  readdirSync,
  readFileSync,
  statSync,
  writeFileSync
} from 'node:fs'
import { join, relative, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))
const SRC_DIR = join(__dirname, '..', 'src')
const ROOT = join(__dirname, '..')
const BASELINE_FILE = join(__dirname, 'i18n-audit-baseline.json')

const args = process.argv.slice(2)
const STRICT = args.includes('--strict')
const JSON_OUT = args.includes('--json')
const UPDATE_BASELINE = args.includes('--update-baseline')

// CJK ideographs plus common CJK punctuation and full-width forms.
const CJK_RE = /[\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF\u3000-\u303F\uFF00-\uFFEF]/u

const SKIP_DIRS = new Set(['node_modules', 'dist', '__tests__', '.git'])
const SKIP_FILES = new Set([
  join(SRC_DIR, 'i18n', 'locales', 'zh.ts'),
  join(SRC_DIR, 'i18n', 'locales', 'en.ts')
])

function normalizePath(path) {
  return path.split(sep).join('/')
}

function walk(dir, acc = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    const st = statSync(full)
    if (st.isDirectory()) {
      if (!SKIP_DIRS.has(entry)) walk(full, acc)
    } else if (entry.endsWith('.vue')) {
      acc.push(full)
    }
  }
  return acc
}

function preserveLines(source) {
  return source.replace(/[^\n]/g, ' ')
}

function templateOnly(source) {
  return source
    .replace(/<(script|style)\b[^>]*>[\s\S]*?<\/\1>/gi, preserveLines)
    .replace(/<!--[\s\S]*?-->/g, preserveLines)
}

function ignoredLineIndexes(source) {
  const ignored = new Set()
  source.split('\n').forEach((line, index) => {
    if (/i18n-ignore/.test(line)) {
      ignored.add(index)
      ignored.add(index + 1)
    }
  })
  return ignored
}

function lineHasHardcodedCJK(line) {
  if (!CJK_RE.test(line)) return false

  // Remove the first argument of t('key') / $t("key") calls. Fallback strings
  // remain visible because they are user-facing hardcoded copy.
  const withoutI18nKeys = line.replace(/\$?t\(\s*(['"`])(?:\\.|(?!\1).)*\1/g, '')
  return CJK_RE.test(withoutI18nKeys)
}

function fingerprint(finding) {
  return `${finding.file}\u0000${finding.text}`
}

function countFindings(findingsToCount) {
  const counts = new Map()
  for (const finding of findingsToCount) {
    const key = fingerprint(finding)
    counts.set(key, (counts.get(key) || 0) + 1)
  }
  return counts
}

function readBaseline() {
  if (!existsSync(BASELINE_FILE)) return new Map()
  const raw = JSON.parse(readFileSync(BASELINE_FILE, 'utf8'))
  const counts = new Map()
  for (const entry of raw.entries || []) {
    counts.set(`${entry.file}\u0000${entry.text}`, entry.count || 1)
  }
  return counts
}

function baselineEntries(findingsToWrite) {
  return [...countFindings(findingsToWrite).entries()]
    .map(([key, count]) => {
      const [file, text] = key.split('\u0000')
      return { file, text, count }
    })
    .sort((a, b) => a.file.localeCompare(b.file) || a.text.localeCompare(b.text))
}

function withoutBaseline(allFindings, baselineCounts) {
  const remainingBaseline = new Map(baselineCounts)
  const unexpected = []

  for (const finding of allFindings) {
    const key = fingerprint(finding)
    const allowance = remainingBaseline.get(key) || 0
    if (allowance > 0) {
      remainingBaseline.set(key, allowance - 1)
    } else {
      unexpected.push(finding)
    }
  }

  return unexpected
}

const findings = []

for (const file of walk(SRC_DIR)) {
  if (SKIP_FILES.has(file)) continue
  const source = readFileSync(file, 'utf8')
  const scanned = templateOnly(source)
  const ignored = ignoredLineIndexes(source)
  const lines = scanned.split('\n')
  lines.forEach((line, index) => {
    if (ignored.has(index)) return
    if (lineHasHardcodedCJK(line)) {
      findings.push({
        file: normalizePath(relative(ROOT, file)),
        line: index + 1,
        text: line.trim().slice(0, 160)
      })
    }
  })
}

if (UPDATE_BASELINE) {
  writeFileSync(
    BASELINE_FILE,
    `${JSON.stringify({ version: 1, entries: baselineEntries(findings) }, null, 2)}\n`
  )
  console.log(`i18n audit baseline updated with ${findings.length} finding(s).`)
  process.exit(0)
}

const baselineCounts = readBaseline()
const unexpectedFindings = withoutBaseline(findings, baselineCounts)
const ignoredCount = findings.length - unexpectedFindings.length

if (JSON_OUT) {
  console.log(JSON.stringify({
    count: unexpectedFindings.length,
    total: findings.length,
    baselineIgnored: ignoredCount,
    findings: unexpectedFindings
  }, null, 2))
} else if (unexpectedFindings.length === 0) {
  const suffix = ignoredCount > 0 ? ` (${ignoredCount} baseline finding(s) ignored)` : ''
  console.log(`OK i18n audit passed: no new hardcoded Chinese found${suffix}.`)
} else {
  console.log(`ERROR i18n audit found ${unexpectedFindings.length} new hardcoded Chinese string(s):\n`)
  for (const f of unexpectedFindings) {
    console.log(`  ${f.file}:${f.line}`)
    console.log(`      ${f.text}`)
  }
  console.log(
    '\nMove these into src/i18n/locales/{en,zh}.ts and use t(\'...\').' +
    '\nIf a string is intentionally not translatable, add an i18n-ignore comment on the line above it.'
  )
}

if (STRICT && unexpectedFindings.length > 0) {
  process.exit(1)
}
