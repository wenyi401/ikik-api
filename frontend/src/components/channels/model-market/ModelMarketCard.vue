<template>
  <button type="button" class="model-card" @click="emit('select', item)">
    <span class="model-card-icon">
      <PlatformIcon :platform="item.platform as GroupPlatform" size="md" />
    </span>

    <span class="model-card-content">
      <span class="model-card-heading">
        <span class="model-card-name" :title="item.name">{{ item.name }}</span>
        <Icon name="chevronRight" size="sm" class="model-card-chevron" />
      </span>
      <span class="model-card-platform">{{ platformLabel(item.platform) }}</span>
      <span class="model-card-price">{{ priceSummary }}</span>
      <span class="model-card-meta">
        <span>{{ t('modelMarket.summary.groups', { count: item.group_count }) }}</span>
        <span aria-hidden="true">·</span>
        <span>{{ t('modelMarket.summary.channels', { count: item.channel_count }) }}</span>
      </span>
    </span>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { GroupPlatform } from '@/types'
import type { ModelMarketCatalogItem } from '@/utils/modelMarket'
import { platformLabel } from '@/utils/platformColors'
import { formatScaled } from '@/utils/pricing'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
} from '@/constants/channel'

const props = defineProps<{ item: ModelMarketCatalogItem }>()
const emit = defineEmits<{ (event: 'select', item: ModelMarketCatalogItem): void }>()
const { t } = useI18n()

const priceSummary = computed(() => {
  if (props.item.pricing_conflict) return t('modelMarket.priceSummary.varies')
  const pricing = props.item.model.pricing
  if (!pricing) return t('availableChannels.noPricing')

  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    const parts: string[] = []
    if (pricing.input_price != null) {
      parts.push(t('modelMarket.priceSummary.input', { price: formatScaled(pricing.input_price, 1_000_000) }))
    }
    if (pricing.output_price != null) {
      parts.push(t('modelMarket.priceSummary.output', { price: formatScaled(pricing.output_price, 1_000_000) }))
    }
    return parts.join(' · ') || t('modelMarket.priceSummary.unknown')
  }

  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    return t('modelMarket.priceSummary.perRequest', { price: formatScaled(pricing.per_request_price, 1) })
  }

  if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    return t('modelMarket.priceSummary.imageOutput', { price: formatScaled(pricing.image_output_price, 1) })
  }

  return t('modelMarket.priceSummary.unknown')
})
</script>

<style scoped>
.model-card {
  display: grid;
  min-width: 0;
  grid-template-columns: 2.5rem minmax(0, 1fr);
  gap: 0.875rem;
  padding: 1rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
  color: var(--ui-text);
  text-align: left;
  transition: border-color 140ms ease, background-color 140ms ease;
}

.model-card:hover {
  border-color: var(--ui-border-strong);
  background: var(--ui-surface-subtle);
}

.model-card:focus-visible {
  outline: 2px solid var(--ui-text);
  outline-offset: 2px;
}

.model-card-icon {
  display: flex;
  width: 2.5rem;
  height: 2.5rem;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.model-card-content,
.model-card-heading {
  display: flex;
  min-width: 0;
}

.model-card-content {
  flex-direction: column;
}

.model-card-heading {
  align-items: center;
  gap: 0.5rem;
}

.model-card-name {
  flex: 1 1 auto;
  overflow: hidden;
  font-size: 0.9375rem;
  font-weight: 600;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-card-chevron {
  flex: 0 0 auto;
  color: var(--ui-text-tertiary);
}

.model-card-platform,
.model-card-price,
.model-card-meta {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-card-platform {
  margin-top: 0.15rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

.model-card-price {
  margin-top: 0.75rem;
  color: var(--ui-text-secondary);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
}

.model-card-meta {
  display: flex;
  gap: 0.375rem;
  margin-top: 0.35rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

@media (max-width: 640px) {
  .model-card {
    padding: 0.875rem;
  }
}
</style>
