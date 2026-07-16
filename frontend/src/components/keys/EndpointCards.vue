<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { endpointKey, normalizeEndpointUrl } from '@/utils/apiEndpoints'
import type { CustomEndpoint } from '@/types'

const props = defineProps<{
  apiBaseUrl: string
  customEndpoints: CustomEndpoint[]
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const copiedEndpoint = ref<string | null>(null)
let resetTimer: number | undefined

const endpoints = computed(() => {
  const items: Array<{ name: string; endpoint: string; description: string; isDefault: boolean }> = []
  const seen = new Set<string>()

  const push = (item: { name: string; endpoint: string; description: string; isDefault: boolean }) => {
    const endpoint = normalizeEndpointUrl(item.endpoint)
    if (!endpoint) return
    const key = endpointKey(endpoint)
    if (seen.has(key)) return
    seen.add(key)
    items.push({ ...item, endpoint })
  }

  const defaultEndpoint = props.apiBaseUrl || (typeof window !== 'undefined' ? window.location.origin : '')
  push({
    name: t('keys.endpoints.defaultRoute'),
    endpoint: defaultEndpoint,
    description: '',
    isDefault: true
  })

  for (const endpoint of props.customEndpoints || []) {
    push({
      name: endpoint.name,
      endpoint: endpoint.endpoint,
      description: endpoint.description,
      isDefault: false
    })
  }

  return items
})

async function copyEndpoint(endpoint: string) {
  const success = await copyToClipboard(endpoint, t('keys.endpoints.copied'))
  if (!success) return

  copiedEndpoint.value = endpoint
  if (resetTimer !== undefined) {
    window.clearTimeout(resetTimer)
  }
  resetTimer = window.setTimeout(() => {
    if (copiedEndpoint.value === endpoint) {
      copiedEndpoint.value = null
    }
  }, 1800)
}

onBeforeUnmount(() => {
  if (resetTimer !== undefined) {
    window.clearTimeout(resetTimer)
  }
})
</script>

<template>
  <section
    v-if="endpoints.length"
    class="rounded-2xl border border-[var(--app-border)] bg-[var(--app-surface)] p-4"
  >
    <div class="mb-3 flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h3 class="text-sm font-semibold text-[var(--app-text)]">
          {{ t('keys.endpoints.cardTitle') }}
        </h3>
        <p class="mt-1 text-xs leading-5 text-[var(--app-muted)]">
          {{ t('keys.endpoints.cardDescription') }}
        </p>
      </div>
    </div>

    <div class="grid gap-2 sm:grid-cols-2">
      <div
        v-for="item in endpoints"
        :key="item.endpoint"
        class="flex min-w-0 items-center justify-between gap-3 rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] px-3 py-2.5 transition-colors hover:border-[var(--app-border-strong)] hover:bg-[var(--app-surface-muted)]"
      >
        <div class="min-w-0">
          <div class="flex min-w-0 items-center gap-2">
            <p class="truncate text-sm font-medium text-[var(--app-text)]">
              {{ item.name }}
            </p>
            <span
              v-if="item.isDefault"
              class="shrink-0 rounded-full bg-[var(--app-primary-soft)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--app-primary-hover)]"
            >
              {{ t('keys.endpoints.default') }}
            </span>
          </div>
          <code class="mt-1 block truncate font-mono text-xs text-[var(--app-muted)]">
            {{ item.endpoint }}
          </code>
          <p
            v-if="item.description"
            class="mt-1 line-clamp-2 text-xs leading-5 text-[var(--app-muted)]"
          >
            {{ item.description }}
          </p>
        </div>

        <button
          type="button"
          class="shrink-0 rounded-lg p-2 transition-colors"
          :class="copiedEndpoint === item.endpoint
            ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-300'
            : 'text-[var(--app-muted)] hover:bg-[var(--app-primary-soft)] hover:text-[var(--app-text)]'"
          :title="copiedEndpoint === item.endpoint ? t('keys.endpoints.copiedHint') : t('keys.endpoints.clickToCopy')"
          @click="copyEndpoint(item.endpoint)"
        >
          <Icon
            :name="copiedEndpoint === item.endpoint ? 'check' : 'copy'"
            size="sm"
            :stroke-width="2"
          />
        </button>
      </div>
    </div>
  </section>
</template>
