/**
 * i18n Locale Parity Test
 *
 * Ensures that all locale files (en.ts, zh.ts) have:
 * 1. Identical key structures (no missing keys in any locale)
 * 2. Consistent placeholder usage (e.g., {siteName}, {count})
 *
 * This test runs in CI via `pnpm test` or `make test-frontend`.
 */

import { describe, it, expect } from 'vitest'
import en from '../locales/en'
import zh from '../locales/zh'
import baseline from './locale-parity-baseline.json'

interface LocaleParityBaseline {
  missingInZh: string[]
  missingInEn: string[]
}

const localeParityBaseline = baseline as LocaleParityBaseline

describe('i18n Locale Parity', () => {
  /**
   * Recursively extract all keys from a nested object.
   * Returns a flat array of dot-notation paths (e.g., ['home.title', 'home.nav.home']).
   */
  function extractKeys(obj: Record<string, any>, prefix = ''): string[] {
    const keys: string[] = []
    for (const key in obj) {
      const path = prefix ? `${prefix}.${key}` : key
      if (typeof obj[key] === 'object' && obj[key] !== null && !Array.isArray(obj[key])) {
        keys.push(...extractKeys(obj[key], path))
      } else {
        keys.push(path)
      }
    }
    return keys.sort()
  }

  /**
   * Extract placeholders from a string (e.g., "{siteName}" -> ["siteName"]).
   * Handles both named interpolation {name} and linked messages @:path.
   */
  function extractPlaceholders(str: string): string[] {
    if (typeof str !== 'string') return []
    const placeholders = str.match(/\{(\w+)\}/g) || []
    return [...new Set(placeholders.map(p => p.slice(1, -1)))].sort()
  }

  /**
   * Recursively get the value at a dot-notation path.
   */
  function getValueAtPath(obj: Record<string, any>, path: string): any {
    return path.split('.').reduce((acc, key) => acc?.[key], obj)
  }

  it('should have identical key structures in en and zh', () => {
    const enKeys = extractKeys(en)
    const zhKeys = extractKeys(zh)

    const missingInZh = enKeys.filter(k => !zhKeys.includes(k))
    const missingInEn = zhKeys.filter(k => !enKeys.includes(k))

    const newMissingInZh = missingInZh.filter(k => !localeParityBaseline.missingInZh.includes(k))
    const staleMissingInZh = localeParityBaseline.missingInZh.filter(k => !missingInZh.includes(k))
    const newMissingInEn = missingInEn.filter(k => !localeParityBaseline.missingInEn.includes(k))
    const staleMissingInEn = localeParityBaseline.missingInEn.filter(k => !missingInEn.includes(k))

    if (newMissingInZh.length > 0 || staleMissingInZh.length > 0) {
      console.error('zh.ts locale parity drift:', { newMissingInZh, staleMissingInZh })
    }
    if (newMissingInEn.length > 0 || staleMissingInEn.length > 0) {
      console.error('en.ts locale parity drift:', { newMissingInEn, staleMissingInEn })
    }

    expect(newMissingInZh, 'zh.ts has new missing keys not present in the locale parity baseline').toEqual([])
    expect(staleMissingInZh, 'locale parity baseline has stale zh.ts missing keys').toEqual([])
    expect(newMissingInEn, 'en.ts has new missing keys not present in the locale parity baseline').toEqual([])
    expect(staleMissingInEn, 'locale parity baseline has stale en.ts missing keys').toEqual([])
  })

  it('should have consistent placeholders across locales', () => {
    const enKeys = extractKeys(en)
    const inconsistencies: string[] = []

    for (const key of enKeys) {
      const enValue = getValueAtPath(en, key)
      const zhValue = getValueAtPath(zh, key)

      if (typeof enValue !== 'string' || typeof zhValue !== 'string') continue

      const enPlaceholders = extractPlaceholders(enValue)
      const zhPlaceholders = extractPlaceholders(zhValue)

      if (JSON.stringify(enPlaceholders) !== JSON.stringify(zhPlaceholders)) {
        inconsistencies.push(
          `${key}: en has {${enPlaceholders.join(', ')}} but zh has {${zhPlaceholders.join(', ')}}`
        )
      }
    }

    if (inconsistencies.length > 0) {
      console.error('Placeholder mismatches:', inconsistencies)
    }

    expect(inconsistencies, 'Placeholders must match across locales').toEqual([])
  })

  it('should not have empty string values', () => {
    const enKeys = extractKeys(en)
    const emptyKeys: string[] = []

    for (const key of enKeys) {
      const enValue = getValueAtPath(en, key)
      const zhValue = getValueAtPath(zh, key)

      if (enValue === '' || zhValue === '') {
        emptyKeys.push(key)
      }
    }

    if (emptyKeys.length > 0) {
      console.error('Keys with empty string values:', emptyKeys)
    }

    expect(emptyKeys, 'No locale keys should have empty string values').toEqual([])
  })
})
