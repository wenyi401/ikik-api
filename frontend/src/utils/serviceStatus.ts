import type { CustomEndpoint } from '@/types'
import { endpointKey, normalizeEndpointUrl } from './apiEndpoints'

export interface ServiceEndpointTarget {
  id: string
  name: string
  endpoint: string
  healthUrl: string
  isDefault: boolean
}

function buildHealthUrl(endpoint: string): string {
  const url = new URL(normalizeEndpointUrl(endpoint))
  url.pathname = '/health'
  url.search = ''
  url.hash = ''
  return url.toString()
}

export function buildServiceEndpointTargets(
  apiBaseUrl: string | null | undefined,
  customEndpoints: CustomEndpoint[] | null | undefined,
  currentOrigin: string,
): ServiceEndpointTarget[] {
  const result: ServiceEndpointTarget[] = []
  const seen = new Set<string>()

  const append = (name: string, endpoint: string, isDefault: boolean) => {
    const normalized = normalizeEndpointUrl(endpoint)
    if (!normalized) return
    const key = endpointKey(normalized)
    if (seen.has(key)) return
    try {
      const healthUrl = buildHealthUrl(normalized)
      seen.add(key)
      result.push({ id: key, name, endpoint: normalized, healthUrl, isDefault })
    } catch {
      // Invalid entries remain editable in settings but are not tested here.
    }
  }

  append('', apiBaseUrl || currentOrigin, true)
  for (const endpoint of customEndpoints ?? []) {
    append(endpoint.name.trim(), endpoint.endpoint, false)
  }
  return result
}

export function median(values: number[]): number | null {
  if (values.length === 0) return null
  const sorted = [...values].sort((a, b) => a - b)
  const middle = Math.floor(sorted.length / 2)
  if (sorted.length % 2 === 1) return sorted[middle]
  return (sorted[middle - 1] + sorted[middle]) / 2
}
