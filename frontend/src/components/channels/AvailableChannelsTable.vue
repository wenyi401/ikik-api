<template>
  <div v-if="loading" class="channel-state">
    <Icon name="refresh" size="lg" class="animate-spin" />
  </div>

  <div v-else-if="rows.length === 0" class="channel-state channel-state--empty">
    <Icon name="inbox" size="xl" />
    <p>{{ emptyLabel }}</p>
  </div>

  <div v-else class="channel-list">
    <article v-for="(channel, channelIndex) in rows" :key="`${channel.name}-${channelIndex}`" class="channel-panel">
      <header class="channel-header">
        <div class="min-w-0">
          <h3>{{ channel.name }}</h3>
          <p v-if="channel.description">{{ channel.description }}</p>
        </div>
        <div class="channel-summary">
          <span>{{ channel.platforms.length }} {{ columns.platform }}</span>
          <span aria-hidden="true">·</span>
          <span>{{ t('availableChannels.summary.models', { count: channelModelCount(channel) }) }}</span>
          <span aria-hidden="true">·</span>
          <span>{{ t('availableChannels.summary.groups', { count: channelGroupCount(channel) }) }}</span>
        </div>
      </header>

      <div class="channel-platforms">
        <section
          v-for="section in channel.platforms"
          :key="`${channel.name}-${section.platform}`"
          class="channel-platform-row"
        >
          <div class="channel-access">
            <div class="channel-platform-name">
              <PlatformIcon :platform="section.platform as GroupPlatform" size="sm" />
              <span>{{ platformLabel(section.platform) }}</span>
            </div>

            <div v-if="section.groups.length" class="channel-groups">
              <GroupBadge
                v-for="group in section.groups"
                :key="group.id"
                :name="group.name"
                :platform="group.platform as GroupPlatform"
                :subscription-type="(group.subscription_type || 'standard') as SubscriptionType"
                :rate-multiplier="group.rate_multiplier"
                :user-rate-multiplier="userGroupRates[group.id] ?? null"
                always-show-rate
              />
            </div>
            <span v-else class="channel-empty-value">-</span>
          </div>

          <div v-if="section.supported_models.length" class="channel-models">
            <button
              v-for="model in section.supported_models"
              :key="`${section.platform}-${model.name}`"
              type="button"
              class="channel-model-row"
              @click="emit('selectModel', { model, platform: section.platform })"
            >
              <span class="channel-model-main">
                <span class="channel-model-name">{{ model.name }}</span>
                <span class="channel-model-price">{{ priceSummary(model) }}</span>
              </span>
              <Icon name="chevronRight" size="sm" />
            </button>
          </div>
          <span v-else class="channel-empty-value">{{ noModelsLabel }}</span>
        </section>
      </div>
    </article>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { UserAvailableChannel, UserSupportedModel } from '@/api/channels'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { platformLabel } from '@/utils/platformColors'
import { formatScaled } from '@/utils/pricing'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
} from '@/constants/channel'

defineProps<{
  columns: {
    name: string
    description: string
    platform: string
    groups: string
    supportedModels: string
  }
  rows: UserAvailableChannel[]
  loading: boolean
  pricingKeyPrefix: string
  noPricingLabel: string
  noModelsLabel: string
  emptyLabel: string
  userGroupRates: Record<number, number>
}>()

const emit = defineEmits<{
  (event: 'selectModel', payload: { model: UserSupportedModel; platform: string }): void
}>()
const { t } = useI18n()

function channelModelCount(channel: UserAvailableChannel): number {
  return channel.platforms.reduce((sum, section) => sum + section.supported_models.length, 0)
}

function channelGroupCount(channel: UserAvailableChannel): number {
  return channel.platforms.reduce((sum, section) => sum + section.groups.length, 0)
}

function priceSummary(model: UserSupportedModel): string {
  const pricing = model.pricing
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
}
</script>

<style scoped>
.channel-state {
  display: flex;
  min-height: 16rem;
  align-items: center;
  justify-content: center;
  color: var(--ui-text-tertiary);
}

.channel-state--empty {
  flex-direction: column;
  gap: 0.75rem;
  font-size: 0.875rem;
}

.channel-list {
  display: grid;
  gap: 0.75rem;
}

.channel-panel {
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.channel-header {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.125rem;
  border-bottom: 1px solid var(--ui-border);
}

.channel-header h3 {
  overflow: hidden;
  color: var(--ui-text);
  font-size: 0.9375rem;
  font-weight: 600;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.channel-header p {
  margin-top: 0.2rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  line-height: 1.45;
}

.channel-summary {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.375rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

.channel-platform-row {
  display: grid;
  grid-template-columns: minmax(15rem, 0.8fr) minmax(24rem, 1.4fr);
  gap: 1.5rem;
  padding: 1rem 1.125rem;
}

.channel-platform-row:not(:last-child) {
  border-bottom: 1px solid var(--ui-border);
}

.channel-access {
  min-width: 0;
}

.channel-platform-name {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  color: var(--ui-text);
  font-size: 0.8125rem;
  font-weight: 600;
}

.channel-groups {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  margin-top: 0.75rem;
}

.channel-models {
  min-width: 0;
}

.channel-model-row {
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.625rem 0;
  color: var(--ui-text-tertiary);
  text-align: left;
}

.channel-model-row:not(:last-child) {
  border-bottom: 1px solid var(--ui-border);
}

.channel-model-row:hover .channel-model-name {
  text-decoration: underline;
  text-underline-offset: 2px;
}

.channel-model-main {
  display: grid;
  min-width: 0;
  flex: 1 1 auto;
  grid-template-columns: minmax(10rem, 1fr) minmax(12rem, auto);
  align-items: center;
  gap: 1rem;
}

.channel-model-name,
.channel-model-price {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.channel-model-name {
  color: var(--ui-text);
  font-size: 0.8125rem;
  font-weight: 500;
}

.channel-model-price {
  color: var(--ui-text-secondary);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.channel-empty-value {
  color: var(--ui-text-tertiary);
  font-size: 0.8125rem;
}

@media (max-width: 900px) {
  .channel-platform-row {
    grid-template-columns: 1fr;
    gap: 0.75rem;
  }
}

@media (max-width: 640px) {
  .channel-header {
    flex-direction: column;
    padding: 0.875rem;
  }

  .channel-summary {
    justify-content: flex-start;
  }

  .channel-platform-row {
    padding: 0.875rem;
  }

  .channel-model-main {
    grid-template-columns: 1fr;
    gap: 0.15rem;
  }

  .channel-model-price {
    text-align: left;
  }
}
</style>
