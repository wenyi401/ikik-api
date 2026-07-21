type LocaleRecord = Record<string, unknown>

function isRecord(value: unknown): value is LocaleRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function mergeMissingLocaleMessages(
  base: LocaleRecord,
  extension: LocaleRecord,
): LocaleRecord {
  const merged: LocaleRecord = { ...base }

  for (const [key, extensionValue] of Object.entries(extension)) {
    const baseValue = merged[key]
    if (!(key in merged)) {
      merged[key] = extensionValue
      continue
    }
    if (isRecord(baseValue) && isRecord(extensionValue)) {
      merged[key] = mergeMissingLocaleMessages(baseValue, extensionValue)
    }
  }

  return merged
}
