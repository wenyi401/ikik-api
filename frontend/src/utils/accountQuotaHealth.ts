import type { AccountQuotaGroupSummary } from '@/types'

export type AccountQuotaGroupHealth = 'normal' | 'degraded' | 'constrained' | 'unavailable'

export function resolveAccountQuotaGroupHealth(summary: AccountQuotaGroupSummary): AccountQuotaGroupHealth {
  if (summary.group_status && summary.group_status !== 'active') return 'unavailable'
  if (summary.account_count > 0 && summary.schedulable_account_count === 0) return 'unavailable'

  const supportHours = fiveHourSupportHours(summary)
  if (supportHours != null && supportHours < 3) return 'constrained'
  if (supportHours != null && supportHours < 5) return 'degraded'
  return 'normal'
}

export function accountQuotaGroupHealthRank(status: AccountQuotaGroupHealth): number {
  if (status === 'unavailable') return 3
  if (status === 'constrained') return 2
  if (status === 'degraded') return 1
  return 0
}

function fiveHourSupportHours(summary: AccountQuotaGroupSummary): number | null {
  const window = summary.usage_windows?.find(item => item.window === '5h')
  const hours = window?.estimated_support_hours
  return typeof hours === 'number' && Number.isFinite(hours) ? hours : null
}
