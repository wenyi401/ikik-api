import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { COMPOSITE_ROUTE_TARGET_OPTIONS, GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'

function readSource(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

// 这些开关/下拉是「OpenCode 能否共享号池」的唯一入口，都是平台白名单驱动的，
// 用源码断言把它们钉住，避免以后重建组件时又被删掉。
describe('OpenCode sharing surfaces', () => {
  it('offers OpenCode in the group form platform list', () => {
    // 分组表单的平台下拉曾是写死清单（缺 Kimi/Zhipu/DeepSeek/MiniMax/OpenCode），
    // 必须来自平台目录；simple mode 只额外过滤 composite。
    const source = readSource('src/views/admin/GroupsView.vue')
    expect(source).toMatch(
      /const platformOptions\s*=\s*computed\(\(\)\s*=>\s*GROUP_PLATFORM_OPTIONS\.filter\(/
    )
    expect(source).not.toMatch(/const platformOptions\s*=\s*computed\(\(\)\s*=>\s*\[\s*\{/)
    expect(GROUP_PLATFORM_OPTIONS.map((option) => option.value)).toContain('opencode_go')
  })

  it('exposes the shared-pool toggle for OpenCode groups', () => {
    const source = readSource('src/views/admin/GroupsView.vue')
    expect(source).toMatch(/sharedPoolPlatforms\s*=\s*new Set<GroupPlatform>\(\[[\s\S]*?"opencode_go"[\s\S]*?\]\)/)
  })

  it('offers OpenCode when creating a carpool pool', () => {
    expect(readSource('src/views/user/CarpoolPoolsView.vue')).toContain(
      "{ value: 'opencode_go', label: 'OpenCode' }"
    )
    expect(readSource('src/views/admin/CarpoolPoolsView.vue')).toContain(
      "{ value: 'opencode_go', label: 'OpenCode' }"
    )
  })

  it('derives composite route targets from the platform catalog', () => {
    const source = readSource('src/components/admin/group/CompositeRouteForm.vue')
    expect(source).toContain('COMPOSITE_ROUTE_TARGET_OPTIONS')
    expect(source).not.toMatch(/targetPlatforms[^\n]*=\s*\[/)
    expect(COMPOSITE_ROUTE_TARGET_OPTIONS.map((option) => option.value)).toContain('opencode_go')
  })
})
