import type { Group } from '@/types'

type AccountAssignableGroup = Pick<Group, 'name' | 'scope' | 'subscription_type' | 'is_exclusive'>

const carpoolInternalGroupNamePattern = /^Carpool\s+\d+\s+·\s+/

export function isCarpoolInternalGroup(group: AccountAssignableGroup | null | undefined): boolean {
  if (!group) return false
  return (
    group.scope === 'public' &&
    group.subscription_type === 'subscription' &&
    group.is_exclusive === true &&
    carpoolInternalGroupNamePattern.test(group.name.trim())
  )
}

export function accountAssignableGroups<T extends AccountAssignableGroup>(groups: T[]): T[] {
  return groups.filter((group) => !isCarpoolInternalGroup(group))
}
