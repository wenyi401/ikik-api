<template>
  <section class="endpoint-section" aria-labelledby="endpoint-status-title">
    <header class="section-header">
      <div>
        <h2 id="endpoint-status-title">{{ t('serviceStatus.endpoints.title') }}</h2>
        <p>{{ t('serviceStatus.endpoints.subtitle') }}</p>
      </div>
      <button type="button" class="test-all-button" :disabled="testingAll" @click="$emit('testAll')">
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': testingAll }" />
        <span class="test-all-label">{{ t('serviceStatus.endpoints.testAll') }}</span>
      </button>
    </header>

    <div v-if="targets.length === 0" class="empty-state">
      {{ t('serviceStatus.endpoints.empty') }}
    </div>

    <div v-else class="endpoint-list">
      <article v-for="target in targets" :key="target.id" class="endpoint-row">
        <div class="endpoint-main">
          <div class="endpoint-title-row">
            <strong>{{ target.name || t('serviceStatus.endpoints.defaultName') }}</strong>
            <span v-if="target.isDefault" class="neutral-pill">{{ t('serviceStatus.endpoints.default') }}</span>
            <span v-if="fastestID === target.id" class="fastest-pill">{{ t('serviceStatus.endpoints.fastest') }}</span>
          </div>
          <button
            type="button"
            class="endpoint-url"
            :title="t('serviceStatus.endpoints.copy')"
            @click="copyEndpoint(target.endpoint)"
          >
            <span>{{ target.endpoint }}</span>
            <Icon name="copy" size="xs" />
          </button>
        </div>

        <div class="endpoint-result">
          <span class="result-label" :class="`result-${resultFor(target.id).status}`">
            <span class="result-dot" />
            {{ statusText(resultFor(target.id).status) }}
          </span>
          <span v-if="resultFor(target.id).latencyMs !== null" class="latency">
            {{ resultFor(target.id).latencyMs }} ms
          </span>
          <span v-else-if="resultFor(target.id).checkedAt" class="checked-at">
            {{ formatDateTime(resultFor(target.id).checkedAt) }}
          </span>
        </div>

        <button
          type="button"
          class="test-button"
          :disabled="resultFor(target.id).status === 'testing'"
          @click="$emit('test', target)"
        >
          <Icon
            name="refresh"
            size="sm"
            :class="{ 'animate-spin': resultFor(target.id).status === 'testing' }"
          />
          {{ t('serviceStatus.endpoints.test') }}
        </button>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import type { ServiceEndpointTarget } from '@/utils/serviceStatus'
import type { EndpointDiagnosticResult, EndpointDiagnosticStatus } from '@/composables/useEndpointDiagnostics'

const props = defineProps<{
  targets: ServiceEndpointTarget[]
  results: Record<string, EndpointDiagnosticResult>
  testingAll: boolean
  resultFor: (id: string) => EndpointDiagnosticResult
}>()

defineEmits<{
  test: [target: ServiceEndpointTarget]
  testAll: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const fastestID = computed(() => {
  const available = props.targets
    .map((target) => ({ target, result: props.results[target.id] }))
    .filter((entry) => entry.result?.status === 'online' && entry.result.latencyMs !== null)
    .sort((a, b) => (a.result.latencyMs ?? Infinity) - (b.result.latencyMs ?? Infinity))
  return available[0]?.target.id ?? ''
})

function statusText(status: EndpointDiagnosticStatus): string {
  return t(`serviceStatus.endpoints.status.${status}`)
}

async function copyEndpoint(endpoint: string) {
  try {
    await navigator.clipboard.writeText(endpoint)
    appStore.showSuccess(t('serviceStatus.endpoints.copied'))
  } catch {
    appStore.showError(t('serviceStatus.endpoints.copyFailed'))
  }
}
</script>

<style scoped>
.endpoint-section {
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 16px;
  background: var(--app-surface);
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.125rem;
  border-bottom: 1px solid var(--app-border);
}

.section-header h2 { color: var(--app-text); font-size: 0.9375rem; font-weight: 600; }
.section-header p { margin-top: 0.2rem; color: var(--app-muted); font-size: 0.75rem; }

.test-all-button, .test-button {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  border-radius: 10px;
  font-size: 0.8125rem;
  font-weight: 500;
  transition: background 150ms ease, opacity 150ms ease;
}

.test-all-button { min-height: 2.25rem; padding: 0.5rem 0.75rem; background: var(--app-text); color: var(--app-bg); }
.test-all-button:hover { opacity: 0.86; }
.test-button { min-height: 2rem; padding: 0.375rem 0.625rem; color: var(--app-text); }
.test-button:hover { background: var(--app-surface-muted); }
.test-all-button:disabled, .test-button:disabled { cursor: not-allowed; opacity: 0.5; }

.endpoint-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(9rem, auto) auto;
  align-items: center;
  gap: 1rem;
  padding: 0.875rem 1.125rem;
}

.endpoint-row + .endpoint-row { border-top: 1px solid var(--app-border); }
.endpoint-main { min-width: 0; }
.endpoint-title-row { display: flex; min-width: 0; align-items: center; gap: 0.5rem; }
.endpoint-title-row strong { overflow: hidden; color: var(--app-text); font-size: 0.875rem; font-weight: 550; text-overflow: ellipsis; white-space: nowrap; }

.neutral-pill, .fastest-pill { flex: 0 0 auto; border-radius: 999px; padding: 0.2rem 0.45rem; font-size: 0.625rem; font-weight: 500; }
.neutral-pill { background: var(--app-surface-muted); color: var(--app-muted-strong); }
.fastest-pill { background: #ecfdf5; color: #047857; }
:global(.dark) .fastest-pill { background: rgb(6 78 59 / 28%); color: #6ee7b7; }

.endpoint-url { display: flex; min-width: 0; max-width: 100%; align-items: center; gap: 0.375rem; margin-top: 0.2rem; color: var(--app-muted); }
.endpoint-url span { overflow: hidden; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.6875rem; text-overflow: ellipsis; white-space: nowrap; }
.endpoint-url svg { flex: 0 0 auto; opacity: 0; transition: opacity 150ms ease; }
.endpoint-url:hover { color: var(--app-muted-strong); }
.endpoint-url:hover svg { opacity: 1; }

.endpoint-result { display: flex; min-width: 0; align-items: center; justify-content: flex-end; gap: 0.75rem; }
.result-label { display: inline-flex; align-items: center; gap: 0.375rem; color: var(--app-muted); font-size: 0.75rem; }
.result-dot { width: 0.4rem; height: 0.4rem; border-radius: 999px; background: currentColor; }
.result-online { color: #059669; }
.result-offline { color: #dc2626; }
.result-testing { color: #d97706; }
.latency { color: var(--app-text); font-size: 0.8125rem; font-variant-numeric: tabular-nums; }
.checked-at { color: var(--app-muted); font-size: 0.6875rem; }
.empty-state { padding: 3rem 1rem; text-align: center; color: var(--app-muted); font-size: 0.8125rem; }

@media (max-width: 640px) {
  .section-header { padding: 0.875rem 1rem; }
  .section-header p { display: none; }
  .test-all-button { width: 2.25rem; padding: 0; }
  .test-all-label { display: none; }
  .endpoint-row { grid-template-columns: minmax(0, 1fr) auto; gap: 0.75rem; padding: 1rem; }
  .endpoint-main { grid-column: 1 / -1; }
  .endpoint-result { justify-content: flex-start; }
  .test-button { justify-self: end; }
  .endpoint-url span { white-space: normal; overflow-wrap: anywhere; }
  .endpoint-url svg { opacity: 1; }
}
</style>
