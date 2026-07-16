<template>
  <BaseDialog
    :show="item != null"
    :title="item?.name || ''"
    width="wide"
    :close-on-click-outside="true"
    @close="emit('close')"
  >
    <template v-if="item">
      <div class="model-detail-summary">
        <span class="model-detail-icon">
          <PlatformIcon :platform="item.platform as GroupPlatform" size="lg" />
        </span>
        <div class="min-w-0">
          <p class="model-detail-platform">{{ platformLabel(item.platform) }}</p>
          <p class="model-detail-meta">
            {{ t('modelMarket.summary.groups', { count: item.group_count }) }}
            <span aria-hidden="true">·</span>
            {{ t('modelMarket.summary.channels', { count: item.channel_count }) }}
            <template v-if="lowestRate != null">
              <span aria-hidden="true">·</span>
              {{ t('modelMarket.lowestRate', { rate: `${formatRate(lowestRate)}x` }) }}
            </template>
          </p>
        </div>
      </div>

      <section class="model-detail-section">
        <h4>{{ t('modelMarket.detail.pricing') }}</h4>
        <p v-if="item.pricing_conflict" class="model-detail-note">
          {{ t('modelMarket.pricingVaries') }}
        </p>
        <div v-else-if="pricingItems.length" class="model-detail-pricing">
          <div v-for="price in pricingItems" :key="price.key" class="model-detail-price-row">
            <span>{{ price.label }}</span>
            <strong>{{ price.value }}</strong>
          </div>
        </div>
        <p v-else class="model-detail-note">{{ t('availableChannels.noPricing') }}</p>

        <div v-if="pricingIntervals.length" class="model-detail-intervals">
          <div v-for="(interval, index) in pricingIntervals" :key="index" class="model-detail-interval-row">
            <span>{{ intervalLabel(interval) }}</span>
            <strong>{{ intervalPrice(interval) }}</strong>
          </div>
        </div>
      </section>

      <section class="model-detail-section">
        <h4>{{ t('modelMarket.detail.accessGroups') }}</h4>
        <div class="model-detail-list">
          <div v-for="group in item.groups" :key="group.id" class="model-detail-row">
            <GroupBadge
              :name="group.name"
              :platform="group.platform as GroupPlatform"
              :subscription-type="(group.subscription_type || 'standard') as SubscriptionType"
              :rate-multiplier="group.rate_multiplier"
              :user-rate-multiplier="userGroupRates[group.id] ?? null"
              always-show-rate
            />
            <span class="model-detail-row-value">
              {{ group.is_exclusive
                ? t('modelMarket.groupVisibility.private')
                : t('modelMarket.groupVisibility.public') }}
            </span>
          </div>
        </div>
      </section>

      <section class="model-detail-section">
        <h4>{{ t('modelMarket.detail.channels') }}</h4>
        <div class="model-detail-list">
          <div v-for="channel in item.channels" :key="channel.name" class="model-detail-row model-detail-row--channel">
            <div class="min-w-0">
              <p class="model-detail-channel-name">{{ channel.name }}</p>
              <p v-if="channel.description" class="model-detail-channel-description">
                {{ channel.description }}
              </p>
            </div>
          </div>
        </div>
      </section>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { UserAvailableGroup, UserPricingInterval } from '@/api/channels'
import type { GroupPlatform, SubscriptionType } from '@/types'
import type { ModelMarketCatalogItem } from '@/utils/modelMarket'
import { platformLabel } from '@/utils/platformColors'
import { formatScaled } from '@/utils/pricing'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
} from '@/constants/channel'

const props = defineProps<{
  item: ModelMarketCatalogItem | null
  userGroupRates: Record<number, number>
}>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t } = useI18n()

const pricingItems = computed(() => {
  const pricing = props.item?.model.pricing
  if (!pricing) return []

  const items: Array<{ key: string; label: string; value: string }> = [{
    key: 'mode',
    label: t('availableChannels.pricing.billingMode'),
    value: billingModeLabel(pricing.billing_mode),
  }]

  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    addPrice(items, 'input', t('availableChannels.pricing.inputPrice'), pricing.input_price, 1_000_000)
    addPrice(items, 'output', t('availableChannels.pricing.outputPrice'), pricing.output_price, 1_000_000)
    addPrice(items, 'cacheWrite', t('availableChannels.pricing.cacheWritePrice'), pricing.cache_write_price, 1_000_000)
    addPrice(items, 'cacheRead', t('availableChannels.pricing.cacheReadPrice'), pricing.cache_read_price, 1_000_000)
    addPrice(items, 'imageOutput', t('availableChannels.pricing.imageOutputPrice'), pricing.image_output_price, 1_000_000)
  } else if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    addPrice(items, 'request', t('availableChannels.pricing.perRequestPrice'), pricing.per_request_price, 1)
  } else if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    addPrice(items, 'image', t('availableChannels.pricing.imageOutputPrice'), pricing.image_output_price, 1)
  }

  return items
})

const pricingIntervals = computed(() => props.item?.model.pricing?.intervals ?? [])
const lowestRate = computed(() => {
  if (!props.item?.groups.length) return null
  return Math.min(...props.item.groups.map(effectiveRate))
})

function addPrice(
  items: Array<{ key: string; label: string; value: string }>,
  key: string,
  label: string,
  value: number | null,
  scale: number,
) {
  if (value == null) return
  const unit = scale === 1_000_000
    ? t('availableChannels.pricing.unitPerMillion')
    : t('availableChannels.pricing.unitPerRequest')
  items.push({ key, label, value: `${formatScaled(value, scale)} ${unit}` })
}

function billingModeLabel(mode: string): string {
  if (mode === BILLING_MODE_TOKEN) return t('availableChannels.pricing.billingModeToken')
  if (mode === BILLING_MODE_PER_REQUEST) return t('availableChannels.pricing.billingModePerRequest')
  if (mode === BILLING_MODE_IMAGE) return t('availableChannels.pricing.billingModeImage')
  return t('modelMarket.priceSummary.unknown')
}

function effectiveRate(group: UserAvailableGroup): number {
  return props.userGroupRates[group.id] ?? group.rate_multiplier
}

function formatRate(rate: number): string {
  return Number.isInteger(rate) ? String(rate) : rate.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')
}

function intervalLabel(interval: UserPricingInterval): string {
  if (interval.tier_label) return interval.tier_label
  const max = interval.max_tokens == null
    ? t('availableChannels.pricing.rangeMax')
    : interval.max_tokens.toLocaleString()
  return `${interval.min_tokens.toLocaleString()} - ${max}`
}

function intervalPrice(interval: UserPricingInterval): string {
  const input = formatScaled(interval.input_price, 1_000_000)
  const output = formatScaled(interval.output_price, 1_000_000)
  return `${t('dashboard.input')} ${input} · ${t('dashboard.output')} ${output}`
}
</script>

<style scoped>
.model-detail-summary {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.875rem;
  padding-bottom: 1.125rem;
}

.model-detail-icon {
  display: flex;
  width: 2.75rem;
  height: 2.75rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
}

.model-detail-platform {
  color: var(--ui-text);
  font-size: 0.9375rem;
  font-weight: 600;
}

.model-detail-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  margin-top: 0.2rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

.model-detail-section {
  padding: 1rem 0;
  border-top: 1px solid var(--ui-border);
}

.model-detail-section h4 {
  margin-bottom: 0.75rem;
  color: var(--ui-text);
  font-size: 0.8125rem;
  font-weight: 600;
}

.model-detail-pricing {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 1.5rem;
}

.model-detail-price-row,
.model-detail-row,
.model-detail-interval-row {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.625rem 0;
}

.model-detail-price-row,
.model-detail-interval-row,
.model-detail-row:not(:last-child) {
  border-bottom: 1px solid var(--ui-border);
}

.model-detail-price-row span,
.model-detail-interval-row span,
.model-detail-note {
  color: var(--ui-text-tertiary);
  font-size: 0.8125rem;
}

.model-detail-price-row strong,
.model-detail-interval-row strong,
.model-detail-row-value {
  color: var(--ui-text);
  font-family: var(--ui-font-mono);
  font-size: 0.75rem;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.model-detail-intervals {
  margin-top: 0.75rem;
}

.model-detail-row--channel {
  align-items: flex-start;
  justify-content: flex-start;
}

.model-detail-channel-name {
  color: var(--ui-text);
  font-size: 0.8125rem;
  font-weight: 500;
}

.model-detail-channel-description {
  margin-top: 0.2rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  line-height: 1.45;
}

@media (max-width: 640px) {
  .model-detail-pricing {
    grid-template-columns: 1fr;
  }

  .model-detail-price-row:nth-child(odd) {
    border-bottom: 1px solid var(--ui-border);
  }
}
</style>
