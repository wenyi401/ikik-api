<template>
  <section class="py-6" aria-labelledby="prompt-profiles-title">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h2 id="prompt-profiles-title" class="text-base font-semibold text-gray-950 dark:text-white">
          {{ t('admin.promptAudit.profiles.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ t('admin.promptAudit.profiles.description') }}</p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" @click="$emit('refresh')">
        {{ t('admin.promptAudit.actions.refresh') }}
      </button>
    </div>

    <form class="mt-5 flex flex-col gap-3 sm:flex-row sm:items-end" @submit.prevent="submitSearch">
      <label class="min-w-0 flex-1 text-sm text-gray-700 dark:text-dark-200">
        <span>{{ t('admin.promptAudit.profiles.search') }}</span>
        <input v-model="keyword" class="input mt-1.5 w-full" type="search" :placeholder="t('admin.promptAudit.profiles.searchPlaceholder')" />
      </label>
      <label class="flex min-h-10 items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
        <input v-model="blockedOnly" type="checkbox" />
        {{ t('admin.promptAudit.profiles.blockedOnly') }}
      </label>
      <button type="submit" class="btn btn-primary">{{ t('common.search') }}</button>
    </form>

    <div v-if="error" role="alert" class="mt-5 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">
      {{ error }}
    </div>

    <div class="mt-5 hidden overflow-x-auto md:block">
      <table class="w-full min-w-[900px] text-left text-sm">
        <thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-300">
          <tr>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.profiles.user') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.profiles.risk') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.profiles.auditStats') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.profiles.sampleRate') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.profiles.lastHit') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('admin.promptAudit.common.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
          <tr v-if="loading"><td colspan="6" class="px-3 py-12 text-center text-gray-500">{{ t('common.loading') }}</td></tr>
          <tr v-else-if="items.length === 0"><td colspan="6" class="px-3 py-12 text-center text-gray-500">{{ t('admin.promptAudit.profiles.empty') }}</td></tr>
          <template v-else>
            <tr v-for="item in items" :key="item.user_id">
              <td class="px-3 py-3"><p class="font-medium text-gray-900 dark:text-white">{{ item.user_email || '—' }}</p><p class="mt-0.5 text-xs text-gray-500">ID {{ item.user_id }}</p></td>
              <td class="px-3 py-3"><span :class="riskClass(item)">{{ profileStatus(item) }}</span><p class="mt-1 text-xs text-gray-500">{{ item.risk_score.toFixed(1) }}</p></td>
              <td class="px-3 py-3 text-gray-700 dark:text-dark-200">{{ item.remote_audits }} / {{ item.total_requests }}<p class="mt-1 text-xs text-gray-500">{{ t('admin.promptAudit.profiles.flagged', { count: item.flagged_requests }) }}</p></td>
              <td class="px-3 py-3">{{ item.current_sample_rate }}%</td>
              <td class="px-3 py-3"><p>{{ item.last_category || '—' }}</p><p class="mt-1 text-xs text-gray-500">{{ formatDate(item.last_hit_at) }}</p></td>
              <td class="px-3 py-3 text-right"><button v-if="item.blocked" type="button" class="btn btn-secondary btn-sm" @click="$emit('unblock', item)">{{ t('admin.promptAudit.profiles.unblock') }}</button><span v-else>—</span></td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <div class="mt-5 divide-y divide-gray-200 border-y border-gray-200 md:hidden dark:divide-dark-700 dark:border-dark-700">
      <p v-if="loading" class="py-10 text-center text-sm text-gray-500">{{ t('common.loading') }}</p>
      <p v-else-if="items.length === 0" class="py-10 text-center text-sm text-gray-500">{{ t('admin.promptAudit.profiles.empty') }}</p>
      <template v-else>
        <article v-for="item in items" :key="item.user_id" class="py-4">
          <div class="flex min-w-0 items-start justify-between gap-3">
            <div class="min-w-0"><p class="truncate font-medium text-gray-900 dark:text-white">{{ item.user_email || '—' }}</p><p class="mt-0.5 text-xs text-gray-500">ID {{ item.user_id }}</p></div>
            <span :class="riskClass(item)">{{ profileStatus(item) }}</span>
          </div>
          <dl class="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
            <div><dt class="text-xs text-gray-500">{{ t('admin.promptAudit.profiles.auditStats') }}</dt><dd class="mt-0.5">{{ item.remote_audits }} / {{ item.total_requests }}</dd></div>
            <div><dt class="text-xs text-gray-500">{{ t('admin.promptAudit.profiles.sampleRate') }}</dt><dd class="mt-0.5">{{ item.current_sample_rate }}%</dd></div>
            <div><dt class="text-xs text-gray-500">{{ t('admin.promptAudit.profiles.lastCategory') }}</dt><dd class="mt-0.5 break-words">{{ item.last_category || '—' }}</dd></div>
            <div><dt class="text-xs text-gray-500">{{ t('admin.promptAudit.profiles.lastHit') }}</dt><dd class="mt-0.5">{{ formatDate(item.last_hit_at) }}</dd></div>
          </dl>
          <button v-if="item.blocked" type="button" class="btn btn-secondary btn-sm mt-3" @click="$emit('unblock', item)">{{ t('admin.promptAudit.profiles.unblock') }}</button>
        </article>
      </template>
    </div>

    <div class="mt-4 flex items-center justify-between gap-3 text-sm">
      <span class="text-gray-500">{{ t('admin.promptAudit.profiles.total', { count: total }) }}</span>
      <div class="flex items-center gap-2">
        <button type="button" class="btn btn-secondary btn-sm !bg-transparent" :disabled="page <= 1" @click="$emit('page', page - 1)">{{ t('admin.promptAudit.profiles.previous') }}</button>
        <span>{{ page }} / {{ Math.max(pages, 1) }}</span>
        <button type="button" class="btn btn-secondary btn-sm !bg-transparent" :disabled="page >= pages" @click="$emit('page', page + 1)">{{ t('admin.promptAudit.profiles.next') }}</button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PromptAuditUserProfile } from '../types'

defineProps<{ items: PromptAuditUserProfile[]; total: number; page: number; pages: number; loading: boolean; error: string }>()
const emit = defineEmits<{
  (event: 'search', value: { keyword: string; blockedOnly: boolean }): void
  (event: 'refresh'): void
  (event: 'page', value: number): void
  (event: 'unblock', value: PromptAuditUserProfile): void
}>()
const { t, locale } = useI18n()
const keyword = ref('')
const blockedOnly = ref(true)

function submitSearch() { emit('search', { keyword: keyword.value.trim(), blockedOnly: blockedOnly.value }) }
function profileStatus(item: PromptAuditUserProfile): string {
  return item.blocked ? t('admin.promptAudit.profiles.blocked') : t(`admin.promptAudit.profiles.levels.${item.risk_level}`)
}
function riskClass(item: PromptAuditUserProfile): string {
  const base = 'inline-flex shrink-0 rounded-full px-2 py-1 text-xs font-medium'
  if (item.blocked) return `${base} bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-300`
  if (item.risk_level === 'watch' || item.risk_level === 'high') return `${base} bg-amber-100 text-amber-800 dark:bg-amber-950/50 dark:text-amber-300`
  return `${base} bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200`
}
function formatDate(value?: string): string {
  if (!value) return '—'
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value))
}
</script>
