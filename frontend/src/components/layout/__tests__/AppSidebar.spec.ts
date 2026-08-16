import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar developer API navigation', () => {
  it('keeps the developer API entry under account management, not personal center', () => {
    const accountManagementStart = componentSource.indexOf("path: '/self/accounts'")
    const personalCenterStart = componentSource.indexOf("path: '/self/profile-center'")
    const additionalMenuStart = componentSource.indexOf("path: '/self/extras'")
    const developerApiEntry = componentSource.indexOf(
      "{ path: '/developer-api', label: t('nav.developerApi'), icon: KeyIcon }",
    )

    expect(accountManagementStart).toBeGreaterThanOrEqual(0)
    expect(developerApiEntry).toBeGreaterThan(accountManagementStart)
    expect(developerApiEntry).toBeLessThan(personalCenterStart)
    expect(componentSource.slice(personalCenterStart, additionalMenuStart)).not.toContain(
      "path: '/developer-api'",
    )
  })
})

describe('AppSidebar pet navigation', () => {
  it('keeps Pet hall under personal center', () => {
    const personalCenterStart = componentSource.indexOf("path: '/self/profile-center'")
    const additionalMenuStart = componentSource.indexOf("path: '/self/extras'")
    const petEntry = componentSource.indexOf("{ path: '/pet', label: t('nav.pet'), icon: GiftIcon }")

    expect(personalCenterStart).toBeGreaterThanOrEqual(0)
    expect(petEntry).toBeGreaterThan(personalCenterStart)
    expect(petEntry).toBeLessThan(additionalMenuStart)
  })
})
