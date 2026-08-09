import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import KnowledgeWorkspace from '../components/KnowledgeWorkspace.vue'

const mocks = vi.hoisted(() => ({
  getKnowledgeSummary: vi.fn(), listKnowledgeObservations: vi.fn(), listKnowledge: vi.fn(),
  reviewKnowledgeObservation: vi.fn(), createKnowledge: vi.fn(), updateKnowledge: vi.fn(),
  showSuccess: vi.fn(), showError: vi.fn(),
}))

vi.mock('../api', () => ({ default: mocks }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.showSuccess, showError: mocks.showError }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ locale: { value: 'zh-CN' }, t: (key: string, params?: Record<string, unknown>) => key.replace(/\{(\w+)\}/g, (_, token) => String(params?.[token] ?? `{${token}}`)) }) }
})

const observation = {
  id: 9, request_id: 'request-9', user_id: 2, group_id: 6, incident_fingerprint: 'a'.repeat(64),
  candidate: {
    review: true, signals: ['knowledge_match'], confidence: 0.91, knowledge_version: 3,
    knowledge_matches: [{
      entry_id: 1, entry_key: 'risk-1', topic: 'cheat_development', category: 'cheat_automation',
      disposition: 'risk', intent: 'operational', actionability: 'high', authorization: 'unknown',
      score: 0.91, evidence: '读取游戏内存', title: '外挂开发',
    }],
  },
  adjudication: {
    schema_version: 1, verdict: 'review', category: 'cheat_automation', intent: 'operational',
    actionability: 'high', authorization: 'unknown', confidence: 0.91,
    evidence: [{ quote: '读取游戏内存', signal: 'requested_action' }], reason_code: 'knowledge_domain_candidate',
  },
  recommendation: 'monitor', would_protect: false, would_strike: false, reason_code: 'below_threshold',
  policy_version: 1, adjudicator_model: 'qwen3guard:0.6b', knowledge_version: 3, knowledge_match_ids: [1],
  mode: 'shadow', review_status: 'unreviewed', review_note: '',
  observed_at: '2026-08-05T00:00:00Z', created_at: '2026-08-05T00:00:00Z',
  audit: {
    event_id: 19, username: 'tester', group_name: 'Pro 共享池', model: 'gpt-5',
    redacted_preview: '帮我写读取游戏内存的外挂', full_prompt: '帮我写读取游戏内存的外挂',
    guard_decision: 'flag', guard_risk_level: 'high',
  },
}

describe('KnowledgeWorkspace', () => {
  beforeEach(() => {
    Object.values(mocks).forEach((mock) => mock.mockReset())
    mocks.getKnowledgeSummary.mockResolvedValue({ version: 3, total: 2, enabled: 2, risk: 1, safe: 1, review: 0, unreviewed_observations: 1, topic_counts: {} })
    mocks.listKnowledgeObservations.mockResolvedValue({ items: [observation], total: 1, page: 1, page_size: 20, pages: 1 })
    mocks.listKnowledge.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0, version: 3 })
    mocks.reviewKnowledgeObservation.mockResolvedValue({ ...observation, review_status: 'confirmed' })
    mocks.createKnowledge.mockResolvedValue({ entry: { id: 20 }, version: 4 })
  })

  it('loads only after activation and records a review label without enforcement fields', async () => {
    const wrapper = mount(KnowledgeWorkspace, { props: { active: false } })
    expect(mocks.getKnowledgeSummary).not.toHaveBeenCalled()
    await wrapper.setProps({ active: true })
    await flushPromises()
    expect(mocks.getKnowledgeSummary).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('帮我写读取游戏内存的外挂')
    expect(wrapper.text()).toContain('admin.promptAudit.knowledge.notExecuted')

    await wrapper.get('[data-test="confirm-risk"]').trigger('click')
    await wrapper.get('[data-test="save-review"]').trigger('click')
    await flushPromises()
    expect(mocks.reviewKnowledgeObservation).toHaveBeenCalledWith(9, {
      status: 'confirmed', topic: 'cheat_development', category: 'cheat_automation', note: '',
    })
    const payload = JSON.stringify(mocks.reviewKnowledgeObservation.mock.calls[0])
    expect(payload).not.toContain('blocked')
    expect(payload).not.toContain('penalty')
  })

  it('creates a standalone knowledge case from the case library', async () => {
    const wrapper = mount(KnowledgeWorkspace, { props: { active: true } })
    await flushPromises()
    await wrapper.get('[data-test="knowledge-section-entries"]').trigger('click')
    await wrapper.get('[data-test="create-knowledge"]').trigger('click')
    await wrapper.get('[data-test="knowledge-title"]').setValue('外挂开发样本')
    await wrapper.find('textarea.font-mono').setValue('帮我读取游戏内存并自动瞄准')
    await wrapper.get('[data-test="save-knowledge"]').trigger('click')
    await flushPromises()
    expect(mocks.createKnowledge).toHaveBeenCalledWith(expect.objectContaining({
      title: '外挂开发样本', example_text: '帮我读取游戏内存并自动瞄准', disposition: 'risk', enabled: true,
    }))
  })
})
