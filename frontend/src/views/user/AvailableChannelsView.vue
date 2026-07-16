<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <div class="available-toolbar">
        <div class="available-search">
          <Icon name="search" size="md" />
          <input
            v-model="searchQuery"
            type="search"
            :placeholder="t('availableChannels.searchPlaceholder')"
          />
        </div>
        <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="loadChannels">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </UiIconButton>
      </div>

      <div class="available-summary">
        <span>{{ t('availableChannels.summary.channels', { count: filteredChannels.length }) }}</span>
        <span aria-hidden="true">·</span>
        <span>{{ t('availableChannels.summary.models', { count: visibleModelCount }) }}</span>
        <span aria-hidden="true">·</span>
        <span>{{ t('availableChannels.summary.groups', { count: visibleGroupCount }) }}</span>
      </div>

      <AvailableChannelsTable
        :columns="columnLabels"
        :rows="filteredChannels"
        :loading="loading"
        :user-group-rates="userGroupRates"
        pricing-key-prefix="availableChannels.pricing"
        :no-pricing-label="t('availableChannels.noPricing')"
        :no-models-label="t('availableChannels.noModels')"
        :empty-label="t('availableChannels.empty')"
        @select-model="openModelDetail"
      />
    </UiPage>

    <ModelMarketDetailDialog
      :item="selectedItem"
      :user-group-rates="userGroupRates"
      @close="selectedItem = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AvailableChannelsTable from '@/components/channels/AvailableChannelsTable.vue'
import ModelMarketDetailDialog from '@/components/channels/model-market/ModelMarketDetailDialog.vue'
import userChannelsAPI, { type UserAvailableChannel, type UserSupportedModel } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { UiIconButton, UiPage } from '@/ui'
import { extractApiErrorMessage } from '@/utils/apiError'
import { buildModelCatalogItems, type ModelMarketCatalogItem } from '@/utils/modelMarket'

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const searchQuery = ref('')
const selectedItem = ref<ModelMarketCatalogItem | null>(null)

const columnLabels = computed(() => ({
  name: t('availableChannels.columns.name'),
  description: t('availableChannels.columns.description'),
  platform: t('availableChannels.columns.platform'),
  groups: t('availableChannels.columns.groups'),
  supportedModels: t('availableChannels.columns.supportedModels'),
}))

const catalogItems = computed(() => buildModelCatalogItems(channels.value))
const filteredChannels = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return channels.value

  return channels.value
    .map((channel) => {
      if (
        channel.name.toLowerCase().includes(query) ||
        (channel.description || '').toLowerCase().includes(query)
      ) {
        return channel
      }

      const platforms = channel.platforms.filter((section) => (
        section.platform.toLowerCase().includes(query) ||
        section.groups.some((group) => group.name.toLowerCase().includes(query)) ||
        section.supported_models.some((model) => model.name.toLowerCase().includes(query))
      ))
      return platforms.length ? { ...channel, platforms } : null
    })
    .filter((channel): channel is UserAvailableChannel => channel != null)
})

const visibleModelCount = computed(() => filteredChannels.value.reduce(
  (total, channel) => total + channel.platforms.reduce(
    (platformTotal, section) => platformTotal + section.supported_models.length,
    0,
  ),
  0,
))
const visibleGroupCount = computed(() => filteredChannels.value.reduce(
  (total, channel) => total + channel.platforms.reduce(
    (platformTotal, section) => platformTotal + section.groups.length,
    0,
  ),
  0,
))

function openModelDetail(payload: { model: UserSupportedModel; platform: string }) {
  const platform = payload.model.platform || payload.platform
  selectedItem.value = catalogItems.value.find((item) => (
    item.name === payload.model.name && item.platform === platform
  )) ?? null
}

async function loadChannels() {
  loading.value = true
  try {
    const [list, rates] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch((err: unknown) => {
        console.error('Failed to load user group rates:', err)
        return {} as Record<number, number>
      }),
    ])
    channels.value = list
    userGroupRates.value = rates
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(loadChannels)
</script>

<style scoped>
.available-toolbar {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.625rem;
}

.available-search {
  display: flex;
  width: min(34rem, 100%);
  min-width: 0;
  height: 2.625rem;
  align-items: center;
  gap: 0.625rem;
  padding: 0 0.75rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
  color: var(--ui-text-tertiary);
}

.available-search:focus-within {
  border-color: var(--ui-border-strong);
  box-shadow: 0 0 0 3px var(--ui-focus);
}

.available-search input {
  width: 100%;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--ui-text);
  font-size: 0.875rem;
}

.available-search input::placeholder {
  color: var(--ui-text-tertiary);
}

.available-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

@media (max-width: 640px) {
  .available-search {
    flex: 1 1 auto;
  }
}
</style>
