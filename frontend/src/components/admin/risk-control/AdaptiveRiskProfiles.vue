<template>
  <section class="risk-profiles">
    <header class="risk-profiles-header">
      <div>
        <div class="flex items-center gap-2">
          <h2 class="text-base font-semibold text-[var(--app-text)]">{{ t('admin.riskControl.riskProfiles.title') }}</h2>
          <span class="risk-mode-badge">{{ t('admin.riskControl.modeAdaptive') }}</span>
        </div>
        <p class="mt-1 text-sm text-[var(--app-muted)]">{{ t('admin.riskControl.riskProfiles.subtitle') }}</p>
      </div>
      <UiIconButton :label="t('admin.riskControl.riskProfiles.refresh')" :disabled="loading" @click="loadProfiles">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
      </UiIconButton>
    </header>

    <div class="risk-summary">
      <div v-for="item in summaryItems" :key="item.key" class="risk-summary-item">
        <span class="text-xs text-[var(--app-muted)]">{{ item.label }}</span>
        <strong class="mt-1 block text-xl font-semibold text-[var(--app-text)]">{{ item.value }}</strong>
      </div>
    </div>

    <div class="risk-toolbar">
      <div class="relative min-w-0 flex-1 sm:max-w-sm">
        <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--app-muted)]" />
        <input
          v-model="search"
          type="search"
          class="input w-full pl-9"
          :placeholder="t('admin.riskControl.riskProfiles.searchPlaceholder')"
          @keyup.enter="applyFilters"
        />
      </div>
      <div class="w-full sm:w-44">
        <Select v-model="level" :options="levelFilterOptions" @change="applyFilters" />
      </div>
    </div>

    <div v-if="loading && profiles.length === 0" class="flex min-h-52 items-center justify-center">
      <div class="h-7 w-7 animate-spin rounded-full border-2 border-gray-200 border-t-gray-700 dark:border-dark-600 dark:border-t-gray-200"></div>
    </div>

    <div v-else-if="profiles.length === 0" class="risk-empty">
      {{ t('admin.riskControl.riskProfiles.empty') }}
    </div>

    <template v-else>
      <div class="hidden overflow-x-auto md:block">
        <table class="w-full table-fixed">
          <thead>
            <tr class="border-b border-[var(--ui-border)] text-left text-xs font-medium text-[var(--app-muted)]">
              <th class="w-[26%] px-5 py-3">{{ t('admin.riskControl.riskProfiles.user') }}</th>
              <th class="w-[14%] px-3 py-3">{{ t('admin.riskControl.riskProfiles.level') }}</th>
              <th class="w-[20%] px-3 py-3">{{ t('admin.riskControl.riskProfiles.auditProgress') }}</th>
              <th class="w-[15%] px-3 py-3">{{ t('admin.riskControl.riskProfiles.riskScore') }}</th>
              <th class="w-[13%] px-3 py-3">{{ t('admin.riskControl.riskProfiles.lastHit') }}</th>
              <th class="w-[12%] px-5 py-3 text-right">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="profile in profiles" :key="profile.user_id" class="border-b border-[var(--ui-border)] last:border-b-0">
              <td class="px-5 py-4">
                <p class="truncate text-sm font-medium text-[var(--app-text)]">{{ profile.user_email || `#${profile.user_id}` }}</p>
                <p class="mt-1 text-xs text-[var(--app-muted)]">ID {{ profile.user_id }} / {{ formatNumber(profile.total_requests) }}</p>
              </td>
              <td class="px-3 py-4">
                <span class="risk-level" :class="riskLevelClass(profile.risk_level)">{{ riskLevelLabel(profile.risk_level) }}</span>
                <p class="mt-1 text-xs text-[var(--app-muted)]">{{ profile.current_sample_rate }}%</p>
              </td>
              <td class="px-3 py-4">
                <div class="flex items-center justify-between gap-3 text-xs text-[var(--app-muted)]">
                  <span>{{ formatNumber(profile.audited_requests) }} / {{ formatNumber(profile.total_requests) }}</span>
                  <span>{{ auditCoverage(profile) }}%</span>
                </div>
                <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                  <div class="h-full rounded-full bg-gray-800 dark:bg-gray-100" :style="{ width: `${auditCoverage(profile)}%` }"></div>
                </div>
              </td>
              <td class="px-3 py-4">
                <div class="flex items-center justify-between gap-2 text-sm font-semibold text-[var(--app-text)]">
                  <span>{{ profile.risk_score.toFixed(1) }}</span>
                  <span class="text-xs font-normal text-[var(--app-muted)]">{{ profile.flagged_requests }}</span>
                </div>
                <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                  <div class="h-full rounded-full" :class="riskBarClass(profile.risk_score)" :style="{ width: `${Math.min(100, profile.risk_score)}%` }"></div>
                </div>
              </td>
              <td class="px-3 py-4">
                <p class="truncate text-xs text-[var(--app-text)]" :title="profile.last_category">{{ profile.last_category || '-' }}</p>
                <p class="mt-1 text-xs text-[var(--app-muted)]">{{ formatDate(profile.last_hit_at) }}</p>
              </td>
              <td class="px-5 py-4">
                <div class="ml-auto flex w-32 items-center gap-2">
                  <Select
                    :model-value="profile.manual_level"
                    :options="manualLevelOptions"
                    :disabled="updatingUserID === profile.user_id"
                    @update:model-value="updateManualLevel(profile, $event)"
                  />
                  <UiIconButton
                    :label="t('admin.riskControl.riskProfiles.resetScore')"
                    :disabled="updatingUserID === profile.user_id"
                    @click="resetScore(profile)"
                  >
                    <Icon name="refresh" size="xs" />
                  </UiIconButton>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="divide-y divide-[var(--ui-border)] md:hidden">
        <article v-for="profile in profiles" :key="profile.user_id" class="px-4 py-4">
          <div class="flex min-w-0 items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-[var(--app-text)]">{{ profile.user_email || `#${profile.user_id}` }}</p>
              <p class="mt-1 text-xs text-[var(--app-muted)]">ID {{ profile.user_id }}</p>
            </div>
            <span class="risk-level shrink-0" :class="riskLevelClass(profile.risk_level)">{{ riskLevelLabel(profile.risk_level) }}</span>
          </div>
          <div class="mt-4 grid grid-cols-3 gap-3">
            <div>
              <p class="text-xs text-[var(--app-muted)]">{{ t('admin.riskControl.riskProfiles.riskScore') }}</p>
              <p class="mt-1 text-base font-semibold text-[var(--app-text)]">{{ profile.risk_score.toFixed(1) }}</p>
            </div>
            <div>
              <p class="text-xs text-[var(--app-muted)]">{{ t('admin.riskControl.riskProfiles.audited') }}</p>
              <p class="mt-1 text-base font-semibold text-[var(--app-text)]">{{ formatNumber(profile.audited_requests) }}</p>
            </div>
            <div>
              <p class="text-xs text-[var(--app-muted)]">{{ t('admin.riskControl.riskProfiles.sampleRate') }}</p>
              <p class="mt-1 text-base font-semibold text-[var(--app-text)]">{{ profile.current_sample_rate }}%</p>
            </div>
          </div>
          <div class="mt-4 flex items-center gap-2">
            <div class="min-w-0 flex-1">
              <Select
                :model-value="profile.manual_level"
                :options="manualLevelOptions"
                :disabled="updatingUserID === profile.user_id"
                @update:model-value="updateManualLevel(profile, $event)"
              />
            </div>
            <UiIconButton
              :label="t('admin.riskControl.riskProfiles.resetScore')"
              :disabled="updatingUserID === profile.user_id"
              @click="resetScore(profile)"
            >
              <Icon name="refresh" size="sm" />
            </UiIconButton>
          </div>
        </article>
      </div>

      <div v-if="pagination.total > pagination.page_size" class="border-t border-[var(--ui-border)] px-4 py-3">
        <Pagination
          :total="pagination.total"
          :page="pagination.page"
          :page-size="pagination.page_size"
          :show-page-size-selector="false"
          @update:page="changePage"
        />
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  ContentModerationManualLevel,
  ContentModerationRiskLevel,
  ContentModerationRiskOverview,
  ContentModerationRiskProfile,
} from '@/api/admin/riskControl'
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
const updatingUserID = ref<number | null>(null)
const profiles = ref<ContentModerationRiskProfile[]>([])
const overview = ref<ContentModerationRiskOverview>({
  total_profiles: 0,
  new_profiles: 0,
  trusted_profiles: 0,
  watch_profiles: 0,
  high_profiles: 0,
  critical_profiles: 0,
  audited_requests: 0,
  flagged_requests: 0,
  average_risk_score: 0,
})
const search = ref('')
const level = ref('all')
const pagination = reactive({ page: 1, page_size: 20, total: 0, pages: 1 })

const levelFilterOptions = computed(() => [
  { value: 'all', label: t('admin.riskControl.riskProfiles.allLevels') },
  { value: 'new', label: t('admin.riskControl.riskLevels.new') },
  { value: 'normal', label: t('admin.riskControl.riskLevels.normal') },
  { value: 'trusted', label: t('admin.riskControl.riskLevels.trusted') },
  { value: 'watch', label: t('admin.riskControl.riskLevels.watch') },
  { value: 'high', label: t('admin.riskControl.riskLevels.high') },
  { value: 'critical', label: t('admin.riskControl.riskLevels.critical') },
])

const manualLevelOptions = computed(() => [
  { value: 'auto', label: t('admin.riskControl.riskProfiles.manualAuto') },
  { value: 'trusted', label: t('admin.riskControl.riskLevels.trusted') },
  { value: 'watch', label: t('admin.riskControl.riskLevels.watch') },
  { value: 'high', label: t('admin.riskControl.riskLevels.high') },
  { value: 'critical', label: t('admin.riskControl.riskLevels.critical') },
])

const summaryItems = computed(() => [
  { key: 'profiles', label: t('admin.riskControl.riskProfiles.totalUsers'), value: formatNumber(overview.value.total_profiles) },
  { key: 'audited', label: t('admin.riskControl.riskProfiles.totalAudited'), value: formatNumber(overview.value.audited_requests) },
  { key: 'watch', label: t('admin.riskControl.riskProfiles.needsAttention'), value: formatNumber(overview.value.watch_profiles + overview.value.high_profiles + overview.value.critical_profiles) },
  { key: 'score', label: t('admin.riskControl.riskProfiles.averageScore'), value: overview.value.average_risk_score.toFixed(1) },
])

async function loadProfiles() {
  loading.value = true
  try {
    const result = await adminAPI.riskControl.listRiskProfiles({
      page: pagination.page,
      page_size: pagination.page_size,
      level: level.value === 'all' ? undefined : level.value,
      search: search.value.trim() || undefined,
    })
    profiles.value = result.items
    overview.value = result.overview
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
    pagination.pages = result.pages
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.riskControl.riskProfiles.loadFailed')))
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  pagination.page = 1
  void loadProfiles()
}

function changePage(page: number) {
  pagination.page = page
  void loadProfiles()
}

async function updateManualLevel(profile: ContentModerationRiskProfile, value: unknown) {
  const manualLevel = String(value) as ContentModerationManualLevel
  updatingUserID.value = profile.user_id
  try {
    const updated = await adminAPI.riskControl.updateRiskProfile(profile.user_id, { manual_level: manualLevel })
    replaceProfile(updated)
    appStore.showSuccess(t('admin.riskControl.riskProfiles.updated'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.riskControl.riskProfiles.updateFailed')))
  } finally {
    updatingUserID.value = null
  }
}

async function resetScore(profile: ContentModerationRiskProfile) {
  if (!window.confirm(t('admin.riskControl.riskProfiles.resetConfirm'))) return
  updatingUserID.value = profile.user_id
  try {
    const updated = await adminAPI.riskControl.updateRiskProfile(profile.user_id, { reset_score: true })
    replaceProfile(updated)
    appStore.showSuccess(t('admin.riskControl.riskProfiles.resetDone'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.riskControl.riskProfiles.updateFailed')))
  } finally {
    updatingUserID.value = null
  }
}

function replaceProfile(updated: ContentModerationRiskProfile) {
  profiles.value = profiles.value.map((item) => item.user_id === updated.user_id ? updated : item)
}

function riskLevelLabel(value: ContentModerationRiskLevel): string {
  return t(`admin.riskControl.riskLevels.${value}`)
}

function riskLevelClass(value: ContentModerationRiskLevel): string {
  if (value === 'critical') return 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300'
  if (value === 'high') return 'bg-orange-50 text-orange-700 dark:bg-orange-950/40 dark:text-orange-300'
  if (value === 'watch') return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300'
  if (value === 'trusted') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function riskBarClass(score: number): string {
  if (score >= 80) return 'bg-red-500'
  if (score >= 60) return 'bg-orange-500'
  if (score >= 40) return 'bg-amber-500'
  return 'bg-emerald-500'
}

function auditCoverage(profile: ContentModerationRiskProfile): number {
  if (profile.total_requests <= 0) return 0
  return Math.min(100, Math.round((profile.audited_requests / profile.total_requests) * 100))
}

function formatDate(value?: string): string {
  return value ? formatDateTime(value) : '-'
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value || 0)
}

onMounted(loadProfiles)
</script>

<style scoped>
.risk-profiles {
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.risk-profiles-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.risk-mode-badge,
.risk-level {
  display: inline-flex;
  align-items: center;
  min-height: 1.5rem;
  border-radius: 999px;
  padding: 0.125rem 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}

.risk-mode-badge {
  background: #171717;
  color: #fff;
}

:global(.dark) .risk-mode-badge {
  background: #f4f4f5;
  color: #171717;
}

.risk-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  border-bottom: 1px solid var(--ui-border);
}

.risk-summary-item {
  min-width: 0;
  padding: 1rem 1.25rem;
  border-right: 1px solid var(--ui-border);
}

.risk-summary-item:last-child {
  border-right: 0;
}

.risk-toolbar {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.risk-empty {
  padding: 4rem 1.25rem;
  text-align: center;
  color: var(--app-muted);
}

@media (max-width: 767px) {
  .risk-profiles-header {
    padding: 1rem;
  }

  .risk-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .risk-summary-item:nth-child(2) {
    border-right: 0;
  }

  .risk-summary-item:nth-child(-n + 2) {
    border-bottom: 1px solid var(--ui-border);
  }

  .risk-toolbar {
    align-items: stretch;
    flex-direction: column;
    padding: 1rem;
  }
}
</style>
