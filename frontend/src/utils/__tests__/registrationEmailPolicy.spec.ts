import { describe, expect, it } from 'vitest'
import {
  formatRegistrationEmailSuffixWhitelistForMessage,
  isRegistrationEmailSuffixAllowed,
  isRegistrationEmailSuffixDomainValid,
  normalizeRegistrationEmailSuffixDomain,
  normalizeRegistrationEmailSuffixDomains,
  normalizeRegistrationEmailSuffixWhitelist,
  parseRegistrationEmailSuffixWhitelistInput
} from '@/utils/registrationEmailPolicy'

describe('registrationEmailPolicy utils', () => {
  it('normalizeRegistrationEmailSuffixDomain lowercases, strips @, and ignores invalid chars', () => {
    expect(normalizeRegistrationEmailSuffixDomain(' @Exa!mple.COM ')).toBe('example.com')
  })

  it('normalizeRegistrationEmailSuffixDomains deduplicates normalized domains', () => {
    expect(
      normalizeRegistrationEmailSuffixDomains([
        '@example.com',
        'Example.com',
        '',
        '-invalid.com',
        'foo..bar.com',
        ' @foo.bar ',
        '@foo.bar'
      ])
    ).toEqual(['example.com', 'foo.bar'])
  })

  it('parseRegistrationEmailSuffixWhitelistInput supports separators and deduplicates', () => {
    const input = '\n  @example.com,example.com，@foo.bar\t@FOO.bar  '
    expect(parseRegistrationEmailSuffixWhitelistInput(input)).toEqual(['example.com', 'foo.bar'])
  })

  it('parseRegistrationEmailSuffixWhitelistInput drops tokens containing invalid chars', () => {
    const input = '@exa!mple.com, @foo.bar, @bad#token.com, @ok-domain.com'
    expect(parseRegistrationEmailSuffixWhitelistInput(input)).toEqual(['foo.bar', 'ok-domain.com'])
  })

  it('parseRegistrationEmailSuffixWhitelistInput drops structurally invalid domains', () => {
    const input = '@-bad.com, @foo..bar.com, @foo.bar, @xn--ok.com'
    expect(parseRegistrationEmailSuffixWhitelistInput(input)).toEqual(['foo.bar', 'xn--ok.com'])
  })

  it('parseRegistrationEmailSuffixWhitelistInput returns empty list for blank input', () => {
    expect(parseRegistrationEmailSuffixWhitelistInput('   \n \n')).toEqual([])
  })

  it('normalizeRegistrationEmailSuffixWhitelist returns canonical @domain list', () => {
    expect(
      normalizeRegistrationEmailSuffixWhitelist([
        '@Example.com',
        'foo.bar',
        '',
        '-invalid.com',
        ' @foo.bar '
      ])
    ).toEqual(['@example.com', '@foo.bar'])
  })

  it('isRegistrationEmailSuffixDomainValid matches backend-compatible domain rules', () => {
    expect(isRegistrationEmailSuffixDomainValid('example.com')).toBe(true)
    expect(isRegistrationEmailSuffixDomainValid('foo-bar.example.com')).toBe(true)
    expect(isRegistrationEmailSuffixDomainValid('-bad.com')).toBe(false)
    expect(isRegistrationEmailSuffixDomainValid('foo..bar.com')).toBe(false)
    expect(isRegistrationEmailSuffixDomainValid('localhost')).toBe(false)
  })

  it('isRegistrationEmailSuffixAllowed allows any email when whitelist is empty', () => {
    expect(isRegistrationEmailSuffixAllowed('user@example.com', [])).toBe(true)
  })

  it('isRegistrationEmailSuffixAllowed applies exact suffix matching', () => {
    expect(isRegistrationEmailSuffixAllowed('user@example.com', ['@example.com'])).toBe(true)
    expect(isRegistrationEmailSuffixAllowed('user@sub.example.com', ['@example.com'])).toBe(false)
  })
})


describe('registration email policy utils (upstream additions)', () => {
  it('isRegistrationEmailSuffixAllowed applies wildcard suffix matching', () => {
    expect(isRegistrationEmailSuffixAllowed('student@cs.edu.cn', ['*.edu.cn'])).toBe(true)
    expect(isRegistrationEmailSuffixAllowed('student@edu.cn', ['*.edu.cn'])).toBe(true)
    expect(isRegistrationEmailSuffixAllowed('student@foo.cn', ['*.edu.cn'])).toBe(false)
  })

  it('isRegistrationEmailSuffixAllowed supports mixed exact and wildcard entries', () => {
    const whitelist = ['@a.com', '*.b.cn']
    expect(isRegistrationEmailSuffixAllowed('user@a.com', whitelist)).toBe(true)
    expect(isRegistrationEmailSuffixAllowed('user@school.b.cn', whitelist)).toBe(true)
    expect(isRegistrationEmailSuffixAllowed('user@b.cn', whitelist)).toBe(true)
    expect(isRegistrationEmailSuffixAllowed('user@c.cn', whitelist)).toBe(false)
  })

  it('formatRegistrationEmailSuffixWhitelistForMessage lists up to five entries', () => {
    expect(
      formatRegistrationEmailSuffixWhitelistForMessage(
        ['@a.com', '@b.com', '@c.com', '@d.com', '@e.com'],
        { separator: ', ', more: (count) => `and ${count} more` }
      )
    ).toBe('@a.com, @b.com, @c.com, @d.com, @e.com')
    expect(
      formatRegistrationEmailSuffixWhitelistForMessage(
        ['@a.com', '@b.com', '@c.com', '@d.com', '@e.com', '*.edu.cn', '@f.com'],
        { separator: ', ', more: (count) => `and ${count} more` }
      )
    ).toBe('@a.com, @b.com, @c.com, @d.com, @e.com, and 2 more')
  })
})
