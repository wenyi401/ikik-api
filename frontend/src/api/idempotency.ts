export function createIdempotencyKey(operation: string): string {
  const requestID = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
  return `${operation}-${requestID}`
}
