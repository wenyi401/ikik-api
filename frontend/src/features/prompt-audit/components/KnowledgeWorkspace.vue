<template>
  <section class="py-6" data-test="knowledge-workspace">
    <div class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-200 pb-5 dark:border-dark-700">
      <div>
        <div class="flex items-center gap-2">
          <h2 class="text-lg font-semibold text-gray-950 dark:text-white">{{ t('admin.promptAudit.knowledge.title') }}</h2>
          <span class="rounded-md bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300">
            {{ t('admin.promptAudit.knowledge.shadowOnly') }}
          </span>
        </div>
        <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-dark-300">{{ t('admin.promptAudit.knowledge.description') }}</p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="loadAll">
        {{ t('admin.promptAudit.knowledge.refresh') }}
      </button>
    </div>

    <div v-if="summary" class="grid grid-cols-2 border-b border-gray-200 sm:grid-cols-3 lg:grid-cols-6 dark:border-dark-700">
      <div v-for="metric in summaryMetrics" :key="metric.label" class="border-r border-gray-200 px-3 py-4 last:border-r-0 dark:border-dark-700">
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ metric.label }}</p>
        <p class="mt-1 text-xl font-semibold tabular-nums text-gray-950 dark:text-white">{{ metric.value }}</p>
      </div>
    </div>

    <div v-if="error" role="alert" class="mt-5 border-l-2 border-red-500 bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">
      {{ error }}
    </div>

    <div class="mt-5 flex items-center gap-1 border-b border-gray-200 dark:border-dark-700" role="tablist">
      <button
        v-for="tab in workspaceTabs"
        :key="tab.id"
        type="button"
        role="tab"
        class="border-b-2 px-4 py-2.5 text-sm font-medium"
        :class="workspaceTab === tab.id ? 'border-primary-600 text-primary-700 dark:text-primary-300' : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-dark-300 dark:hover:text-white'"
        :aria-selected="workspaceTab === tab.id"
        :data-test="`knowledge-section-${tab.id}`"
        @click="workspaceTab = tab.id"
      >
        {{ tab.label }}
        <span v-if="tab.id === 'observations' && summary?.unreviewed_observations" class="ml-1 rounded-full bg-amber-100 px-1.5 py-0.5 text-[11px] text-amber-800 dark:bg-amber-950/50 dark:text-amber-200">
          {{ summary.unreviewed_observations }}
        </span>
      </button>
    </div>

    <div v-show="workspaceTab === 'observations'" class="pt-5">
      <form class="grid gap-3 md:grid-cols-[180px_220px_minmax(220px,1fr)_auto]" @submit.prevent="searchObservations">
        <label class="text-xs font-medium text-gray-600 dark:text-dark-200">
          {{ t('admin.promptAudit.knowledge.reviewStatus') }}
          <select v-model="observationFilters.review_status" class="input mt-1 w-full">
            <option value="">{{ t('admin.promptAudit.knowledge.all') }}</option>
            <option v-for="status in reviewStatuses" :key="status" :value="status">{{ reviewStatusLabel(status) }}</option>
          </select>
        </label>
        <label class="text-xs font-medium text-gray-600 dark:text-dark-200">
          {{ t('admin.promptAudit.knowledge.category') }}
          <select v-model="observationFilters.category" class="input mt-1 w-full">
            <option value="">{{ t('admin.promptAudit.knowledge.all') }}</option>
            <option v-for="category in riskCategories" :key="category" :value="category">{{ categoryLabel(category) }}</option>
          </select>
        </label>
        <label class="text-xs font-medium text-gray-600 dark:text-dark-200">
          {{ t('admin.promptAudit.knowledge.search') }}
          <input v-model.trim="observationFilters.keyword" class="input mt-1 w-full" type="search" :placeholder="t('admin.promptAudit.knowledge.searchObservationPlaceholder')" />
        </label>
        <button type="submit" class="btn btn-primary btn-sm self-end">{{ t('common.search') }}</button>
      </form>

      <div v-if="loadingObservations" class="py-14 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('common.loading') }}</div>
      <div v-else-if="observations.items.length === 0" class="py-14 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('admin.promptAudit.knowledge.noObservations') }}</div>
      <div v-else class="mt-5 divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-700 dark:border-dark-700">
        <article v-for="item in observations.items" :key="item.id" class="grid gap-4 py-5 xl:grid-cols-[minmax(0,1fr)_260px_250px]">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2 text-xs">
              <span :class="reviewStatusClass(item.review_status)" class="rounded-md px-2 py-1 font-medium">{{ reviewStatusLabel(item.review_status) }}</span>
              <span class="text-gray-500 dark:text-dark-400">#{{ item.id }}</span>
              <span class="text-gray-500 dark:text-dark-400">{{ formatDate(item.observed_at) }}</span>
              <span v-if="item.audit.username" class="text-gray-700 dark:text-dark-200">{{ item.audit.username }}</span>
              <span v-if="item.audit.group_name" class="text-gray-500 dark:text-dark-400">{{ item.audit.group_name }}</span>
            </div>
            <p class="mt-3 whitespace-pre-wrap break-words text-sm leading-6 text-gray-900 dark:text-dark-100">{{ observationPrompt(item) }}</p>
            <div v-if="item.candidate.knowledge_matches?.length" class="mt-3 flex flex-wrap gap-2">
              <span v-for="match in item.candidate.knowledge_matches.slice(0, 3)" :key="match.entry_id" class="rounded-md bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                {{ topicLabel(match.topic) }} · {{ Math.round(match.score * 100) }}%
              </span>
            </div>
          </div>

          <dl class="grid content-start grid-cols-2 gap-x-4 gap-y-2 text-xs">
            <dt class="text-gray-500 dark:text-dark-400">{{ t('admin.promptAudit.knowledge.knowledgeDecision') }}</dt>
            <dd class="text-right font-medium text-gray-800 dark:text-dark-100">{{ categoryLabel(item.adjudication.category) }}</dd>
            <dt class="text-gray-500 dark:text-dark-400">{{ t('admin.promptAudit.knowledge.confidence') }}</dt>
            <dd class="text-right tabular-nums text-gray-800 dark:text-dark-100">{{ Math.round(item.adjudication.confidence * 100) }}%</dd>
            <dt class="text-gray-500 dark:text-dark-400">Qwen3Guard</dt>
            <dd class="text-right text-gray-800 dark:text-dark-100">{{ item.audit.guard_decision || '-' }} / {{ item.audit.guard_risk_level || '-' }}</dd>
            <dt class="text-gray-500 dark:text-dark-400">{{ t('admin.promptAudit.knowledge.simulation') }}</dt>
            <dd class="text-right font-medium text-emerald-700 dark:text-emerald-300">{{ t('admin.promptAudit.knowledge.notExecuted') }}</dd>
          </dl>

          <div class="flex flex-wrap content-start gap-2 xl:justify-end">
            <template v-if="item.review_status === 'unreviewed'">
              <button type="button" class="btn btn-primary btn-sm" data-test="confirm-risk" @click="openReview(item, 'confirmed')">{{ t('admin.promptAudit.knowledge.confirmRisk') }}</button>
              <button type="button" class="btn btn-secondary btn-sm" @click="openReview(item, 'false_positive')">{{ t('admin.promptAudit.knowledge.markFalsePositive') }}</button>
              <button type="button" class="btn btn-ghost btn-sm" @click="openReview(item, 'inconclusive')">{{ t('admin.promptAudit.knowledge.markInconclusive') }}</button>
            </template>
            <button type="button" class="btn btn-ghost btn-sm" @click="openEntryFromObservation(item)">{{ t('admin.promptAudit.knowledge.addAsCase') }}</button>
          </div>
        </article>
      </div>
      <PageControls :page="observations.page" :pages="observations.pages" :total="observations.total" @page="changeObservationPage" />
    </div>

    <div v-show="workspaceTab === 'entries'" class="pt-5">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <form class="grid flex-1 gap-3 md:grid-cols-[220px_180px_minmax(220px,1fr)_auto]" @submit.prevent="searchEntries">
          <label class="text-xs font-medium text-gray-600 dark:text-dark-200">
            {{ t('admin.promptAudit.knowledge.topic') }}
            <select v-model="entryFilters.topic" class="input mt-1 w-full">
              <option value="">{{ t('admin.promptAudit.knowledge.all') }}</option>
              <option v-for="topic in topics" :key="topic" :value="topic">{{ topicLabel(topic) }}</option>
            </select>
          </label>
          <label class="text-xs font-medium text-gray-600 dark:text-dark-200">
            {{ t('admin.promptAudit.knowledge.disposition') }}
            <select v-model="entryFilters.disposition" class="input mt-1 w-full">
              <option value="">{{ t('admin.promptAudit.knowledge.all') }}</option>
              <option value="risk">{{ dispositionLabel('risk') }}</option>
              <option value="safe">{{ dispositionLabel('safe') }}</option>
              <option value="review">{{ dispositionLabel('review') }}</option>
            </select>
          </label>
          <label class="text-xs font-medium text-gray-600 dark:text-dark-200">
            {{ t('admin.promptAudit.knowledge.search') }}
            <input v-model.trim="entryFilters.keyword" class="input mt-1 w-full" type="search" :placeholder="t('admin.promptAudit.knowledge.searchCasePlaceholder')" />
          </label>
          <button type="submit" class="btn btn-secondary btn-sm self-end">{{ t('common.search') }}</button>
        </form>
        <button type="button" class="btn btn-primary btn-sm" data-test="create-knowledge" @click="openCreateEntry">
          <span aria-hidden="true">+</span> {{ t('admin.promptAudit.knowledge.createCase') }}
        </button>
      </div>

      <div v-if="loadingEntries" class="py-14 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('common.loading') }}</div>
      <div v-else-if="entries.items.length === 0" class="py-14 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('admin.promptAudit.knowledge.noCases') }}</div>
      <div v-else class="mt-5 overflow-x-auto border-y border-gray-200 dark:border-dark-700">
        <table class="min-w-full divide-y divide-gray-200 text-left text-sm dark:divide-dark-700">
          <thead class="text-xs text-gray-500 dark:text-dark-400">
            <tr>
              <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.knowledge.case') }}</th>
              <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.knowledge.topic') }}</th>
              <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.knowledge.disposition') }}</th>
              <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.knowledge.status') }}</th>
              <th class="px-3 py-3 text-right font-medium">{{ t('admin.promptAudit.common.actions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
            <tr v-for="entry in entries.items" :key="entry.id">
              <td class="max-w-xl px-3 py-4 align-top">
                <p class="font-medium text-gray-900 dark:text-dark-100">{{ entry.title }}</p>
                <p class="mt-1 line-clamp-2 whitespace-pre-wrap text-xs leading-5 text-gray-500 dark:text-dark-400">{{ entry.example_text }}</p>
              </td>
              <td class="px-3 py-4 align-top text-xs text-gray-700 dark:text-dark-200">{{ topicLabel(entry.topic) }}</td>
              <td class="px-3 py-4 align-top"><span :class="dispositionClass(entry.disposition)" class="rounded-md px-2 py-1 text-xs font-medium">{{ dispositionLabel(entry.disposition) }}</span></td>
              <td class="px-3 py-4 align-top text-xs" :class="entry.enabled ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-400'">{{ entry.enabled ? t('admin.promptAudit.knowledge.enabled') : t('admin.promptAudit.knowledge.disabled') }}</td>
              <td class="px-3 py-4 text-right align-top"><button type="button" class="btn btn-ghost btn-sm" @click="openEditEntry(entry)">{{ t('common.edit') }}</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <PageControls :page="entries.page" :pages="entries.pages" :total="entries.total" @page="changeEntryPage" />
    </div>

    <div v-if="reviewTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" role="dialog" aria-modal="true">
      <div class="w-full max-w-xl rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900">
        <h3 class="text-lg font-semibold text-gray-950 dark:text-white">{{ reviewStatusLabel(reviewDraft.status) }}</h3>
        <p class="mt-1 line-clamp-3 whitespace-pre-wrap text-sm text-gray-500 dark:text-dark-300">{{ observationPrompt(reviewTarget) }}</p>
        <div v-if="reviewDraft.status !== 'inconclusive'" class="mt-5 grid gap-4 sm:grid-cols-2">
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.topic') }}
            <select v-model="reviewDraft.topic" class="input mt-1 w-full"><option v-for="topic in topics" :key="topic" :value="topic">{{ topicLabel(topic) }}</option></select>
          </label>
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.category') }}
            <select v-model="reviewDraft.category" class="input mt-1 w-full"><option v-for="category in riskCategories" :key="category" :value="category">{{ categoryLabel(category) }}</option></select>
          </label>
        </div>
        <label class="mt-4 block text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.reviewNote') }}
          <textarea v-model="reviewDraft.note" maxlength="500" class="input mt-1 min-h-24 w-full resize-y" :placeholder="t('admin.promptAudit.knowledge.reviewNotePlaceholder')" />
        </label>
        <p class="mt-3 text-xs text-emerald-700 dark:text-emerald-300">{{ t('admin.promptAudit.knowledge.reviewNoEnforcement') }}</p>
        <div class="mt-5 flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="savingReview" @click="reviewTarget = null">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" data-test="save-review" :disabled="savingReview" @click="submitReview">{{ savingReview ? t('common.loading') : t('common.confirm') }}</button>
        </div>
      </div>
    </div>

    <div v-if="entryEditor" class="fixed inset-0 z-50 overflow-y-auto bg-black/50 p-4" role="dialog" aria-modal="true">
      <div class="mx-auto my-4 w-full max-w-3xl rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900">
        <div class="flex items-start justify-between gap-4">
          <div><h3 class="text-lg font-semibold text-gray-950 dark:text-white">{{ entryEditor.id ? t('admin.promptAudit.knowledge.editCase') : t('admin.promptAudit.knowledge.createCase') }}</h3><p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ t('admin.promptAudit.knowledge.caseEditorHint') }}</p></div>
          <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200"><input v-model="entryEditor.enabled" type="checkbox" /> {{ t('admin.promptAudit.knowledge.enabled') }}</label>
        </div>
        <div class="mt-5 grid gap-4 sm:grid-cols-2">
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.titleLabel') }}<input v-model.trim="entryEditor.title" maxlength="160" class="input mt-1 w-full" data-test="knowledge-title" /></label>
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.language') }}<select v-model="entryEditor.language" class="input mt-1 w-full"><option value="zh">{{ t('admin.promptAudit.knowledge.chinese') }}</option><option value="en">{{ t('admin.promptAudit.knowledge.english') }}</option><option value="multilingual">{{ t('admin.promptAudit.knowledge.multilingual') }}</option></select></label>
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.topic') }}<select v-model="entryEditor.topic" class="input mt-1 w-full"><option v-for="topic in topics" :key="topic" :value="topic">{{ topicLabel(topic) }}</option></select></label>
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.category') }}<select v-model="entryEditor.category" class="input mt-1 w-full"><option v-for="category in allCategories" :key="category" :value="category">{{ categoryLabel(category) }}</option></select></label>
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.disposition') }}<select v-model="entryEditor.disposition" class="input mt-1 w-full"><option value="risk">{{ dispositionLabel('risk') }}</option><option value="safe">{{ dispositionLabel('safe') }}</option><option value="review">{{ dispositionLabel('review') }}</option></select></label>
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.intent') }}<select v-model="entryEditor.intent" class="input mt-1 w-full"><option v-for="value in intents" :key="value" :value="value">{{ t(`admin.promptAudit.knowledge.intents.${value}`) }}</option></select></label>
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.actionability') }}<select v-model="entryEditor.actionability" class="input mt-1 w-full"><option v-for="value in actionabilities" :key="value" :value="value">{{ t(`admin.promptAudit.knowledge.actionabilities.${value}`) }}</option></select></label>
          <label class="text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.authorization') }}<select v-model="entryEditor.authorization" class="input mt-1 w-full"><option v-for="value in authorizations" :key="value" :value="value">{{ t(`admin.promptAudit.knowledge.authorizations.${value}`) }}</option></select></label>
        </div>
        <label class="mt-4 block text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.example') }}<textarea v-model="entryEditor.example_text" maxlength="2000" class="input mt-1 min-h-32 w-full resize-y font-mono text-sm" /></label>
        <label class="mt-4 block text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.aliases') }}<textarea v-model="entryEditor.aliases_text" class="input mt-1 min-h-20 w-full resize-y" :placeholder="t('admin.promptAudit.knowledge.aliasesHint')" /></label>
        <label class="mt-4 block text-sm text-gray-700 dark:text-dark-200">{{ t('admin.promptAudit.knowledge.rationale') }}<textarea v-model="entryEditor.rationale" maxlength="1000" class="input mt-1 min-h-20 w-full resize-y" /></label>
        <div class="mt-5 flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="savingEntry" @click="entryEditor = null">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" data-test="save-knowledge" :disabled="savingEntry || !entryEditor.title || !entryEditor.example_text.trim()" @click="saveEntry">{{ savingEntry ? t('common.loading') : t('common.save') }}</button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import promptAuditAPI from '../api'
import type {
  RiskDomainCategory, RiskKnowledgeActionability, RiskKnowledgeAuthorization,
  RiskKnowledgeDisposition, RiskKnowledgeEntry, RiskKnowledgeEntryPage, RiskKnowledgeIntent,
  RiskKnowledgeSummary, RiskKnowledgeTopic, RiskKnowledgeWriteInput, RiskObservation,
  RiskObservationPage, RiskObservationReviewInput, RiskObservationReviewStatus,
} from '../types'

const props = defineProps<{ active: boolean }>()
const { t, locale } = useI18n()
const appStore = useAppStore()
const topics: RiskKnowledgeTopic[] = ['cheat_development', 'cheat_usage', 'reverse_engineering', 'license_cracking', 'detection_bypass', 'account_automation', 'third_party_scripts', 'credential_abuse', 'benign_research']
const allCategories: RiskDomainCategory[] = ['none', 'cheat_automation', 'auth_reverse_engineering', 'exploit_reverse_engineering', 'credential_theft', 'safety_bypass', 'account_automation', 'cyber_abuse']
const riskCategories = allCategories.filter((value) => value !== 'none')
const reviewStatuses: RiskObservationReviewStatus[] = ['unreviewed', 'confirmed', 'false_positive', 'inconclusive']
const intents: RiskKnowledgeIntent[] = ['neutral', 'educational', 'defensive', 'operational', 'evasion', 'unknown']
const actionabilities: RiskKnowledgeActionability[] = ['none', 'low', 'medium', 'high']
const authorizations: RiskKnowledgeAuthorization[] = ['authorized', 'unauthorized', 'unknown']

const workspaceTab = ref<'observations' | 'entries'>('observations')
const summary = ref<RiskKnowledgeSummary | null>(null)
const observations = reactive<RiskObservationPage>({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
const entries = reactive<RiskKnowledgeEntryPage>({ items: [], total: 0, page: 1, page_size: 20, pages: 0, version: 0 })
const observationFilters = reactive({ review_status: 'unreviewed', category: '', keyword: '' })
const appliedObservationFilters = reactive({ review_status: 'unreviewed', category: '', keyword: '' })
const entryFilters = reactive({ topic: '', disposition: '', keyword: '' })
const appliedEntryFilters = reactive({ topic: '', disposition: '', keyword: '' })
const loading = ref(false)
const loadingObservations = ref(false)
const loadingEntries = ref(false)
const savingReview = ref(false)
const savingEntry = ref(false)
const error = ref('')
const loaded = ref(false)
const reviewTarget = ref<RiskObservation | null>(null)
const reviewDraft = reactive<{ status: Exclude<RiskObservationReviewStatus, 'unreviewed'>; topic: RiskKnowledgeTopic; category: RiskDomainCategory; note: string }>({ status: 'confirmed', topic: 'cheat_development', category: 'cheat_automation', note: '' })
type EntryDraft = RiskKnowledgeWriteInput & { id?: number; aliases_text: string }
const entryEditor = ref<EntryDraft | null>(null)

const workspaceTabs = computed(() => [
  { id: 'observations' as const, label: t('admin.promptAudit.knowledge.observations') },
  { id: 'entries' as const, label: t('admin.promptAudit.knowledge.caseLibrary') },
])
const summaryMetrics = computed(() => summary.value ? [
  { label: t('admin.promptAudit.knowledge.version'), value: `v${summary.value.version}` },
  { label: t('admin.promptAudit.knowledge.enabledCases'), value: summary.value.enabled },
  { label: t('admin.promptAudit.knowledge.riskCases'), value: summary.value.risk },
  { label: t('admin.promptAudit.knowledge.safeCases'), value: summary.value.safe },
  { label: t('admin.promptAudit.knowledge.reviewCases'), value: summary.value.review },
  { label: t('admin.promptAudit.knowledge.pendingReviews'), value: summary.value.unreviewed_observations },
] : [])

const PageControls = defineComponent({
  props: { page: { type: Number, required: true }, pages: { type: Number, required: true }, total: { type: Number, required: true } },
  emits: ['page'],
  setup(componentProps, { emit }) {
    return () => h('div', { class: 'mt-5 flex items-center justify-between gap-3 text-sm text-gray-500 dark:text-dark-300' }, [
      h('span', t('admin.promptAudit.knowledge.total', { count: componentProps.total })),
      h('div', { class: 'flex items-center gap-2' }, [
        h('button', { type: 'button', class: 'btn btn-secondary btn-sm', disabled: componentProps.page <= 1, onClick: () => emit('page', componentProps.page - 1) }, t('admin.promptAudit.profiles.previous')),
        h('span', { class: 'min-w-16 text-center tabular-nums' }, `${componentProps.page} / ${Math.max(componentProps.pages, 1)}`),
        h('button', { type: 'button', class: 'btn btn-secondary btn-sm', disabled: componentProps.page >= componentProps.pages, onClick: () => emit('page', componentProps.page + 1) }, t('admin.promptAudit.profiles.next')),
      ]),
    ])
  },
})

function topicLabel(value: string) { return t(`admin.promptAudit.knowledge.topics.${value}`) }
function categoryLabel(value: string) { return t(`admin.promptAudit.knowledge.categories.${value}`) }
function dispositionLabel(value: string) { return t(`admin.promptAudit.knowledge.dispositions.${value}`) }
function reviewStatusLabel(value: string) { return t(`admin.promptAudit.knowledge.reviewStatuses.${value}`) }
function dispositionClass(value: RiskKnowledgeDisposition) {
  if (value === 'risk') return 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300'
  if (value === 'safe') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300'
}
function reviewStatusClass(value: RiskObservationReviewStatus) {
  if (value === 'confirmed') return 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300'
  if (value === 'false_positive') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
  if (value === 'inconclusive') return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300'
}
function observationPrompt(item: RiskObservation) { return item.audit.full_prompt || item.audit.redacted_preview || t('admin.promptAudit.knowledge.promptUnavailable') }
function formatDate(value: string) { return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
function apiError(value: unknown) { return extractApiErrorMessage(value, t('admin.promptAudit.knowledge.loadFailed')) }

async function loadSummary() { summary.value = await promptAuditAPI.getKnowledgeSummary() }
async function loadObservations() {
  loadingObservations.value = true
  try {
    Object.assign(observations, await promptAuditAPI.listKnowledgeObservations({
      page: observations.page, page_size: observations.page_size,
      review_status: appliedObservationFilters.review_status || undefined,
      category: appliedObservationFilters.category || undefined,
      keyword: appliedObservationFilters.keyword || undefined,
    }))
  } finally { loadingObservations.value = false }
}
async function loadEntries() {
  loadingEntries.value = true
  try {
    Object.assign(entries, await promptAuditAPI.listKnowledge({
      page: entries.page, page_size: entries.page_size,
      topic: appliedEntryFilters.topic || undefined,
      disposition: appliedEntryFilters.disposition || undefined,
      keyword: appliedEntryFilters.keyword || undefined,
    }))
  } finally { loadingEntries.value = false }
}
async function loadAll() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    await Promise.all([loadSummary(), loadObservations(), loadEntries()])
    loaded.value = true
  } catch (value) { error.value = apiError(value) }
  finally { loading.value = false }
}
function searchObservations() {
  Object.assign(appliedObservationFilters, observationFilters)
  observations.page = 1
  void loadObservations().catch((value) => { error.value = apiError(value) })
}
function searchEntries() {
  Object.assign(appliedEntryFilters, entryFilters)
  entries.page = 1
  void loadEntries().catch((value) => { error.value = apiError(value) })
}
function changeObservationPage(page: number) { observations.page = page; void loadObservations().catch((value) => { error.value = apiError(value) }) }
function changeEntryPage(page: number) { entries.page = page; void loadEntries().catch((value) => { error.value = apiError(value) }) }

function defaultTopic(item: RiskObservation): RiskKnowledgeTopic { return item.candidate.knowledge_matches?.[0]?.topic || 'cheat_development' }
function defaultCategory(item: RiskObservation): RiskDomainCategory {
  const value = item.adjudication.category || item.candidate.knowledge_matches?.[0]?.category
  return value && value !== 'none' ? value : 'cheat_automation'
}
function openReview(item: RiskObservation, status: Exclude<RiskObservationReviewStatus, 'unreviewed'>) {
  reviewTarget.value = item
  reviewDraft.status = status
  reviewDraft.topic = defaultTopic(item)
  reviewDraft.category = defaultCategory(item)
  reviewDraft.note = ''
}
async function submitReview() {
  if (!reviewTarget.value || savingReview.value) return
  savingReview.value = true
  const input: RiskObservationReviewInput = { status: reviewDraft.status, note: reviewDraft.note }
  if (reviewDraft.status !== 'inconclusive') {
    input.topic = reviewDraft.topic
    input.category = reviewDraft.category
  }
  try {
    await promptAuditAPI.reviewKnowledgeObservation(reviewTarget.value.id, input)
    reviewTarget.value = null
    appStore.showSuccess(t('admin.promptAudit.knowledge.reviewSaved'))
    await Promise.all([loadSummary(), loadObservations()])
  } catch (value) { appStore.showError(apiError(value)) }
  finally { savingReview.value = false }
}

function freshEntry(): EntryDraft {
  return {
    topic: 'cheat_development', category: 'cheat_automation', disposition: 'risk',
    intent: 'operational', actionability: 'high', authorization: 'unknown', language: 'zh',
    title: '', example_text: '', aliases: [], aliases_text: '', rationale: '', enabled: true,
    source_type: 'admin',
  }
}
function openCreateEntry() { entryEditor.value = freshEntry() }
function openEditEntry(entry: RiskKnowledgeEntry) {
  entryEditor.value = { ...entry, aliases: [...entry.aliases], aliases_text: entry.aliases.join('\n') }
}
function openEntryFromObservation(item: RiskObservation) {
  const disposition: RiskKnowledgeDisposition = item.review_status === 'false_positive' || item.adjudication.verdict === 'safe' ? 'safe' : 'risk'
  entryEditor.value = {
    ...freshEntry(), topic: defaultTopic(item), category: defaultCategory(item), disposition,
    intent: disposition === 'safe' ? 'defensive' : item.adjudication.intent,
    actionability: disposition === 'safe' ? 'low' : item.adjudication.actionability,
    authorization: disposition === 'safe' ? 'authorized' : item.adjudication.authorization,
    title: t('admin.promptAudit.knowledge.observationCaseTitle', { id: item.id }),
    example_text: observationPrompt(item), source_type: 'audit_review', source_event_id: item.audit.event_id,
    rationale: item.review_note,
  }
  workspaceTab.value = 'entries'
}
async function saveEntry() {
  const editor = entryEditor.value
  if (!editor || savingEntry.value) return
  savingEntry.value = true
  const input: RiskKnowledgeWriteInput = {
    topic: editor.topic, category: editor.category, disposition: editor.disposition,
    intent: editor.intent, actionability: editor.actionability, authorization: editor.authorization,
    language: editor.language, title: editor.title.trim(), example_text: editor.example_text.trim(),
    aliases: editor.aliases_text.split(/[\n,，]/).map((value) => value.trim()).filter(Boolean),
    rationale: editor.rationale.trim(), enabled: editor.enabled, source_type: editor.source_type,
    source_event_id: editor.source_event_id,
  }
  try {
    if (editor.id) await promptAuditAPI.updateKnowledge(editor.id, input)
    else await promptAuditAPI.createKnowledge(input)
    entryEditor.value = null
    appStore.showSuccess(t('admin.promptAudit.knowledge.caseSaved'))
    await Promise.all([loadSummary(), loadEntries()])
  } catch (value) { appStore.showError(apiError(value)) }
  finally { savingEntry.value = false }
}

watch(() => props.active, (active) => { if (active && !loaded.value) void loadAll() }, { immediate: true })
</script>
