<template>
  <OfficialStatusCard
    card-id="openai-official-status"
    :title="t('serviceStatus.official.title')"
    :updated-at="snapshot?.fetched_at || ''"
    :stale="snapshot?.stale || false"
    :status="overallStatus"
    :items="productRows"
    :active-incidents="activeIncidents"
    :recent-resolved="recentResolved"
    source-url="https://status.openai.com/"
    :loading="loading"
    :error="error"
    :load-failed-text="t('serviceStatus.official.loadFailed')"
    @refresh="$emit('refresh')"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import OfficialStatusCard from './OfficialStatusCard.vue'
import type { OpenAIStatusSnapshot, ProviderHealthStatus } from '@/api/serviceStatus'

const props = defineProps<{
  snapshot: OpenAIStatusSnapshot | null
  loading: boolean
  error: boolean
}>()

defineEmits<{ refresh: [] }>()

const { t } = useI18n()

const productRows = computed(() => {
  const fallback: Array<{
    id: 'chatgpt' | 'codex'
    status: ProviderHealthStatus
  }> = [
    { id: 'chatgpt', status: 'unknown' },
    { id: 'codex', status: 'unknown' },
  ]
  const byID = new Map((props.snapshot?.products || []).map((product) => [product.id, product]))
  return fallback.map((product) => {
    const resolved = byID.get(product.id) ?? product
    return {
      id: resolved.id,
      name: resolved.id === 'codex' ? 'Codex' : 'ChatGPT',
      status: resolved.status,
      icon: resolved.id === 'codex' ? ('terminal' as const) : ('chatBubble' as const),
    }
  })
})

const overallStatus = computed<ProviderHealthStatus>(() => {
  const ranks: Record<ProviderHealthStatus, number> = {
    operational: 0,
    degraded: 1,
    partial_outage: 2,
    major_outage: 3,
    unknown: -1,
  }
  return productRows.value.reduce<ProviderHealthStatus>((current, product) => (
    ranks[product.status] > ranks[current] ? product.status : current
  ), 'unknown')
})

const activeIncidents = computed(() => (props.snapshot?.active_incidents || []).map((incident) => ({
  id: incident.id,
  name: incident.name,
  latestBody: incident.latest_body,
  updatedAt: incident.updated_at,
  sourceUrl: incident.source_url,
})))

const recentResolved = computed(() => (props.snapshot?.recent_resolved || []).map((incident) => ({
  id: incident.id,
  name: incident.name,
  latestBody: incident.latest_body,
  updatedAt: incident.resolved_at || incident.updated_at,
  sourceUrl: incident.source_url,
})))
</script>
