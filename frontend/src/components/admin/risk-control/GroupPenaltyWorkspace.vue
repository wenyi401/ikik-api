<template>
  <section class="penalty-workspace">
    <header class="penalty-header">
      <div>
        <div class="flex items-center gap-2">
          <h2 class="text-base font-semibold text-[var(--app-text)]">{{ t('admin.riskControl.penalties.title') }}</h2>
          <span v-if="overview.active > 0" class="penalty-count">{{ overview.active }}</span>
        </div>
        <p class="mt-1 text-sm text-[var(--app-muted)]">{{ t('admin.riskControl.penalties.subtitle') }}</p>
      </div>
      <UiIconButton :label="t('admin.riskControl.penalties.refresh')" :disabled="loading" @click="loadPenalties">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
      </UiIconButton>
    </header>

    <div class="penalty-summary">
      <div v-for="item in summaryItems" :key="item.key" class="penalty-summary-item">
        <span class="text-xs text-[var(--app-muted)]">{{ item.label }}</span>
        <strong class="mt-1 block text-xl font-semibold" :class="item.valueClass">{{ item.value }}</strong>
      </div>
    </div>

    <div class="penalty-toolbar">
      <div class="relative min-w-0 flex-1 lg:max-w-sm">
        <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--app-muted)]" />
        <input
          v-model="filters.search"
          type="search"
          class="input w-full pl-9"
          :placeholder="t('admin.riskControl.penalties.searchPlaceholder')"
          @keyup.enter="applyFilters"
        />
      </div>
      <div class="w-full sm:w-40">
        <Select v-model="filters.status" :options="statusOptions" @change="applyFilters" />
      </div>
      <div class="w-full sm:w-52">
        <Select v-model="filters.groupID" :options="groupOptions" searchable @change="applyFilters" />
      </div>
      <div class="w-full sm:w-60">
        <Select v-model="filters.category" :options="categoryOptions" searchable @change="applyFilters" />
      </div>
    </div>

    <div v-if="loading && penalties.length === 0" class="flex min-h-52 items-center justify-center">
      <div class="h-7 w-7 animate-spin rounded-full border-2 border-gray-200 border-t-gray-700 dark:border-dark-600 dark:border-t-gray-200"></div>
    </div>
    <div v-else-if="penalties.length === 0" class="penalty-empty">
      {{ t('admin.riskControl.penalties.empty') }}
    </div>

    <template v-else>
      <div class="hidden overflow-x-auto lg:block">
        <table class="w-full table-fixed">
          <thead>
            <tr class="border-b border-[var(--ui-border)] text-left text-xs font-medium text-[var(--app-muted)]">
              <th class="w-[21%] px-5 py-3">{{ t('admin.riskControl.penalties.user') }}</th>
              <th class="w-[18%] px-3 py-3">{{ t('admin.riskControl.penalties.group') }}</th>
              <th class="w-[22%] px-3 py-3">{{ t('admin.riskControl.penalties.category') }}</th>
              <th class="w-[13%] px-3 py-3">{{ t('admin.riskControl.penalties.strikes') }}</th>
              <th class="w-[14%] px-3 py-3">{{ t('admin.riskControl.penalties.status') }}</th>
              <th class="w-[12%] px-5 py-3 text-right">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in penalties" :key="`${item.user_id}:${item.group_id}`" class="border-b border-[var(--ui-border)] last:border-b-0">
              <td class="px-5 py-4">
                <p class="truncate text-sm font-medium text-[var(--app-text)]">{{ item.user_email || item.username || `#${item.user_id}` }}</p>
                <p class="mt-1 text-xs text-[var(--app-muted)]">ID {{ item.user_id }}</p>
              </td>
              <td class="px-3 py-4">
                <p class="truncate text-sm text-[var(--app-text)]" :title="item.group_name">{{ item.group_name || `#${item.group_id}` }}</p>
                <p class="mt-1 text-xs text-[var(--app-muted)]">{{ item.group_platform }} / ID {{ item.group_id }}</p>
              </td>
              <td class="px-3 py-4">
                <p class="truncate text-sm text-[var(--app-text)]" :title="categoryLabel(item.last_category)">{{ categoryLabel(item.last_category) }}</p>
                <p class="mt-1 text-xs text-[var(--app-muted)]">{{ formatPercent(item.last_score) }}</p>
              </td>
              <td class="px-3 py-4">
                <div class="strike-track" :aria-label="t('admin.riskControl.penalties.strikeValue', { count: item.strike_count })">
                  <span v-for="index in 3" :key="index" :class="index <= item.strike_count ? 'strike-filled' : ''"></span>
                </div>
                <p class="mt-1.5 text-xs text-[var(--app-muted)]">{{ t('admin.riskControl.penalties.strikeValue', { count: item.strike_count }) }}</p>
              </td>
              <td class="px-3 py-4">
                <span class="penalty-status" :class="statusClass(item)">{{ statusLabel(item) }}</span>
                <p class="mt-1.5 truncate text-xs text-[var(--app-muted)]" :title="item.blocked_until ? formatDate(item.blocked_until) : ''">
                  {{ item.permanent ? t('admin.riskControl.penalties.noExpiry') : formatDate(item.blocked_until) }}
                </p>
              </td>
              <td class="px-5 py-4">
                <div class="flex justify-end gap-1">
                  <UiIconButton :label="t('admin.riskControl.penalties.viewHistory')" @click="openHistory(item)">
                    <Icon name="document" size="xs" />
                  </UiIconButton>
                  <UiIconButton
                    :label="t('admin.riskControl.penalties.release')"
                    :disabled="!item.active || actingKey === penaltyKey(item)"
                    @click="releasePenalty(item)"
                  >
                    <Icon name="checkCircle" size="xs" />
                  </UiIconButton>
                  <UiIconButton
                    :label="t('admin.riskControl.penalties.reset')"
                    :disabled="actingKey === penaltyKey(item)"
                    @click="resetPenalty(item)"
                  >
                    <Icon name="trash" size="xs" />
                  </UiIconButton>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="divide-y divide-[var(--ui-border)] lg:hidden">
        <article v-for="item in penalties" :key="`${item.user_id}:${item.group_id}`" class="p-4">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-[var(--app-text)]">{{ item.user_email || item.username || `#${item.user_id}` }}</p>
              <p class="mt-1 truncate text-xs text-[var(--app-muted)]">{{ item.group_name || `#${item.group_id}` }}</p>
            </div>
            <span class="penalty-status shrink-0" :class="statusClass(item)">{{ statusLabel(item) }}</span>
          </div>
          <p class="mt-3 text-sm text-[var(--app-text)]">{{ categoryLabel(item.last_category) }}</p>
          <div class="mt-3 flex items-center justify-between gap-3 text-xs text-[var(--app-muted)]">
            <span>{{ t('admin.riskControl.penalties.strikeValue', { count: item.strike_count }) }}</span>
            <span>{{ item.permanent ? t('admin.riskControl.penalties.noExpiry') : formatDate(item.blocked_until) }}</span>
          </div>
          <div class="mt-4 flex justify-end gap-2">
            <button type="button" class="btn btn-secondary btn-sm" @click="openHistory(item)">{{ t('admin.riskControl.penalties.viewHistory') }}</button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="!item.active || actingKey === penaltyKey(item)" @click="releasePenalty(item)">{{ t('admin.riskControl.penalties.release') }}</button>
            <button type="button" class="btn btn-danger btn-sm" :disabled="actingKey === penaltyKey(item)" @click="resetPenalty(item)">{{ t('admin.riskControl.penalties.reset') }}</button>
          </div>
        </article>
      </div>

      <div v-if="pagination.total > pagination.page_size" class="border-t border-[var(--ui-border)] px-4 py-3">
        <Pagination :total="pagination.total" :page="pagination.page" :page-size="pagination.page_size" :show-page-size-selector="false" @update:page="changePage" />
      </div>
    </template>

    <BaseDialog :show="historyOpen" :title="t('admin.riskControl.penalties.historyTitle')" width="wide" @close="closeHistory">
      <div v-if="historyLoading" class="flex min-h-40 items-center justify-center">
        <div class="h-7 w-7 animate-spin rounded-full border-2 border-gray-200 border-t-gray-700 dark:border-dark-600 dark:border-t-gray-200"></div>
      </div>
      <div v-else-if="history.length === 0" class="py-12 text-center text-sm text-[var(--app-muted)]">{{ t('admin.riskControl.penalties.historyEmpty') }}</div>
      <div v-else class="divide-y divide-[var(--ui-border)]">
        <div v-for="event in history" :key="event.id" class="grid gap-2 py-4 sm:grid-cols-[160px_minmax(0,1fr)_90px] sm:items-center">
          <span class="text-xs text-[var(--app-muted)]">{{ formatDate(event.created_at) }}</span>
          <div class="min-w-0">
            <p class="truncate text-sm font-medium text-[var(--app-text)]">{{ categoryLabel(event.category) }}</p>
            <p class="mt-1 truncate font-mono text-xs text-[var(--app-muted)]" :title="event.request_id">{{ event.request_id }}</p>
          </div>
          <span class="text-right text-sm font-semibold text-[var(--app-text)]">{{ formatPercent(event.score) }}</span>
        </div>
      </div>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ContentModerationGroupPenalty, ContentModerationGroupPenaltyEvent, ContentModerationGroupPenaltyOverview } from '@/api/admin/riskControl'
import type { AdminGroup } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { UiIconButton } from '@/ui'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const actingKey = ref('')
const penalties = ref<ContentModerationGroupPenalty[]>([])
const groups = ref<AdminGroup[]>([])
const overview = ref<ContentModerationGroupPenaltyOverview>({ total: 0, active: 0, expired: 0, permanent: 0, today_events: 0 })
const filters = reactive({ search: '', status: 'active', groupID: 0, category: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0, pages: 1 })
const historyOpen = ref(false)
const historyLoading = ref(false)
const history = ref<ContentModerationGroupPenaltyEvent[]>([])

const categoryLabels: Record<string, string> = {
  'gateway_abuse/safety_bypass': '安全绕过 / Safety bypass',
  'gateway_abuse/credential_theft': '凭据窃取 / Credential theft',
  'gateway_abuse/account_automation': '第三方脚本 / Third-party scripting',
  'gateway_abuse/auth_reverse_engineering': '认证逆向 / Authentication reverse engineering',
  'gateway_abuse/exploit_reverse_engineering': '漏洞逆向 / Exploit reverse engineering',
  'gateway_abuse/cheat_automation': '外挂自动化 / Cheat automation',
  'policy/cyber_abuse': '网络攻击或暴力破解 / Cyber abuse or brute force',
  'policy/violence_terrorism_or_hate': '暴力、恐怖或仇恨 / Violence, terrorism or hate',
  'policy/weapons': '武器相关 / Weapons',
}

const summaryItems = computed(() => [
  { key: 'active', label: t('admin.riskControl.penalties.active'), value: overview.value.active, valueClass: 'text-red-600 dark:text-red-300' },
  { key: 'expired', label: t('admin.riskControl.penalties.expired'), value: overview.value.expired, valueClass: 'text-[var(--app-text)]' },
  { key: 'permanent', label: t('admin.riskControl.penalties.permanent'), value: overview.value.permanent, valueClass: 'text-orange-600 dark:text-orange-300' },
  { key: 'today', label: t('admin.riskControl.penalties.todayEvents'), value: overview.value.today_events, valueClass: 'text-[var(--app-text)]' },
])
const statusOptions = computed(() => [
  { value: 'all', label: t('admin.riskControl.penalties.allStatuses') },
  { value: 'active', label: t('admin.riskControl.penalties.active') },
  { value: 'expired', label: t('admin.riskControl.penalties.expired') },
  { value: 'permanent', label: t('admin.riskControl.penalties.permanent') },
])
const groupOptions = computed(() => [
  { value: 0, label: t('admin.riskControl.penalties.allGroups') },
  ...groups.value.map((group) => ({ value: group.id, label: `${group.name} (#${group.id})` })),
])
const categoryOptions = computed(() => [
  { value: '', label: t('admin.riskControl.penalties.allCategories') },
  ...Object.entries(categoryLabels).map(([value, label]) => ({ value, label })),
])

async function loadPenalties() {
  loading.value = true
  try {
    const result = await adminAPI.riskControl.listGroupPenalties({
      page: pagination.page,
      page_size: pagination.page_size,
      status: filters.status as 'all' | 'active' | 'expired' | 'permanent',
      search: filters.search.trim() || undefined,
      category: filters.category || undefined,
      group_id: filters.groupID || undefined,
    })
    penalties.value = result.items
    overview.value = result.overview
    Object.assign(pagination, { total: result.total, page: result.page, page_size: result.page_size, pages: result.pages })
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.riskControl.penalties.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function loadGroups() {
  try { groups.value = await adminAPI.groups.getAll() } catch { groups.value = [] }
}
function applyFilters() { pagination.page = 1; void loadPenalties() }
function changePage(page: number) { pagination.page = page; void loadPenalties() }
function penaltyKey(item: ContentModerationGroupPenalty) { return `${item.user_id}:${item.group_id}` }

async function openHistory(item: ContentModerationGroupPenalty) {
  historyOpen.value = true
  historyLoading.value = true
  history.value = []
  try {
    history.value = (await adminAPI.riskControl.listGroupPenaltyEvents(item.user_id, item.group_id)).items
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.riskControl.penalties.historyFailed')))
  } finally {
    historyLoading.value = false
  }
}
function closeHistory() { historyOpen.value = false; history.value = [] }

async function releasePenalty(item: ContentModerationGroupPenalty) {
  if (!window.confirm(t('admin.riskControl.penalties.releaseConfirm', { user: item.user_email || item.user_id, group: item.group_name || item.group_id }))) return
  actingKey.value = penaltyKey(item)
  try {
    await adminAPI.riskControl.releaseGroupPenalty(item.user_id, item.group_id)
    appStore.showSuccess(t('admin.riskControl.penalties.released'))
    await loadPenalties()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.riskControl.penalties.actionFailed')))
  } finally { actingKey.value = '' }
}

async function resetPenalty(item: ContentModerationGroupPenalty) {
  if (!window.confirm(t('admin.riskControl.penalties.resetConfirm', { user: item.user_email || item.user_id, group: item.group_name || item.group_id }))) return
  actingKey.value = penaltyKey(item)
  try {
    await adminAPI.riskControl.resetGroupPenalty(item.user_id, item.group_id)
    appStore.showSuccess(t('admin.riskControl.penalties.resetDone'))
    await loadPenalties()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.riskControl.penalties.actionFailed')))
  } finally { actingKey.value = '' }
}

function categoryLabel(category: string) { return categoryLabels[category] || category || '-' }
function statusLabel(item: ContentModerationGroupPenalty) {
  if (item.permanent) return t('admin.riskControl.penalties.permanent')
  return item.active ? t('admin.riskControl.penalties.active') : t('admin.riskControl.penalties.expired')
}
function statusClass(item: ContentModerationGroupPenalty) {
  if (item.permanent) return 'bg-orange-50 text-orange-700 dark:bg-orange-950/40 dark:text-orange-300'
  if (item.active) return 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}
function formatDate(value?: string) { return value ? formatDateTime(value) : '-' }
function formatPercent(value: number) { return `${Math.round((value || 0) * 100)}%` }

onMounted(() => { void Promise.all([loadGroups(), loadPenalties()]) })
</script>

<style scoped>
.penalty-workspace { overflow: hidden; border: 1px solid var(--ui-border); border-radius: 8px; background: var(--app-surface); }
.penalty-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; padding: 1.25rem; border-bottom: 1px solid var(--ui-border); }
.penalty-count { display: inline-flex; min-width: 1.5rem; height: 1.5rem; align-items: center; justify-content: center; border-radius: 999px; padding: 0 0.45rem; background: #dc2626; color: white; font-size: 0.75rem; font-weight: 700; }
.penalty-summary { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border-bottom: 1px solid var(--ui-border); }
.penalty-summary-item { min-width: 0; padding: 1rem 1.25rem; border-right: 1px solid var(--ui-border); }
.penalty-summary-item:last-child { border-right: 0; }
.penalty-toolbar { display: flex; align-items: center; gap: 0.75rem; padding: 1rem 1.25rem; border-bottom: 1px solid var(--ui-border); }
.penalty-empty { padding: 4rem 1.25rem; text-align: center; color: var(--app-muted); }
.penalty-status { display: inline-flex; min-height: 1.5rem; align-items: center; border-radius: 999px; padding: 0.125rem 0.5rem; font-size: 0.75rem; font-weight: 600; white-space: nowrap; }
.strike-track { display: flex; gap: 0.3rem; }
.strike-track span { width: 1.1rem; height: 0.3rem; border-radius: 999px; background: var(--app-surface-muted); }
.strike-track .strike-filled { background: #dc2626; }
@media (max-width: 1023px) {
  .penalty-toolbar { align-items: stretch; flex-wrap: wrap; }
}
@media (max-width: 639px) {
  .penalty-header { padding: 1rem; }
  .penalty-summary { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .penalty-summary-item:nth-child(2) { border-right: 0; }
  .penalty-summary-item:nth-child(-n + 2) { border-bottom: 1px solid var(--ui-border); }
  .penalty-toolbar { flex-direction: column; padding: 1rem; }
}
</style>
