<template>
  <section class="monitor-toolbar">
    <div role="tablist" class="monitor-windows">
        <button
          v-for="opt in windowOptions"
          :key="opt.value"
          type="button"
          role="tab"
          :aria-selected="window === opt.value"
          :class="['monitor-window', { 'monitor-window--active': window === opt.value }]"
          @click="emit('update:window', opt.value)"
        >
          {{ opt.label }}
        </button>
    </div>

    <div class="monitor-actions">
      <span
        class="monitor-overall"
        :class="overallToneClass"
      >
        <span />
        {{ overallLabel }}
      </span>

      <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="emit('refresh')">
        <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
      </UiIconButton>

      <AutoRefreshButton
        v-if="autoRefresh"
        :enabled="autoRefresh.enabled.value"
        :interval-seconds="autoRefresh.intervalSeconds.value"
        :countdown="autoRefresh.countdown.value"
        :intervals="autoRefresh.intervals"
        @update:enabled="autoRefresh.setEnabled"
        @update:interval="autoRefresh.setInterval"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import AutoRefreshButton from '@/components/common/AutoRefreshButton.vue'
import { UiIconButton } from '@/ui'
export type MonitorWindow = '7d' | '15d' | '30d'
export type OverallStatus = 'operational' | 'degraded' | 'constrained' | 'unavailable'

const props = defineProps<{
  overallStatus: OverallStatus
  intervalSeconds: number
  window: MonitorWindow
  loading: boolean
  autoRefresh?: {
    enabled: { value: boolean }
    intervalSeconds: { value: number }
    countdown: { value: number }
    intervals: readonly number[]
    setEnabled: (v: boolean) => void
    setInterval: (v: number) => void
  }
}>()

const emit = defineEmits<{
  (e: 'update:window', value: MonitorWindow): void
  (e: 'refresh'): void
}>()

const { t } = useI18n()

const windowOptions = computed<{ value: MonitorWindow; label: string }[]>(() => [
  { value: '7d', label: t('channelStatus.windowTab.7d') },
  { value: '15d', label: t('channelStatus.windowTab.15d') },
  { value: '30d', label: t('channelStatus.windowTab.30d') },
])

const overallLabel = computed(() => t(`channelStatus.overall.${props.overallStatus}`))

const overallToneClass = computed(() => {
  switch (props.overallStatus) {
    case 'operational':
      return 'monitor-overall--success'
    case 'degraded':
      return 'monitor-overall--warning'
    case 'constrained':
      return 'monitor-overall--warning'
    case 'unavailable':
    default:
      return 'monitor-overall--danger'
  }
})
</script>

<style scoped>
.monitor-toolbar {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--ui-border);
}

.monitor-windows,
.monitor-actions {
  display: flex;
  min-width: 0;
  align-items: center;
}

.monitor-windows {
  gap: 1.25rem;
}

.monitor-window {
  position: relative;
  min-height: 2.25rem;
  color: var(--ui-text-tertiary);
  font-size: 0.8125rem;
  font-weight: 500;
}

.monitor-window::after {
  position: absolute;
  right: 0;
  bottom: -0.8125rem;
  left: 0;
  height: 2px;
  background: transparent;
  content: '';
}

.monitor-window:hover,
.monitor-window--active {
  color: var(--ui-text);
}

.monitor-window--active::after {
  background: var(--ui-text);
}

.monitor-actions {
  flex: 0 0 auto;
  gap: 0.625rem;
}

.monitor-overall {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.75rem;
  font-weight: 500;
  white-space: nowrap;
}

.monitor-overall span {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: currentColor;
}

.monitor-overall--success {
  color: var(--ui-success);
}

.monitor-overall--warning {
  color: var(--ui-warning);
}

.monitor-overall--danger {
  color: var(--ui-danger);
}

@media (max-width: 640px) {
  .monitor-toolbar {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.625rem;
  }

  .monitor-actions {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
