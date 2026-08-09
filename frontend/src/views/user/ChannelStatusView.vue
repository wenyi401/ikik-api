<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <div class="monitor-guide-toolbar">
        <button
          v-if="returnPath"
          type="button"
          class="monitor-return-action"
          @click="returnToKeyCreation"
        >
          <Icon name="arrowLeft" size="sm" />
          {{ t('onboarding.monitor.returnToKey') }}
        </button>
        <UiIconButton :label="t('onboarding.pageHelp')" @click="startMonitorTutorial">
          <Icon name="questionCircle" size="md" />
        </UiIconButton>
      </div>

      <div data-guide="monitor-overview">
        <MonitorHero
          :overall-status="overallStatus"
          :interval-seconds="DEFAULT_INTERVAL_SECONDS"
          :window="currentWindow"
          :loading="loading"
          :auto-refresh="autoRefresh"
          @update:window="handleWindowChange"
          @refresh="manualReload"
        />
      </div>

      <div class="channel-quota-grid" data-guide="monitor-groups">
        <ChannelQuotaSummary
          :dashboard="quotaPoolDashboard?.platform ?? null"
          :loading="quotaPoolLoading"
          :error="quotaPoolError"
          :desktop-page-size="3"
          :mobile-page-size="3"
          prioritize-account-levels
          :title="t('channelStatus.quotaPool.platformTitle')"
          :empty-message="t('channelStatus.quotaPool.platformEmpty')"
          :load-failed-message="t('channelStatus.quotaPool.loadFailed')"
          @refresh="reloadQuotaPool(false)"
        />

        <ChannelQuotaSummary
          :dashboard="quotaPoolDashboard?.mine ?? null"
          :loading="quotaPoolLoading"
          :error="quotaPoolError"
          :title="t('channelStatus.quotaPool.mineTitle')"
          :empty-message="t('channelStatus.quotaPool.mineEmpty')"
          :load-failed-message="t('channelStatus.quotaPool.loadFailed')"
          @refresh="reloadQuotaPool(false)"
        />
      </div>

      <div data-guide="monitor-history">
        <MonitorCardGrid
          :items="items"
          :window="currentWindow"
          :countdown-seconds="countdown"
          :loading="loading"
          :detail-cache="detailCache"
          @card-click="openDetail"
        />
      </div>
    </UiPage>

    <MonitorDetailDialog
      :show="showDetail"
      :monitor-id="detailTarget?.id ?? null"
      :title="detailTitle"
      @close="closeDetail"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useOnboardingStore } from '@/stores/onboarding'
import { usePageTutorial } from '@/composables/usePageTutorial'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getQuotaDashboard as fetchQuotaPoolDashboard } from '@/api/accounts'
import {
  list as listChannelMonitorViews,
  status as fetchChannelMonitorDetail,
  type UserMonitorView,
  type UserMonitorDetail,
} from '@/api/channelMonitor'
import type { UserAccountQuotaPoolDashboard } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import { UiIconButton, UiPage } from '@/ui'
import Icon from '@/components/icons/Icon.vue'
import ChannelQuotaSummary from '@/components/user/monitor/ChannelQuotaSummary.vue'
import {
  accountQuotaGroupHealthRank,
  resolveAccountQuotaGroupHealth,
  type AccountQuotaGroupHealth,
} from '@/utils/accountQuotaHealth'
import MonitorHero, {
  type MonitorWindow,
  type OverallStatus,
} from '@/components/user/monitor/MonitorHero.vue'
import MonitorCardGrid from '@/components/user/monitor/MonitorCardGrid.vue'
import MonitorDetailDialog from '@/components/user/MonitorDetailDialog.vue'
import { DEFAULT_INTERVAL_SECONDS } from '@/constants/channelMonitor'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import type { DriveStep } from 'driver.js'

const { t } = useI18n()
const appStore = useAppStore()
const onboardingStore = useOnboardingStore()
const route = useRoute()
const router = useRouter()
const { startPageTutorial } = usePageTutorial()

// ── State ──
const items = ref<UserMonitorView[]>([])
const loading = ref(false)
const quotaPoolDashboard = ref<UserAccountQuotaPoolDashboard | null>(null)
const quotaPoolLoading = ref(false)
const quotaPoolError = ref(false)
const currentWindow = ref<MonitorWindow>('7d')
const detailCache = reactive<Record<number, UserMonitorDetail>>({})
const showDetail = ref(false)
const detailTarget = ref<UserMonitorView | null>(null)

let abortController: AbortController | null = null
let quotaPoolAbortController: AbortController | null = null

const autoRefresh = useAutoRefresh({
  storageKey: 'channel-status-auto-refresh',
  intervals: [30, 60, 120] as const,
  defaultInterval: DEFAULT_INTERVAL_SECONDS,
  onRefresh: () => reloadAll(true),
  shouldPause: () => document.hidden || loading.value || quotaPoolLoading.value,
})
const countdown = autoRefresh.countdown

// ── Computed ──
const overallStatus = computed<OverallStatus>(() => {
  let quotaStatus: AccountQuotaGroupHealth = 'normal'
  for (const summary of quotaPoolDashboard.value?.platform?.group_summaries ?? []) {
    const status = resolveAccountQuotaGroupHealth(summary)
    if (accountQuotaGroupHealthRank(status) > accountQuotaGroupHealthRank(quotaStatus)) {
      quotaStatus = status
    }
  }
  for (const it of items.value) {
    if (it.primary_status === 'failed' || it.primary_status === 'error') return 'unavailable'
  }
  return quotaStatus === 'normal' ? 'operational' : quotaStatus
})

const detailTitle = computed(() => {
  return detailTarget.value?.name || t('channelStatus.detailTitle')
})
const returnPath = computed(() => typeof route.query.return === 'string' ? route.query.return : '')

function clearGuideQuery(): void {
  if (!route.query.guide) return
  const query = { ...route.query }
  delete query.guide
  void router.replace({ path: route.path, query })
}

async function startMonitorTutorial(): Promise<void> {
  const steps: DriveStep[] = [{
    element: '[data-guide="monitor-overview"]',
    popover: {
      title: t('onboarding.monitor.tour.overviewTitle'),
      description: t('onboarding.monitor.tour.overviewDescription'),
      side: 'bottom',
      align: 'start'
    }
  }, {
    element: '[data-guide="monitor-groups"]',
    popover: {
      title: t('onboarding.monitor.tour.groupsTitle'),
      description: t('onboarding.monitor.tour.groupsDescription'),
      side: 'top',
      align: 'start'
    }
  }, {
    element: '[data-guide="monitor-history"]',
    popover: {
      title: t('onboarding.monitor.tour.historyTitle'),
      description: t('onboarding.monitor.tour.historyDescription'),
      side: 'top',
      align: 'start'
    }
  }]
  await startPageTutorial(steps, () => {
    onboardingStore.completeMission('monitor')
  })
  clearGuideQuery()
}

async function returnToKeyCreation(): Promise<void> {
  onboardingStore.completeMission('monitor')
  await router.push({
    path: returnPath.value || '/keys',
    query: {
      draft: route.query.draft === 'key' ? 'key' : undefined,
      guide: 'key'
    }
  })
}

// ── Loaders ──
async function reload(silent = false) {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  if (!silent) loading.value = true
  try {
    const res = await listChannelMonitorViews({ signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    items.value = res.items || []
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('channelStatus.loadError')))
  } finally {
    if (abortController === ctrl) {
      if (!silent) loading.value = false
      countdown.value = DEFAULT_INTERVAL_SECONDS
      abortController = null
    }
  }
}

async function reloadQuotaPool(silent = false) {
  if (quotaPoolAbortController) quotaPoolAbortController.abort()
  const ctrl = new AbortController()
  quotaPoolAbortController = ctrl
  if (!silent) quotaPoolLoading.value = true
  quotaPoolError.value = false
  try {
    const dashboard = await fetchQuotaPoolDashboard({ signal: ctrl.signal })
    if (ctrl.signal.aborted || quotaPoolAbortController !== ctrl) return
    quotaPoolDashboard.value = dashboard
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    quotaPoolError.value = true
    appStore.showError(extractApiErrorMessage(err, t('channelStatus.quotaPool.loadFailed')))
  } finally {
    if (quotaPoolAbortController === ctrl) {
      if (!silent) quotaPoolLoading.value = false
      quotaPoolAbortController = null
    }
  }
}

async function reloadAll(silent = false) {
  await Promise.all([
    reload(silent),
    reloadQuotaPool(silent)
  ])
}

async function manualReload() {
  await reloadAll(false)
  // After base reload, refresh any cached detail records so non-7d availability
  // values stay in sync without forcing the user to switch tabs again.
  if (currentWindow.value !== '7d') {
    await Promise.all(items.value.map(it => loadDetail(it.id, true)))
  }
}

async function loadDetail(id: number, force = false) {
  if (!force && detailCache[id]) return
  try {
    detailCache[id] = await fetchChannelMonitorDetail(id)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('channelStatus.detailLoadError')))
  }
}

async function ensureDetailsForWindow() {
  if (currentWindow.value === '7d') return
  await Promise.all(items.value.map(it => loadDetail(it.id)))
}

// ── Handlers ──
async function handleWindowChange(value: MonitorWindow) {
  currentWindow.value = value
  await ensureDetailsForWindow()
}

function openDetail(row: UserMonitorView) {
  detailTarget.value = row
  showDetail.value = true
}

function closeDetail() {
  showDetail.value = false
  detailTarget.value = null
}

watch(items, () => {
  void ensureDetailsForWindow()
})

watch(
  () => appStore.cachedPublicSettings?.channel_monitor_enabled,
  (enabled) => {
    if (enabled === false) autoRefresh.stop()
    else if (autoRefresh.enabled.value) autoRefresh.start()
  },
)

watch(
  () => route.query.guide,
  async (guide, previousGuide) => {
    if (guide === 'monitor' && guide !== previousGuide) {
      await startMonitorTutorial()
    }
  },
)

onMounted(async () => {
  await reloadAll(false)
  if (appStore.cachedPublicSettings?.channel_monitor_enabled !== false) {
    autoRefresh.setEnabled(true)
  }
  if (route.query.guide === 'monitor') {
    await startMonitorTutorial()
  }
})

onBeforeUnmount(() => {
  if (abortController) abortController.abort()
  if (quotaPoolAbortController) quotaPoolAbortController.abort()
})
</script>

<style scoped>
.channel-quota-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.monitor-guide-toolbar {
  display: flex;
  min-height: 2.5rem;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.monitor-return-action {
  display: inline-flex;
  min-height: 2.25rem;
  align-items: center;
  gap: 0.4rem;
  padding: 0.45rem 0.7rem;
  border: 1px solid var(--app-primary-border);
  border-radius: 6px;
  background: var(--app-primary-soft);
  color: var(--app-primary);
  font-size: 0.75rem;
  font-weight: 700;
  transition: border-color 180ms ease, background-color 180ms ease;
}

.monitor-return-action:hover {
  border-color: var(--app-primary);
  background: var(--app-surface);
}

@media (max-width: 1100px) {
  .channel-quota-grid {
    grid-template-columns: 1fr;
  }
}
</style>
