<template>
  <div class="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)]">
    <button
      type="button"
      class="flex w-full items-start justify-between gap-3 px-3 py-2.5 text-left transition-colors hover:bg-[var(--app-surface-muted)]"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <span class="flex min-w-0 flex-1 flex-col items-start gap-1">
        <span
          :class="[
          'inline-flex max-w-full items-center gap-1 rounded-lg border px-2 py-0.5 text-xs font-medium',
            badgeClass,
          ]"
        >
          <PlatformIcon
            v-if="effectivePlatform"
            :platform="effectivePlatform as GroupPlatform"
            size="xs"
          />
          <span class="min-w-0 truncate">{{ model.name }}</span>
        </span>
        <span class="text-[11px] leading-4 text-[var(--app-muted-strong)]">
          {{ priceSummary }}
        </span>
      </span>
      <Icon
        :name="expanded ? 'chevronUp' : 'chevronDown'"
        size="sm"
        class="flex-shrink-0 text-[var(--app-muted)]"
      />
    </button>

    <div
      v-if="expanded"
      class="space-y-3 border-t border-[var(--app-border)] px-3 py-3"
    >
      <div v-if="!model.pricing" class="text-xs text-[var(--app-muted-strong)]">
        {{ noPricingLabel }}
      </div>

      <template v-else>
        <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
          <div
            v-for="item in pricingItems"
            :key="item.key"
            class="rounded-lg bg-[var(--app-surface-muted)] px-2.5 py-2"
          >
            <div class="text-[11px] text-[var(--app-muted-strong)]">{{ item.label }}</div>
            <div class="mt-1 font-mono text-xs font-semibold text-[var(--app-text)]">
              {{ item.value }}
            </div>
          </div>
        </div>

        <div
          v-if="model.pricing.intervals && model.pricing.intervals.length > 0"
          class="rounded-xl border border-[var(--app-border)]"
        >
          <div class="border-b border-[var(--app-border)] px-2.5 py-2 text-xs font-medium text-[var(--app-muted-strong)]">
            {{ t(prefixKey('intervals')) }}
          </div>
          <div class="divide-y divide-[var(--app-border)]">
            <div
              v-for="(iv, idx) in model.pricing.intervals"
              :key="idx"
              class="grid gap-2 px-2.5 py-2 text-[11px] text-[var(--app-muted-strong)] md:grid-cols-[minmax(7rem,1fr)_repeat(4,minmax(6rem,auto))]"
            >
              <span class="font-medium text-[var(--app-text)]">
                <template v-if="iv.tier_label">{{ iv.tier_label }}</template>
                <template v-else>{{ formatRange(iv.min_tokens, iv.max_tokens) }}</template>
              </span>
              <span>{{ t(prefixKey('inputPrice')) }} {{ formatPrice(iv.input_price, tokenScale) }}</span>
              <span>{{ t(prefixKey('outputPrice')) }} {{ formatPrice(iv.output_price, tokenScale) }}</span>
              <span>{{ t(prefixKey('cacheWritePrice')) }} {{ formatPrice(iv.cache_write_price, tokenScale) }}</span>
              <span>{{ t(prefixKey('cacheReadPrice')) }} {{ formatPrice(iv.cache_read_price, tokenScale) }}</span>
            </div>
          </div>
        </div>
      </template>

      <div class="rounded-xl border border-[var(--app-border)]">
        <div class="border-b border-[var(--app-border)] px-2.5 py-2 text-xs font-medium text-[var(--app-muted-strong)]">
          {{ t('availableChannels.groupRates.title') }}
        </div>
        <div class="divide-y divide-[var(--app-border)]">
          <div
            v-for="group in groups"
            :key="group.id"
            class="grid gap-2 px-2.5 py-2 text-xs sm:grid-cols-[minmax(10rem,1fr)_minmax(7rem,auto)_minmax(12rem,1.2fr)] sm:items-center"
          >
            <GroupBadge
              :name="group.name"
              :platform="group.platform as GroupPlatform"
              :subscription-type="(group.subscription_type || 'standard') as SubscriptionType"
              :rate-multiplier="group.rate_multiplier"
              :user-rate-multiplier="userGroupRates[group.id] ?? null"
              always-show-rate
            />
            <div class="font-mono text-[var(--app-text)]">
              {{ effectiveRate(group) }}x
            </div>
            <div class="min-w-0 text-[11px] text-[var(--app-muted-strong)]">
              {{ effectivePriceSummary(group) }}
            </div>
          </div>
          <div v-if="groups.length === 0" class="px-2.5 py-2 text-xs text-[var(--app-muted)]">-</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { UserAvailableGroup, UserPricingInterval, UserSupportedModel } from '@/api/channels'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { BILLING_MODE_IMAGE, BILLING_MODE_PER_REQUEST, BILLING_MODE_TOKEN } from '@/constants/channel'
import { formatScaled } from '@/utils/pricing'
import { platformBadgeClass } from '@/utils/platformColors'

const props = withDefaults(
  defineProps<{
    model: UserSupportedModel
    groups: UserAvailableGroup[]
    userGroupRates: Record<number, number>
    pricingKeyPrefix?: string
    noPricingLabel: string
    platformHint?: string
  }>(),
  {
    pricingKeyPrefix: 'availableChannels.pricing',
    platformHint: '',
  },
)

const { t } = useI18n()
const expanded = ref(false)
const tokenScale = 1_000_000

const effectivePlatform = computed(() => props.model.platform || props.platformHint || '')
const badgeClass = computed(() =>
  effectivePlatform.value
    ? platformBadgeClass(effectivePlatform.value)
    : 'border-[var(--app-border)] bg-[var(--app-surface-muted)] text-[var(--app-muted-strong)]',
)

function prefixKey(k: string): string {
  return `${props.pricingKeyPrefix}.${k}`
}

function unitForScale(scale: number): string {
  return scale === tokenScale ? t(prefixKey('unitPerMillion')) : t(prefixKey('unitPerRequest'))
}

function formatPrice(value: number | null, scale: number): string {
  if (value == null) return '-'
  return `${formatScaled(value, scale)} ${unitForScale(scale)}`
}

function formatEffectivePrice(value: number | null, scale: number, rate: number): string {
  if (value == null) return '-'
  return `${formatScaled(value * rate, scale)} ${unitForScale(scale)}`
}

function formatRange(min: number, max: number | null): string {
  const maxLabel = max == null ? t(prefixKey('rangeMax')) : String(max)
  return `(${min}, ${maxLabel}]`
}

function effectiveRate(group: UserAvailableGroup): number {
  return props.userGroupRates[group.id] ?? group.rate_multiplier
}

function intervalHasPrice(iv: UserPricingInterval): boolean {
  return (
    iv.input_price != null ||
    iv.output_price != null ||
    iv.cache_write_price != null ||
    iv.cache_read_price != null ||
    iv.per_request_price != null
  )
}

const billingModeLabel = computed(() => {
  const mode = props.model.pricing?.billing_mode
  switch (mode) {
    case BILLING_MODE_TOKEN:
      return t(prefixKey('billingModeToken'))
    case BILLING_MODE_PER_REQUEST:
      return t(prefixKey('billingModePerRequest'))
    case BILLING_MODE_IMAGE:
      return t(prefixKey('billingModeImage'))
    default:
      return '-'
  }
})

const pricingItems = computed(() => {
  const pricing = props.model.pricing
  if (!pricing) return []
  const items: Array<{ key: string; label: string; value: string }> = [
    {
      key: 'billingMode',
      label: t(prefixKey('billingMode')),
      value: billingModeLabel.value,
    },
  ]
  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    items.push(
      {
        key: 'input',
        label: t(prefixKey('inputPrice')),
        value: formatPrice(pricing.input_price, tokenScale),
      },
      {
        key: 'output',
        label: t(prefixKey('outputPrice')),
        value: formatPrice(pricing.output_price, tokenScale),
      },
      {
        key: 'cacheWrite',
        label: t(prefixKey('cacheWritePrice')),
        value: formatPrice(pricing.cache_write_price, tokenScale),
      },
      {
        key: 'cacheRead',
        label: t(prefixKey('cacheReadPrice')),
        value: formatPrice(pricing.cache_read_price, tokenScale),
      },
    )
    if (pricing.image_output_price != null && pricing.image_output_price > 0) {
      items.push({
        key: 'imageOutput',
        label: t(prefixKey('imageOutputPrice')),
        value: formatPrice(pricing.image_output_price, tokenScale),
      })
    }
  } else if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    items.push({
      key: 'perRequest',
      label: t(prefixKey('perRequestPrice')),
      value: formatPrice(pricing.per_request_price, 1),
    })
  } else if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    items.push({
      key: 'imageOutput',
      label: t(prefixKey('imageOutputPrice')),
      value: formatPrice(pricing.image_output_price, 1),
    })
  }
  return items
})

const priceSummary = computed(() => {
  const pricing = props.model.pricing
  if (!pricing) return props.noPricingLabel
  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    return `${t(prefixKey('inputPrice'))} ${formatScaled(pricing.input_price, tokenScale)} / ${t(prefixKey('outputPrice'))} ${formatScaled(pricing.output_price, tokenScale)}`
  }
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    return `${t(prefixKey('perRequestPrice'))} ${formatScaled(pricing.per_request_price, 1)}`
  }
  if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    return `${t(prefixKey('imageOutputPrice'))} ${formatScaled(pricing.image_output_price, 1)}`
  }
  return billingModeLabel.value
})

function effectivePriceSummary(group: UserAvailableGroup): string {
  const pricing = props.model.pricing
  if (!pricing) return props.noPricingLabel
  const rate = effectiveRate(group)
  const usableIntervals = pricing.intervals?.filter(intervalHasPrice) ?? []
  if (usableIntervals.length > 0) {
    return t('availableChannels.groupRates.intervalMultiplierHint')
  }
  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    return [
      `${t(prefixKey('inputPrice'))} ${formatEffectivePrice(pricing.input_price, tokenScale, rate)}`,
      `${t(prefixKey('outputPrice'))} ${formatEffectivePrice(pricing.output_price, tokenScale, rate)}`,
      `${t(prefixKey('cacheWritePrice'))} ${formatEffectivePrice(pricing.cache_write_price, tokenScale, rate)}`,
      `${t(prefixKey('cacheReadPrice'))} ${formatEffectivePrice(pricing.cache_read_price, tokenScale, rate)}`,
    ].join(' · ')
  }
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    return `${t(prefixKey('perRequestPrice'))} ${formatEffectivePrice(pricing.per_request_price, 1, rate)}`
  }
  if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    return `${t(prefixKey('imageOutputPrice'))} ${formatEffectivePrice(pricing.image_output_price, 1, rate)}`
  }
  return '-'
}
</script>
