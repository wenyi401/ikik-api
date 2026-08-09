<template>
  <section class="border-t border-gray-200 py-6 dark:border-dark-700" aria-labelledby="prompt-test-title">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
      <div>
        <h2 id="prompt-test-title" class="text-base font-semibold text-gray-950 dark:text-white">{{ t('admin.promptAudit.test.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ t('admin.promptAudit.test.description') }}</p>
      </div>
      <span class="inline-flex w-fit items-center rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300">
        {{ t('admin.promptAudit.test.noSideEffects') }}
      </span>
    </div>

    <div class="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1.25fr)_minmax(320px,0.75fr)]">
      <div>
        <textarea
          v-model="prompt"
          rows="7"
          class="input min-h-44 w-full resize-y font-mono text-sm leading-6"
          :placeholder="t('admin.promptAudit.test.placeholder')"
          :disabled="loading"
        ></textarea>
        <div class="mt-3 flex justify-end">
          <button type="button" class="btn btn-primary inline-flex items-center gap-2" :disabled="loading || !prompt.trim()" @click="runTest">
            <Icon name="beaker" size="sm" :class="loading ? 'animate-pulse' : ''" />
            {{ loading ? t('admin.promptAudit.test.testing') : t('admin.promptAudit.test.run') }}
          </button>
        </div>
      </div>

      <div class="min-h-44 rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40">
        <div v-if="!result && !error" class="flex h-full min-h-36 items-center justify-center text-sm text-gray-400 dark:text-dark-400">
          {{ t('admin.promptAudit.test.empty') }}
        </div>
        <div v-else-if="error" role="alert" class="text-sm text-red-700 dark:text-red-300">{{ error }}</div>
        <div v-else-if="result" class="space-y-4">
          <div class="flex items-center justify-between gap-3">
            <span class="result-badge" :class="decisionClass(result.result.decision)">{{ decisionLabel(result.result.decision) }}</span>
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ result.latency_ms }} ms / {{ t('admin.promptAudit.test.chunks', { count: result.chunk_total }) }}</span>
          </div>
          <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
            <div>
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.promptAudit.test.action') }}</dt>
              <dd class="mt-1 font-medium text-gray-900 dark:text-white">{{ t(`admin.promptAudit.actions.${result.result.action}`) }}</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.promptAudit.test.safety') }}</dt>
              <dd class="mt-1 font-medium text-gray-900 dark:text-white">{{ result.result.safety || '-' }}</dd>
            </div>
          </dl>
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.promptAudit.test.categories') }}</p>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <span v-for="category in result.result.categories" :key="category" class="rounded-md bg-white px-2 py-1 text-xs text-gray-700 dark:bg-dark-700 dark:text-dark-100">
                {{ categoryLabel(category) }}
              </span>
              <span v-if="result.result.categories.length === 0" class="text-sm text-gray-400">-</span>
            </div>
          </div>
          <p class="truncate text-xs text-gray-500 dark:text-dark-400" :title="result.result.guard_endpoint_id">
            {{ result.result.guard_endpoint_id || '-' }} / {{ result.result.scanner_backend || '-' }}
          </p>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import promptAuditAPI from '../api'
import type { PromptAuditTestResult, PromptDecision } from '../types'

const { t } = useI18n()
const prompt = ref('')
const loading = ref(false)
const error = ref('')
const result = ref<PromptAuditTestResult | null>(null)

async function runTest() {
  if (!prompt.value.trim() || loading.value) return
  loading.value = true
  error.value = ''
  result.value = null
  try {
    result.value = await promptAuditAPI.testPrompt(prompt.value.trim())
  } catch (caught: unknown) {
    error.value = extractApiErrorMessage(caught, t('admin.promptAudit.errors.test'))
  } finally {
    loading.value = false
  }
}

function decisionLabel(value: PromptDecision) { return t(`admin.promptAudit.decisions.${value}`) }
function decisionClass(value: PromptDecision) {
  if (value === 'critical') return 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300'
  if (value === 'flag') return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300'
  return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
}
function categoryLabel(category: string) {
  const key = `admin.promptAudit.scanners.${category}`
  const translated = t(key)
  return translated === key ? category : translated
}
</script>

<style scoped>
.result-badge { display: inline-flex; min-height: 1.75rem; align-items: center; border-radius: 999px; padding: 0.2rem 0.65rem; font-size: 0.75rem; font-weight: 700; }
</style>
