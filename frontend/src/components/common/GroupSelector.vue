<template>
  <div>
    <label class="input-label">
      {{ t('admin.users.groups') }}
      <span class="font-normal text-gray-400">{{ t('common.selectedCount', { count: visibleSelectedCount }) }}</span>
    </label>
    <div
      class="grid max-h-48 grid-cols-1 gap-1 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-2 md:grid-cols-2 dark:border-dark-600 dark:bg-dark-800"
    >
      <label
        v-for="group in filteredGroups"
        :key="group.id"
        class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 transition-colors hover:bg-white dark:hover:bg-dark-700"
        :title="group.rate_multiplier == null ? group.name : t('admin.groups.rateAndAccounts', { rate: group.rate_multiplier, count: group.account_count || 0 })"
      >
        <input
          type="checkbox"
          :value="group.id"
          :checked="modelValue.includes(group.id)"
          @change="handleChange(group.id, ($event.target as HTMLInputElement).checked)"
          class="mt-0.5 h-3.5 w-3.5 shrink-0 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
        />
        <GroupBadge
          :name="group.name"
          :platform="group.platform"
          :scope="group.scope"
          :subscription-type="group.subscription_type || undefined"
          :rate-multiplier="group.rate_multiplier == null ? undefined : group.rate_multiplier"
          :truncate-name="false"
          class="min-w-0 flex-1"
        />
      </label>
      <div
        v-if="filteredGroups.length === 0"
        class="col-span-2 py-2 text-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('common.noGroupsAvailable') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import GroupBadge from './GroupBadge.vue'
import type { Group, GroupPlatform } from '@/types'
import { COMPOSITE_ROUTE_TARGET_OPTIONS } from '@/constants/platforms'
import { useAuthStore } from '@/stores'

const { t } = useI18n()
const authStore = useAuthStore()

interface Props {
  modelValue: number[]
  groups: (Group & { account_count?: number })[]
  platform?: GroupPlatform // Optional platform filter
  mixedScheduling?: boolean // For antigravity accounts: allow anthropic/gemini groups
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

// Filter groups by platform if specified
const filteredGroups = computed(() => {
  const base = authStore.isSimpleMode
    ? props.groups.filter((g) => g.platform !== 'composite')
    : props.groups
  if (!props.platform) {
    return base
  }
  // antigravity 账户启用混合调度后，可选择 anthropic/gemini 分组
  if (props.platform === 'antigravity' && props.mixedScheduling) {
    return base.filter(
      (g) => g.platform === 'antigravity' || g.platform === 'anthropic' || g.platform === 'gemini' || g.platform === 'composite'
    )
  }
  // 默认：只能选择同 platform 的分组；能作为复合路由目标的平台（含国产平台与
  // OpenCode）额外可以选择 composite 分组，composite 分组按模型路由挑账号。
  const supportsComposite = COMPOSITE_ROUTE_TARGET_OPTIONS.some(
    (option) => option.value === props.platform
  )
  return base.filter(
    (g) => g.platform === props.platform || (supportsComposite && g.platform === 'composite')
  )
})

const visibleSelectedCount = computed(() => {
  const visibleGroupIDs = new Set(filteredGroups.value.map((group) => group.id))
  return props.modelValue.filter((groupID) => visibleGroupIDs.has(groupID)).length
})

watch(
  () => [authStore.isSimpleMode, props.groups, props.modelValue] as const,
  () => {
    if (!authStore.isSimpleMode || props.groups.length === 0) return
    const visibleIDs = new Set(props.groups.filter((group) => group.platform !== 'composite').map((group) => group.id))
    const cleaned = props.modelValue.filter((id) => visibleIDs.has(id))
    if (cleaned.length !== props.modelValue.length) emit('update:modelValue', cleaned)
  },
  { immediate: true, deep: true }
)

const handleChange = (groupId: number, checked: boolean) => {
  const newValue = checked
    ? [...props.modelValue, groupId]
    : props.modelValue.filter((id) => id !== groupId)
  emit('update:modelValue', newValue)
}
</script>
