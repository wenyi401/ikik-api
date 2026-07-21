<template>
  <div class="table-wrapper">
    <table class="channels-table">
      <colgroup>
        <col class="channel-name-column" />
        <col class="channel-description-column" />
        <col class="channel-platform-column" />
        <col class="channel-groups-column" />
        <col class="channel-models-column" />
      </colgroup>
      <thead>
        <tr>
          <th scope="col">{{ columns.name }}</th>
          <th scope="col">{{ columns.description }}</th>
          <th scope="col">{{ columns.platform }}</th>
          <th scope="col">{{ columns.groups }}</th>
          <th scope="col">{{ columns.supportedModels }}</th>
        </tr>
      </thead>

      <tbody v-if="loading">
        <tr>
          <td colspan="5">
            <div class="channel-state">
              <Icon name="refresh" size="lg" class="animate-spin" />
            </div>
          </td>
        </tr>
      </tbody>

      <tbody v-else-if="rows.length === 0">
        <tr>
          <td colspan="5">
            <div class="channel-state channel-state--empty">
              <Icon name="inbox" size="xl" />
              <p>{{ emptyLabel }}</p>
            </div>
          </td>
        </tr>
      </tbody>

      <tbody
        v-else
        v-for="(channel, channelIndex) in rows"
        :key="`${channel.name}-${channelIndex}`"
        class="channel-body"
      >
        <tr
          v-for="(section, sectionIndex) in channel.platforms"
          :key="`${channel.name}-${section.platform}`"
        >
          <td
            v-if="sectionIndex === 0"
            :rowspan="channel.platforms.length"
            class="channel-name-cell"
          >
            {{ channel.name }}
          </td>
          <td
            v-if="sectionIndex === 0"
            :rowspan="channel.platforms.length"
            class="channel-description-cell"
          >
            {{ channel.description || '-' }}
          </td>

          <td class="channel-platform-cell">
            <span class="channel-platform-name">
              <PlatformIcon :platform="section.platform as GroupPlatform" size="sm" />
              <span>{{ platformLabel(section.platform) }}</span>
            </span>
          </td>

          <td class="channel-groups-cell">
            <div v-if="exclusiveGroups(section).length" class="channel-group-set">
              <span
                class="channel-group-kind channel-group-kind--exclusive"
                :title="t('availableChannels.exclusiveTooltip')"
              >
                <Icon name="shield" size="xs" />
                {{ t('availableChannels.exclusive') }}
              </span>
              <div
                v-for="group in exclusiveGroups(section)"
                :key="`exclusive-${group.id}`"
                class="channel-group-item"
              >
                <GroupBadge
                  :name="group.name"
                  :platform="group.platform as GroupPlatform"
                  :subscription-type="(group.subscription_type || 'standard') as SubscriptionType"
                  :rate-multiplier="group.rate_multiplier"
                  :user-rate-multiplier="userGroupRates[group.id] ?? null"
                  always-show-rate
                />
                <span
                  v-if="hasPeakRate(group)"
                  class="channel-peak-rate"
                  :title="peakRateTitle(group)"
                >
                  <Icon name="clock" size="xs" />
                  {{ peakRateLabel(group) }}
                </span>
              </div>
            </div>

            <div v-if="publicGroups(section).length" class="channel-group-set">
              <span
                class="channel-group-kind"
                :title="t('availableChannels.publicTooltip')"
              >
                <Icon name="globe" size="xs" />
                {{ t('availableChannels.public') }}
              </span>
              <div
                v-for="group in publicGroups(section)"
                :key="`public-${group.id}`"
                class="channel-group-item"
              >
                <GroupBadge
                  :name="group.name"
                  :platform="group.platform as GroupPlatform"
                  :subscription-type="(group.subscription_type || 'standard') as SubscriptionType"
                  :rate-multiplier="group.rate_multiplier"
                  :user-rate-multiplier="userGroupRates[group.id] ?? null"
                  always-show-rate
                />
                <span
                  v-if="hasPeakRate(group)"
                  class="channel-peak-rate"
                  :title="peakRateTitle(group)"
                >
                  <Icon name="clock" size="xs" />
                  {{ peakRateLabel(group) }}
                </span>
              </div>
            </div>

            <span v-if="section.groups.length === 0" class="channel-empty-value">-</span>
          </td>

          <td class="channel-models-cell">
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
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type {
  UserAvailableChannel,
  UserAvailableGroup,
  UserChannelPlatformSection,
  UserSupportedModel,
} from '@/api/channels'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { platformLabel } from '@/utils/platformColors'
import { formatScaled } from '@/utils/pricing'
import { useAppStore } from '@/stores/app'
import {
  formatPeakRateWindow,
  hasPeakRate as groupHasPeakRate,
  serverTimezoneLabel,
} from '@/utils/peak-rate'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
} from '@/constants/channel'

const props = defineProps<{
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
const appStore = useAppStore()

function exclusiveGroups(section: UserChannelPlatformSection): UserAvailableGroup[] {
  return section.groups.filter((group) => group.is_exclusive)
}

function publicGroups(section: UserChannelPlatformSection): UserAvailableGroup[] {
  return section.groups.filter((group) => !group.is_exclusive)
}

function hasPeakRate(group: UserAvailableGroup): boolean {
  return groupHasPeakRate(group)
}

function peakRateLabel(group: UserAvailableGroup): string {
  const timezone = serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)
  return formatPeakRateWindow(group, timezone)
}

function peakRateTitle(group: UserAvailableGroup): string {
  return t('common.peakRateTooltip', { window: peakRateLabel(group) }) + t('common.peakRateImageNote')
}

function priceSummary(model: UserSupportedModel): string {
  const pricing = model.pricing
  if (!pricing) return props.noPricingLabel

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
.table-wrapper {
  width: 100%;
  overflow-x: auto;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.channels-table {
  width: 100%;
  min-width: 64rem;
  border-collapse: collapse;
  table-layout: fixed;
}

.channel-name-column {
  width: 10rem;
}

.channel-description-column {
  width: 13rem;
}

.channel-platform-column {
  width: 10rem;
}

.channel-groups-column {
  width: 21rem;
}

.channels-table th {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--ui-border);
  background: var(--ui-surface-muted);
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0;
  text-align: left;
  text-transform: uppercase;
}

.channels-table td {
  padding: 0.875rem 1rem;
  border-bottom: 1px solid var(--ui-border);
  color: var(--ui-text-secondary);
  vertical-align: top;
}

.channel-body + .channel-body tr:first-child td {
  border-top: 2px solid var(--ui-border-strong);
}

.channel-body:last-child tr:last-child td {
  border-bottom: 0;
}

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

.channel-name-cell {
  color: var(--ui-text);
  font-size: 0.875rem;
  font-weight: 600;
  line-height: 1.4;
}

.channel-description-cell {
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  line-height: 1.5;
}

.channel-platform-name {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  color: var(--ui-text);
  font-size: 0.8125rem;
  font-weight: 600;
}

.channel-group-set {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem 0.5rem;
}

.channel-group-set + .channel-group-set {
  margin-top: 0.625rem;
}

.channel-group-kind,
.channel-peak-rate {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.6875rem;
  font-weight: 600;
}

.channel-group-kind {
  width: 4rem;
  color: var(--ui-text-tertiary);
}

.channel-group-kind--exclusive {
  color: var(--ui-warning);
}

.channel-group-item {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.25rem;
}

.channel-peak-rate {
  padding: 0.125rem 0.375rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-sm);
  background: var(--ui-warning-soft);
  color: var(--ui-warning);
  font-weight: 500;
  white-space: nowrap;
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
  padding: 0.5rem 0;
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
  .channels-table {
    min-width: 58rem;
  }
}

@media (max-width: 640px) {
  .channel-model-main {
    grid-template-columns: 1fr;
    gap: 0.15rem;
  }

  .channel-model-price {
    text-align: left;
  }
}
</style>
