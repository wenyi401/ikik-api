<template>
  <section
    class="overflow-hidden rounded-lg border border-[#d9d9e3] bg-[#ffffff] shadow-[0_18px_48px_rgba(0,0,0,0.08)] dark:border-[#3f3f46] dark:bg-[#212121]"
  >
    <div class="border-b border-[#d9d9e3] bg-[#f3f3f6] px-6 py-6 dark:border-[#3f3f46] dark:bg-[#171717]">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center">
        <div
          class="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg border border-[#d9d9e3] bg-[#ffffff] text-[#10a37f] dark:border-[#565869] dark:bg-[#2f2f2f] dark:text-[#45d09a]"
        >
          <Icon name="gift" size="lg" />
        </div>
        <div>
          <p class="text-xs font-semibold uppercase text-[#10a37f] dark:text-[#45d09a]">
            {{ t('payment.externalPurchase.kicker') }}
          </p>
          <h1 class="mt-1 text-2xl font-semibold text-[#171717] dark:text-[#ececf1]">
            {{ t('payment.externalPurchase.title') }}
          </h1>
          <p class="mt-2 max-w-2xl text-sm leading-6 text-[#6e6e80] dark:text-[#c5c5d2]">
            {{ t('payment.externalPurchase.description') }}
          </p>
        </div>
      </div>
    </div>

    <div class="grid gap-4 p-6 sm:grid-cols-3">
      <div
        v-for="step in steps"
        :key="step.title"
        class="rounded-lg border border-[#d9d9e3] bg-[#ffffff] p-4 dark:border-[#3f3f46] dark:bg-[#212121]"
      >
        <div class="mb-3 flex h-8 w-8 items-center justify-center rounded-md bg-[#e6f6f1] text-sm font-semibold text-[#0d8f70] dark:bg-[#2f2f2f] dark:text-[#45d09a]">
          {{ step.index }}
        </div>
        <h2 class="text-sm font-semibold text-[#171717] dark:text-[#ececf1]">
          {{ step.title }}
        </h2>
        <p class="mt-2 text-sm leading-6 text-[#6e6e80] dark:text-[#c5c5d2]">
          {{ step.description }}
        </p>
      </div>
    </div>

    <div class="flex flex-col gap-3 border-t border-[#d9d9e3] bg-[#f3f3f6] p-6 dark:border-[#3f3f46] dark:bg-[#171717] sm:flex-row">
      <button
        type="button"
        class="inline-flex flex-1 items-center justify-center gap-2 rounded-lg bg-[#171717] px-5 py-3 text-sm font-semibold text-[#ffffff] shadow-[0_10px_24px_rgba(0,0,0,0.18)] transition hover:bg-black dark:bg-[#ececf1] dark:text-[#171717] dark:hover:bg-white"
        @click="openStore"
      >
        <Icon name="externalLink" size="sm" />
        {{ t('payment.externalPurchase.openStore') }}
      </button>
      <router-link
        to="/redeem"
        class="inline-flex flex-1 items-center justify-center gap-2 rounded-lg border border-[#c5c5d2] bg-[#ffffff] px-5 py-3 text-sm font-semibold text-[#3f3f46] transition hover:bg-white dark:border-[#565869] dark:bg-[#2f2f2f] dark:text-[#ececf1] dark:hover:bg-[#2f2f2f]"
      >
        <Icon name="checkCircle" size="sm" />
        {{ t('payment.externalPurchase.redeemCode') }}
      </router-link>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  purchaseUrl: string
}>()

const { t } = useI18n()

const steps = computed(() => [
  {
    index: '1',
    title: t('payment.externalPurchase.stepBuyTitle'),
    description: t('payment.externalPurchase.stepBuyDescription'),
  },
  {
    index: '2',
    title: t('payment.externalPurchase.stepReceiveTitle'),
    description: t('payment.externalPurchase.stepReceiveDescription'),
  },
  {
    index: '3',
    title: t('payment.externalPurchase.stepRedeemTitle'),
    description: t('payment.externalPurchase.stepRedeemDescription'),
  },
])

function openStore() {
  if (!props.purchaseUrl || typeof window === 'undefined') return
  window.open(props.purchaseUrl, '_blank', 'noopener,noreferrer')
}
</script>
