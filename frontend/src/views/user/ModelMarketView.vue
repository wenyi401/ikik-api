<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <div class="market-toolbar">
        <div class="market-search">
          <Icon name="search" size="md" />
          <input
            v-model="filters.search"
            type="search"
            :placeholder="t('modelMarket.searchPlaceholder')"
          />
        </div>

        <button
          type="button"
          class="market-filter-toggle"
          :aria-expanded="showMobileFilters"
          :aria-label="t('modelMarket.filters.advanced')"
          :title="t('modelMarket.filters.advanced')"
          @click="showMobileFilters = !showMobileFilters"
        >
          <Icon name="filter" size="md" />
        </button>

        <div class="market-filters" :class="{ 'market-filters--open': showMobileFilters }">
          <select v-model="filters.platform" class="input" :aria-label="t('modelMarket.filters.allPlatforms')">
            <option value="">{{ t('modelMarket.filters.allPlatforms') }}</option>
            <option v-for="platform in filterOptions.platforms" :key="platform" :value="platform">
              {{ platformLabel(platform) }}
            </option>
          </select>

          <select v-model="filters.channel" class="input" :aria-label="t('modelMarket.filters.allChannels')">
            <option value="">{{ t('modelMarket.filters.allChannels') }}</option>
            <option v-for="channel in filterOptions.channels" :key="channel" :value="channel">
              {{ channel }}
            </option>
          </select>

          <select v-model="filters.pricing" class="input" :aria-label="t('modelMarket.filters.allPricing')">
            <option value="all">{{ t('modelMarket.filters.allPricing') }}</option>
            <option value="with">{{ t('modelMarket.filters.withPricing') }}</option>
            <option value="without">{{ t('modelMarket.filters.withoutPricing') }}</option>
          </select>
        </div>

        <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="loadMarket">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </UiIconButton>
      </div>

      <div class="market-navigation">
        <div class="market-categories" role="tablist" :aria-label="t('modelMarket.columns.model')">
          <button
            v-for="category in categoryOptions"
            :key="category.key"
            type="button"
            role="tab"
            :aria-selected="filters.category === category.key"
            :class="['market-category', { 'market-category--active': filters.category === category.key }]"
            @click="filters.category = category.key"
          >
            {{ t(`modelMarket.categories.${category.key}`) }}
            <span>{{ category.count }}</span>
          </button>
        </div>

        <div class="market-summary">
          <span>{{ t('modelMarket.summary.visibleModels', { count: filteredItems.length }) }}</span>
          <span aria-hidden="true">·</span>
          <span>{{ t('modelMarket.summary.groups', { count: filteredGroupCount }) }}</span>
          <span aria-hidden="true">·</span>
          <span>{{ t('modelMarket.summary.channels', { count: filteredChannelCount }) }}</span>
          <button v-if="hasActiveFilters" type="button" @click="clearFilters">
            {{ t('modelMarket.clearFilters') }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="market-loading">
        <Icon name="refresh" size="lg" class="animate-spin" />
      </div>

      <div v-else-if="filteredItems.length === 0" class="market-empty">
        <Icon name="inbox" size="xl" />
        <p>{{ t('modelMarket.empty') }}</p>
      </div>

      <div v-else class="market-grid">
        <ModelMarketCard
          v-for="item in filteredItems"
          :key="item.key"
          :item="item"
          @select="selectedItem = $event"
        />
      </div>
    </UiPage>

    <ModelMarketDetailDialog
      :item="selectedItem"
      :user-group-rates="userGroupRates"
      @close="selectedItem = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelMarketCard from '@/components/channels/model-market/ModelMarketCard.vue'
import ModelMarketDetailDialog from '@/components/channels/model-market/ModelMarketDetailDialog.vue'
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { UiIconButton, UiPage } from '@/ui'
import { extractApiErrorMessage } from '@/utils/apiError'
import { platformLabel } from '@/utils/platformColors'
import {
  buildModelCatalogItems,
  countModelCatalogChannels,
  countModelCatalogGroups,
  filterModelCatalogItems,
  getModelCatalogFilterOptions,
  type ModelMarketCatalogItem,
  type ModelMarketCategoryKey,
  type ModelMarketFilters,
} from '@/utils/modelMarket'

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const showMobileFilters = ref(false)
const selectedItem = ref<ModelMarketCatalogItem | null>(null)
const filters = reactive<ModelMarketFilters>({
  search: '',
  category: 'all',
  platform: '',
  channel: '',
  pricing: 'all',
})

const categoryOrder: ModelMarketCategoryKey[] = [
  'all',
  'openai',
  'anthropic',
  'gemini',
  'qwen',
  'deepseek',
  'zhipu',
  'image',
  'embedding',
  'other',
]

const catalogItems = computed(() => buildModelCatalogItems(channels.value))
const filterOptions = computed(() => getModelCatalogFilterOptions(catalogItems.value))
const filteredItems = computed(() => filterModelCatalogItems(catalogItems.value, filters))
const filteredGroupCount = computed(() => countModelCatalogGroups(filteredItems.value))
const filteredChannelCount = computed(() => countModelCatalogChannels(filteredItems.value))
const categoryOptions = computed(() => categoryOrder
  .map((key) => ({
    key,
    count: key === 'all'
      ? catalogItems.value.length
      : catalogItems.value.filter((item) => item.category === key).length,
  }))
  .filter((item) => item.key === 'all' || item.count > 0))

const hasActiveFilters = computed(() => (
  filters.search.trim() !== '' ||
  filters.category !== 'all' ||
  filters.platform !== '' ||
  filters.channel !== '' ||
  filters.pricing !== 'all'
))

function clearFilters() {
  filters.search = ''
  filters.category = 'all'
  filters.platform = ''
  filters.channel = ''
  filters.pricing = 'all'
}

async function loadMarket() {
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

onMounted(loadMarket)
</script>

<style scoped>
.market-toolbar {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.625rem;
}

.market-search {
  display: flex;
  min-width: 16rem;
  max-width: 34rem;
  flex: 1 1 24rem;
  height: 2.625rem;
  align-items: center;
  gap: 0.625rem;
  padding: 0 0.75rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
  color: var(--ui-text-tertiary);
}

.market-search:focus-within {
  border-color: var(--ui-border-strong);
  box-shadow: 0 0 0 3px var(--ui-focus);
}

.market-search input {
  width: 100%;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--ui-text);
  font-size: 0.875rem;
}

.market-search input::placeholder {
  color: var(--ui-text-tertiary);
}

.market-filters {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.market-filters .input {
  width: 9.75rem;
  min-height: 2.625rem;
}

.market-filter-toggle {
  display: none;
  width: 2.625rem;
  height: 2.625rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  color: var(--ui-text-secondary);
}

.market-navigation {
  min-width: 0;
}

.market-categories {
  display: flex;
  min-width: 0;
  gap: 1.25rem;
  overflow-x: auto;
  border-bottom: 1px solid var(--ui-border);
  scrollbar-width: none;
}

.market-categories::-webkit-scrollbar {
  display: none;
}

.market-category {
  position: relative;
  display: inline-flex;
  min-height: 2.5rem;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.35rem;
  color: var(--ui-text-tertiary);
  font-size: 0.8125rem;
  font-weight: 500;
}

.market-category::after {
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 2px;
  background: transparent;
  content: '';
}

.market-category:hover,
.market-category--active {
  color: var(--ui-text);
}

.market-category--active::after {
  background: var(--ui-text);
}

.market-category span {
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
  font-variant-numeric: tabular-nums;
}

.market-summary {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  padding-top: 0.75rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

.market-summary button {
  margin-left: 0.375rem;
  color: var(--ui-text);
  font-weight: 500;
}

.market-summary button:hover {
  text-decoration: underline;
}

.market-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
}

.market-loading,
.market-empty {
  display: flex;
  min-height: 16rem;
  align-items: center;
  justify-content: center;
  color: var(--ui-text-tertiary);
}

.market-empty {
  flex-direction: column;
  gap: 0.75rem;
  font-size: 0.875rem;
}

@media (max-width: 1200px) {
  .market-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .market-filters .input {
    width: 8.5rem;
  }
}

@media (max-width: 820px) {
  .market-toolbar {
    flex-wrap: wrap;
  }

  .market-search {
    min-width: 0;
    flex: 1 1 0;
  }

  .market-filter-toggle {
    display: inline-flex;
  }

  .market-filters {
    display: none;
    width: 100%;
    order: 4;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .market-filters--open {
    display: grid;
  }

  .market-filters .input {
    width: 100%;
  }

  .market-filters .input:last-child {
    grid-column: 1 / -1;
  }
}

@media (max-width: 640px) {
  .market-grid {
    grid-template-columns: 1fr;
    gap: 0.625rem;
  }

  .market-categories {
    margin-inline: -1rem;
    padding-inline: 1rem;
  }

  .market-summary {
    white-space: nowrap;
  }
}
</style>
