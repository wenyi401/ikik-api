import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useEndpointDiagnostics, type EndpointDiagnosticResult } from '../useEndpointDiagnostics'
import type { ServiceEndpointTarget } from '@/utils/serviceStatus'

interface DiagnosticsExposed {
  test: (target: ServiceEndpointTarget) => Promise<void>
  resultFor: (id: string) => EndpointDiagnosticResult
}

const Harness = defineComponent({
  setup(_, { expose }) {
    const diagnostics = useEndpointDiagnostics()
    expose(diagnostics)
    return () => null
  },
})

afterEach(() => vi.restoreAllMocks())

describe('useEndpointDiagnostics', () => {
  it('treats an opaque cross-origin health response as reachable', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      type: 'opaque',
      ok: false,
      status: 0,
    } as Response)
    const wrapper = mount(Harness)
    const diagnostics = wrapper.vm as unknown as DiagnosticsExposed
    const target: ServiceEndpointTarget = {
      id: 'ai.ikik.net',
      name: 'Optimized',
      endpoint: 'https://ai.ikik.net',
      healthUrl: 'https://ai.ikik.net/health',
      isDefault: false,
    }

    await diagnostics.test(target)

    expect(diagnostics.resultFor(target.id).status).toBe('online')
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({ mode: 'no-cors' })
  })
})
