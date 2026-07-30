<template>
  <OfficialStatusCard
    :card-id="`${provider.id}-official-status`"
    :title="t('serviceStatus.providers.officialTitle', { provider: providerName })"
    :updated-at="provider.source_updated_at || fetchedAt"
    :stale="provider.stale"
    :status="provider.status"
    :items="items"
    :active-incidents="activeIncidents"
    :loading="loading"
    :error="error"
    :load-failed-text="t('serviceStatus.providers.loadFailed')"
    @refresh="$emit('refresh')"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import OfficialStatusCard from './OfficialStatusCard.vue'
import type { GroupPlatform } from '@/types'
import type { ProviderStatusProvider } from '@/api/serviceStatus'

const props = defineProps<{
  provider: ProviderStatusProvider
  fetchedAt: string
  loading: boolean
  error: boolean
}>()

defineEmits<{ refresh: [] }>()

const { t } = useI18n()

const providerName = computed(() => ({
  claude: 'Claude',
  grok: 'Grok',
  gemini: 'Gemini',
})[props.provider.id])

const providerPlatform = computed<GroupPlatform>(() => ({
  claude: 'anthropic',
  grok: 'grok',
  gemini: 'gemini',
})[props.provider.id] as GroupPlatform)

const items = computed(() => [{
  id: props.provider.id,
  name: providerName.value,
  status: props.provider.status,
  platform: providerPlatform.value,
}])

const activeIncidents = computed(() => props.provider.active_incidents.map((incident) => ({
  id: incident.id,
  name: incident.name,
  latestBody: incident.latest_body,
  updatedAt: incident.updated_at,
})))
</script>
