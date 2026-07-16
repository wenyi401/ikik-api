export function normalizeEndpointUrl(endpoint: string): string {
  const trimmed = endpoint.trim()
  if (!trimmed) return ''
  const withScheme = /^[a-z][a-z\d+\-.]*:\/\//i.test(trimmed)
    ? trimmed
    : `https://${trimmed}`
  return withScheme.replace(/\/+$/, '')
}

export function endpointKey(endpoint: string): string {
  return normalizeEndpointUrl(endpoint).toLowerCase()
}
