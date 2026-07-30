<template>
  <div class="provider-list">
    <ProviderStatusCard
      v-for="provider in providerRows"
      :key="provider.id"
      :provider="provider"
      :fetched-at="snapshot?.fetched_at || ''"
      :loading="loading"
      :error="error"
      @refresh="$emit('refresh')"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import ProviderStatusCard from './ProviderStatusCard.vue'
import type { ProviderStatusProvider, ProviderStatusSnapshot } from '@/api/serviceStatus'

const props = defineProps<{
  snapshot: ProviderStatusSnapshot | null
  loading: boolean
  error: boolean
}>()

defineEmits<{ refresh: [] }>()

const fallbackProviders: ProviderStatusProvider[] = [
  {
    id: 'claude',
    status: 'unknown',
    stale: false,
    source_updated_at: '',
    active_incidents: [],
    components: [
      { id: 'claude_web', name: 'claude.ai', status: 'unknown' },
      { id: 'claude_api', name: 'Claude API', status: 'unknown' },
      { id: 'claude_code', name: 'Claude Code', status: 'unknown' },
    ],
  },
  {
    id: 'grok',
    status: 'unknown',
    stale: false,
    source_updated_at: '',
    active_incidents: [],
    components: [
      { id: 'grok_web', name: 'Grok Web', status: 'unknown' },
      { id: 'xai_api', name: 'xAI API', status: 'unknown' },
    ],
  },
  {
    id: 'gemini',
    status: 'unknown',
    stale: false,
    source_updated_at: '',
    active_incidents: [],
    components: [{ id: 'gemini_api', name: 'Gemini API', status: 'unknown' }],
  },
]

const providerRows = computed<ProviderStatusProvider[]>(() => {
  if (!props.snapshot) return fallbackProviders
  const byID = new Map(props.snapshot.providers.map((provider) => [provider.id, provider]))
  return fallbackProviders.map((fallback) => {
    const provider = byID.get(fallback.id)
    if (!provider) return fallback
    return {
      ...provider,
      components: Array.isArray(provider.components) ? provider.components : fallback.components,
      active_incidents: Array.isArray(provider.active_incidents) ? provider.active_incidents : [],
    }
  })
})
</script>

<style scoped>
.provider-list {
  display: grid;
  gap: 1rem;
}
</style>
