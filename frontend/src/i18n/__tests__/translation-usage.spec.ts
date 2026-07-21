import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from '../locales/runtime-en'
import zh from '../locales/runtime-zh'

const srcRoot = join(process.cwd(), 'src')
const localeRoot = join(srcRoot, 'i18n', 'locales')

function sourceFiles(directory: string): string[] {
  return readdirSync(directory).flatMap((name) => {
    const path = join(directory, name)
    if (path.startsWith(localeRoot)) return []
    return statSync(path).isDirectory()
      ? sourceFiles(path)
      : /\.(?:ts|vue)$/.test(name)
        ? [path]
        : []
  })
}

function hasKey(messages: Record<string, unknown>, key: string): boolean {
  let current: unknown = messages
  for (const segment of key.split('.')) {
    if (!current || typeof current !== 'object' || !(segment in current)) return false
    current = (current as Record<string, unknown>)[segment]
  }
  return typeof current === 'string' || typeof current === 'number'
}

function staticTranslationKeys(source: string): string[] {
  const keys = new Set<string>()
  const patterns = [
    /(?:\bt|\$t|\.t)\(\s*(['"`])([^'"`$]+)\1/g,
    /\btitleKey\s*:\s*(['"`])([^'"`$]+)\1/g,
  ]

  for (const pattern of patterns) {
    for (const match of source.matchAll(pattern)) {
      const key = match[2].trim()
      if (key && !key.endsWith('.')) keys.add(key)
    }
  }
  return [...keys]
}

describe('static translation usage', () => {
  it('resolves every static key in both runtime locales', () => {
    const missing: string[] = []
    for (const file of sourceFiles(srcRoot)) {
      const source = readFileSync(file, 'utf8')
      for (const key of staticTranslationKeys(source)) {
        const missingLocales = [
          !hasKey(en as Record<string, unknown>, key) ? 'en' : '',
          !hasKey(zh as Record<string, unknown>, key) ? 'zh' : '',
        ].filter(Boolean)
        if (missingLocales.length > 0) {
          missing.push(`${relative(srcRoot, file)}: ${key} (${missingLocales.join(', ')})`)
        }
      }
    }

    expect(missing.sort()).toEqual([])
  })
})
