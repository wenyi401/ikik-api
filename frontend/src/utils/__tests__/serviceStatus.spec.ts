import { describe, expect, it } from 'vitest'
import { buildServiceEndpointTargets, median } from '../serviceStatus'

describe('buildServiceEndpointTargets', () => {
  it('uses the configured default endpoint and de-duplicates custom endpoints', () => {
    const targets = buildServiceEndpointTargets(
      'https://ikik.net/v1',
      [
        { name: 'Global', endpoint: 'https://ikik.net/v1/', description: '' },
        { name: 'Optimized', endpoint: 'ai.ikik.net/v1', description: '' },
      ],
      'https://fallback.test',
    )

    expect(targets).toHaveLength(2)
    expect(targets[0]).toMatchObject({
      endpoint: 'https://ikik.net/v1',
      healthUrl: 'https://ikik.net/health',
      isDefault: true,
    })
    expect(targets[1].healthUrl).toBe('https://ai.ikik.net/health')
  })

  it('falls back to the current origin', () => {
    expect(buildServiceEndpointTargets('', [], 'https://console.test')[0].healthUrl).toBe(
      'https://console.test/health',
    )
  })
})

describe('median', () => {
  it('returns the median latency', () => {
    expect(median([90, 12, 30])).toBe(30)
    expect(median([10, 20])).toBe(15)
    expect(median([])).toBeNull()
  })
})
