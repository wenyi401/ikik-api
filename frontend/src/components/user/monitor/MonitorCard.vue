<template>
  <button type="button" class="monitor-row" @click="emit('click')">
    <div class="monitor-identity">
      <span class="monitor-provider-icon">
        <ProviderIcon :provider="item.provider" :size="18" />
      </span>
      <div class="min-w-0">
        <h3>{{ item.name }}</h3>
        <p>
          {{ providerLabel(item.provider) }}
          <span aria-hidden="true">·</span>
          <span class="font-mono">{{ item.primary_model }}</span>
          <template v-if="item.group_name">
            <span aria-hidden="true">·</span>
            {{ item.group_name }}
          </template>
        </p>
      </div>
    </div>

    <div class="monitor-metrics">
      <div class="monitor-metric">
        <span>{{ t('monitorCommon.dialogLatency') }}</span>
        <strong>{{ formatLatency(item.primary_latency_ms) }} <small>ms</small></strong>
      </div>
      <div class="monitor-metric">
        <span>{{ t('monitorCommon.endpointPing') }}</span>
        <strong>{{ formatLatency(item.primary_ping_latency_ms) }} <small>ms</small></strong>
      </div>
      <div class="monitor-metric">
        <span>{{ availabilityLabel }}</span>
        <strong>{{ availabilityDisplay }}<small v-if="availabilityValue != null">%</small></strong>
      </div>
    </div>

    <div class="monitor-row-timeline">
      <MonitorTimeline
        :buckets="item.timeline"
        :countdown-seconds="countdownSeconds"
        :length="30"
        compact
      />
    </div>

    <span :class="['monitor-status', statusToneClass]">
      <i />
      {{ statusLabel(item.primary_status) }}
    </span>

    <Icon name="chevronRight" size="sm" class="monitor-chevron" />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserMonitorView } from '@/api/channelMonitor'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import ProviderIcon from './ProviderIcon.vue'
import MonitorTimeline from './MonitorTimeline.vue'

const props = defineProps<{
  item: UserMonitorView
  window: '7d' | '15d' | '30d'
  availabilityValue: number | null
  countdownSeconds: number
}>()
const emit = defineEmits<{ (event: 'click'): void }>()
const { t } = useI18n()
const { statusLabel, providerLabel, formatLatency } = useChannelMonitorFormat()

const availabilityLabel = computed(() => (
  `${t('monitorCommon.availabilityPrefix')} · ${t(`channelStatus.windowTab.${props.window}`)}`
))
const availabilityDisplay = computed(() => (
  props.availabilityValue == null || Number.isNaN(props.availabilityValue)
    ? t('monitorCommon.latencyEmpty')
    : props.availabilityValue.toFixed(2)
))
const statusToneClass = computed(() => {
  if (props.item.primary_status === 'operational') return 'monitor-status--success'
  if (props.item.primary_status === 'degraded') return 'monitor-status--warning'
  return 'monitor-status--danger'
})
</script>

<style scoped>
.monitor-row {
  display: grid;
  width: 100%;
  min-width: 0;
  grid-template-columns: minmax(16rem, 1fr) minmax(18rem, 0.9fr) minmax(14rem, 0.7fr) auto 1rem;
  align-items: center;
  gap: 1.25rem;
  padding: 0.875rem 1rem;
  background: var(--ui-surface);
  color: var(--ui-text);
  text-align: left;
  transition: background-color 140ms ease;
}

.monitor-row:not(:last-child) {
  border-bottom: 1px solid var(--ui-border);
}

.monitor-row:hover {
  background: var(--ui-surface-subtle);
}

.monitor-row:focus-visible {
  position: relative;
  z-index: 1;
  outline: 2px solid var(--ui-text);
  outline-offset: -2px;
}

.monitor-identity {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}

.monitor-provider-icon {
  display: flex;
  width: 2.25rem;
  height: 2.25rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
}

.monitor-identity h3 {
  overflow: hidden;
  font-size: 0.875rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.monitor-identity p {
  display: flex;
  min-width: 0;
  gap: 0.3rem;
  margin-top: 0.2rem;
  overflow: hidden;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.monitor-metrics {
  display: grid;
  min-width: 0;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
}

.monitor-metric {
  min-width: 0;
}

.monitor-metric span {
  display: block;
  overflow: hidden;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.monitor-metric strong {
  display: block;
  margin-top: 0.2rem;
  color: var(--ui-text);
  font-family: var(--ui-font-mono);
  font-size: 0.8125rem;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.monitor-metric small {
  color: var(--ui-text-tertiary);
  font-size: 0.625rem;
  font-weight: 400;
}

.monitor-row-timeline {
  min-width: 0;
}

.monitor-status {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.6875rem;
  font-weight: 500;
  white-space: nowrap;
}

.monitor-status i {
  width: 0.4rem;
  height: 0.4rem;
  border-radius: 50%;
  background: currentColor;
}

.monitor-status--success {
  color: var(--ui-success);
}

.monitor-status--warning {
  color: var(--ui-warning);
}

.monitor-status--danger {
  color: var(--ui-danger);
}

.monitor-chevron {
  color: var(--ui-text-tertiary);
}

@media (max-width: 1100px) {
  .monitor-row {
    grid-template-columns: minmax(15rem, 1fr) minmax(18rem, 1fr) auto 1rem;
  }

  .monitor-row-timeline {
    display: none;
  }
}

@media (max-width: 720px) {
  .monitor-row {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.75rem;
    padding: 0.875rem;
  }

  .monitor-metrics,
  .monitor-row-timeline {
    grid-column: 1 / -1;
  }

  .monitor-row-timeline {
    display: block;
  }

  .monitor-status {
    grid-column: 2;
    grid-row: 1;
  }

  .monitor-chevron {
    display: none;
  }
}
</style>
