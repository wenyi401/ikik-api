<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <UiPageHeader :title="t('promptLibrary.title')">
        <template #actions>
          <button type="button" class="prompt-submit-button" @click="submitDialogOpen = true">
            <Icon name="plus" size="sm" />
            {{ t('promptLibrary.submit.action') }}
          </button>
        </template>
      </UiPageHeader>

      <section class="prompt-search" :aria-label="t('promptLibrary.search.label')">
        <div class="prompt-search__input-wrap">
          <Icon name="search" size="md" class="prompt-search__icon" />
          <input
            v-model="query"
            type="text"
            inputmode="search"
            class="prompt-search__input"
            :placeholder="t('promptLibrary.search.placeholder')"
            @keydown.enter.prevent="searchWithAI"
          />
          <button
            v-if="query"
            type="button"
            class="prompt-search__clear"
            :aria-label="t('promptLibrary.actions.clear')"
            @click="clearSearch"
          >
            <Icon name="x" size="sm" />
          </button>
        </div>

        <div class="prompt-search__actions">
          <button type="button" class="model-trigger" @click="settingsOpen = true">
            <Icon name="sparkles" size="sm" />
            <span class="model-trigger__label">{{ selectedModel || t('promptLibrary.settings.chooseModel') }}</span>
            <Icon name="chevronDown" size="xs" />
          </button>
          <button
            type="button"
            class="ai-search-button"
            :disabled="searching || !query.trim()"
            @click="searchWithAI"
          >
            <Icon v-if="!searching" name="arrowRight" size="sm" />
            <span v-else class="ai-search-button__spinner" aria-hidden="true"></span>
            <span class="ai-search-button__label">
              {{ searching ? t('promptLibrary.search.searching') : t('promptLibrary.search.aiSearch') }}
            </span>
          </button>
        </div>
      </section>

      <PromptCategoryTabs :selected="activeCategory" @select="selectCategory" />

      <div class="prompt-results-bar">
        <p>{{ resultLabel }}</p>
        <button type="button" class="refresh-button" :aria-label="t('promptLibrary.actions.refresh')" @click="loadPage(1)">
          <Icon name="refresh" size="sm" :class="loading && 'animate-spin'" />
        </button>
      </div>

      <div v-if="loading" class="prompt-grid" aria-busy="true">
        <div v-for="index in 6" :key="index" class="prompt-skeleton">
          <span></span><span></span><span></span>
        </div>
      </div>

      <div v-else-if="error" class="prompt-state">
        <Icon name="exclamationCircle" size="lg" />
        <p>{{ error }}</p>
        <button type="button" class="btn btn-secondary" @click="loadPage(1)">
          {{ t('promptLibrary.actions.retry') }}
        </button>
      </div>

      <div v-else-if="prompts.length === 0" class="prompt-state">
        <Icon name="search" size="lg" />
        <p>{{ t('promptLibrary.empty') }}</p>
      </div>

      <div v-else class="prompt-grid">
        <PromptCard
          v-for="item in prompts"
          :key="item.id"
          :item="item"
          :title="displayTitle(item)"
          @open="openPrompt(item)"
          @copy="copyPrompt(item)"
        />
      </div>

      <nav v-if="!aiResults && totalPages > 1" class="prompt-pagination" :aria-label="t('promptLibrary.pagination.label')">
        <button type="button" class="pagination-button" :disabled="page <= 1 || loading" @click="loadPage(page - 1)">
          <Icon name="chevronLeft" size="sm" />
          {{ t('promptLibrary.pagination.previous') }}
        </button>
        <span>{{ page }} / {{ totalPages }}</span>
        <button type="button" class="pagination-button" :disabled="page >= totalPages || loading" @click="loadPage(page + 1)">
          {{ t('promptLibrary.pagination.next') }}
          <Icon name="chevronRight" size="sm" />
        </button>
      </nav>
    </UiPage>

    <PromptSearchSettingsDialog
      :show="settingsOpen"
      :api-keys="apiKeys"
      :selected-key-id="selectedKeyId"
      :selected-model="selectedModel"
      :loading-keys="loadingKeys"
      @close="settingsOpen = false"
      @save="saveSettings"
    />

    <PromptSubmitDialog
      :show="submitDialogOpen"
      :submitting="submittingPrompt"
      @close="submitDialogOpen = false"
      @submit="submitPrompt"
    />

    <PromptDetailDialog
      :item="activePrompt"
      :title="activePrompt ? displayTitle(activePrompt) : ''"
      @close="activePrompt = null"
      @copy="activePrompt && copyPrompt(activePrompt)"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PromptCard from '@/components/user/prompt-library/PromptCard.vue'
import PromptCategoryTabs from '@/components/user/prompt-library/PromptCategoryTabs.vue'
import PromptDetailDialog from '@/components/user/prompt-library/PromptDetailDialog.vue'
import PromptSearchSettingsDialog from '@/components/user/prompt-library/PromptSearchSettingsDialog.vue'
import PromptSubmitDialog from '@/components/user/prompt-library/PromptSubmitDialog.vue'
import {
  getPromptLibraryCategory,
  matchesPromptLibraryCategory,
  type PromptLibraryCategoryKey,
} from '@/components/user/prompt-library/categories'
import { UiPage, UiPageHeader } from '@/ui'
import { keysAPI } from '@/api/keys'
import {
  createPromptSearchPlan,
  listPromptLibrary,
  rankPromptCandidates,
  type PromptLibraryItem,
} from '@/api/promptLibrary'
import { useAppStore } from '@/stores/app'
import {
  createPromptSubmission,
  listApprovedPromptSubmissions,
  type CreatePromptSubmissionRequest,
  type PromptSubmission,
} from '@/api/promptSubmissions'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ApiKey } from '@/types'

const STORAGE_KEY_ID = 'prompt_library_api_key_id'
const STORAGE_MODEL = 'prompt_library_model'
const PAGE_SIZE = 12

const { t, locale } = useI18n()
const appStore = useAppStore()
const query = ref('')
const prompts = ref<PromptLibraryItem[]>([])
const page = ref(1)
const total = ref(0)
const totalPages = ref(1)
const loading = ref(false)
const searching = ref(false)
const error = ref('')
const aiResults = ref(false)
const activePrompt = ref<PromptLibraryItem | null>(null)
const settingsOpen = ref(false)
const submitDialogOpen = ref(false)
const submittingPrompt = ref(false)
const apiKeys = ref<ApiKey[]>([])
const loadingKeys = ref(false)
const selectedKeyId = ref<number | null>(readStoredNumber(STORAGE_KEY_ID))
const selectedModel = ref(localStorage.getItem(STORAGE_MODEL) || '')
const activeCategory = ref<PromptLibraryCategoryKey>('featured')

const selectedKey = computed(() => apiKeys.value.find(key => key.id === selectedKeyId.value) || null)
const activeCategoryConfig = computed(() => getPromptLibraryCategory(activeCategory.value))
const resultLabel = computed(() => {
  if (aiResults.value) return t('promptLibrary.results.ai', { count: prompts.value.length })
  if (activeCategory.value === 'featured') return t('promptLibrary.results.featured')
  return t('promptLibrary.results.category', {
    name: t(`promptLibrary.categories.${activeCategory.value}`),
    count: total.value,
  })
})

onMounted(async () => {
  await Promise.all([loadKeys(), loadPage(1)])
})

watch(locale, () => {
  void loadPage(1)
})

async function loadKeys() {
  loadingKeys.value = true
  try {
    const response = await keysAPI.list(1, 1000, { status: 'active', sort_by: 'id', sort_order: 'desc' })
    apiKeys.value = (response.items || []).filter(key => key.status === 'active')
    if (selectedKeyId.value && !apiKeys.value.some(key => key.id === selectedKeyId.value)) {
      selectedKeyId.value = null
      selectedModel.value = ''
    }
  } catch {
    apiKeys.value = []
  } finally {
    loadingKeys.value = false
  }
}

async function loadPage(nextPage: number) {
  loading.value = true
  error.value = ''
  aiResults.value = false
  try {
    const category = activeCategoryConfig.value
    if (category.key === 'community') {
      const result = await listApprovedPromptSubmissions(nextPage, PAGE_SIZE)
      prompts.value = result.items.map(mapCommunityPrompt)
      page.value = result.page || nextPage
      total.value = result.total || 0
      totalPages.value = Math.max(1, result.pages || 1)
      return
    }
    const result = await listPromptLibrary({
      page: nextPage,
      perPage: PAGE_SIZE,
      q: category.query,
      type: category.type,
      sort: category.sort || 'upvotes',
      locale: locale.value,
    })
    prompts.value = filterPromptsForCategory(result.prompts || [], category.key)
    if (category.key === 'featured') {
      page.value = 1
      total.value = prompts.value.length
      totalPages.value = 1
    } else {
      page.value = result.page || nextPage
      total.value = result.total || 0
      totalPages.value = Math.max(1, result.totalPages || 1)
    }
  } catch (loadError) {
    error.value = loadError instanceof Error ? loadError.message : t('promptLibrary.errors.loadFailed')
  } finally {
    loading.value = false
  }
}

async function submitPrompt(value: CreatePromptSubmissionRequest) {
  if (submittingPrompt.value) return
  submittingPrompt.value = true
  try {
    await createPromptSubmission(value)
    submitDialogOpen.value = false
    appStore.showSuccess(t('promptLibrary.submit.success'))
  } catch (submitError) {
    appStore.showError(extractApiErrorMessage(submitError, t('promptLibrary.submit.failed')))
  } finally {
    submittingPrompt.value = false
  }
}

function mapCommunityPrompt(item: PromptSubmission): PromptLibraryItem {
  return {
    id: `community-${item.id}`,
    title: item.title,
    description: item.description,
    content: item.content,
    type: item.type,
    slug: `community-${item.id}`,
    mediaUrl: item.media_url || null,
    author: { username: item.username },
  }
}

async function searchWithAI() {
  const searchQuery = query.value.trim()
  if (!searchQuery || searching.value) return
  if (!selectedKey.value || !selectedModel.value) {
    settingsOpen.value = true
    return
  }

  searching.value = true
  loading.value = true
  error.value = ''
  try {
    const plan = await createPromptSearchPlan({
      query: searchQuery,
      apiKey: selectedKey.value.key,
      model: selectedModel.value,
    })
    const pages = await Promise.all(plan.keywords.map(keyword =>
      listPromptLibrary({ page: 1, perPage: 24, q: keyword, sort: 'upvotes', locale: locale.value }),
    ))
    const candidates = dedupePrompts(pages.flatMap(result => result.prompts || []))
    if (candidates.length === 0) {
      const fallback = await listPromptLibrary({ page: 1, perPage: 36, sort: 'upvotes', locale: locale.value })
      candidates.push(...fallback.prompts)
    }
    const ranking = await rankPromptCandidates({
      query: searchQuery,
      apiKey: selectedKey.value.key,
      model: selectedModel.value,
      locale: locale.value,
      candidates,
    })
    const byID = new Map(candidates.map(item => [item.id, item]))
    const rankedPrompts: PromptLibraryItem[] = []
    for (const rank of ranking) {
      const item = byID.get(rank.id)
      if (item) rankedPrompts.push({
        ...item,
        aiTitle: rank.title,
        aiDescription: rank.description,
        aiScore: rank.score,
      })
    }
    prompts.value = rankedPrompts
    if (prompts.value.length === 0) prompts.value = candidates.slice(0, PAGE_SIZE)
    total.value = prompts.value.length
    aiResults.value = true
  } catch (searchError) {
    error.value = searchError instanceof Error ? searchError.message : t('promptLibrary.errors.searchFailed')
  } finally {
    searching.value = false
    loading.value = false
  }
}

function clearSearch() {
  query.value = ''
  void loadPage(1)
}

function selectCategory(key: PromptLibraryCategoryKey) {
  if (activeCategory.value === key && !aiResults.value) return
  activeCategory.value = key
  query.value = ''
  void loadPage(1)
}

function saveSettings(value: { apiKeyId: number; model: string }) {
  selectedKeyId.value = value.apiKeyId
  selectedModel.value = value.model
  localStorage.setItem(STORAGE_KEY_ID, String(value.apiKeyId))
  localStorage.setItem(STORAGE_MODEL, value.model)
  settingsOpen.value = false
}

function displayTitle(item: PromptLibraryItem): string {
  return item.aiTitle || item.title
}

function openPrompt(item: PromptLibraryItem) {
  activePrompt.value = item
}

async function copyPrompt(item: PromptLibraryItem) {
  try {
    await navigator.clipboard.writeText(item.content)
    appStore.showSuccess(t('promptLibrary.messages.copied'))
  } catch {
    appStore.showError(t('promptLibrary.messages.copyFailed'))
  }
}

function dedupePrompts(items: PromptLibraryItem[]): PromptLibraryItem[] {
  const seen = new Set<string>()
  return items.filter(item => {
    if (!item.id || seen.has(item.id)) return false
    seen.add(item.id)
    return true
  })
}

function filterPromptsForCategory(
  items: PromptLibraryItem[],
  category: PromptLibraryCategoryKey,
): PromptLibraryItem[] {
  return items.filter(item => matchesPromptLibraryCategory(item, category))
}

function readStoredNumber(key: string): number | null {
  const value = Number(localStorage.getItem(key))
  return Number.isFinite(value) && value > 0 ? value : null
}
</script>

<style scoped>
.prompt-search {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
}

.prompt-submit-button {
  display: inline-flex;
  min-height: 2.5rem;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0 0.875rem;
  border-radius: 0.75rem;
  background: var(--app-text);
  color: var(--app-surface);
  font-size: 0.8125rem;
  font-weight: 650;
  white-space: nowrap;
}

.prompt-search__input-wrap {
  position: relative;
  min-width: 0;
}

.prompt-search__icon {
  position: absolute;
  top: 50%;
  left: 1rem;
  color: var(--app-muted);
  transform: translateY(-50%);
  pointer-events: none;
}

.prompt-search__input {
  width: 100%;
  height: 3rem;
  padding: 0 2.75rem;
  border: 1px solid var(--app-border);
  border-radius: 0.875rem;
  outline: none;
  background: var(--app-surface);
  color: var(--app-text);
  font-size: 0.9375rem;
  transition: border-color 160ms ease, box-shadow 160ms ease;
}

.prompt-search__input:focus {
  border-color: color-mix(in srgb, var(--app-text) 38%, var(--app-border));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--app-text) 8%, transparent);
}

.prompt-search__clear {
  position: absolute;
  top: 50%;
  right: 0.75rem;
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: var(--app-muted);
  transform: translateY(-50%);
}

.prompt-search__actions {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
}

.model-trigger,
.ai-search-button,
.pagination-button,
.refresh-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  transition: background-color 160ms ease, color 160ms ease, opacity 160ms ease;
}

.model-trigger {
  max-width: 15rem;
  height: 3rem;
  padding: 0 0.875rem;
  border: 1px solid var(--app-border);
  border-radius: 0.875rem;
  background: var(--app-surface);
  color: var(--app-text);
  font-size: 0.8125rem;
  font-weight: 600;
}

.model-trigger__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-search-button {
  height: 3rem;
  padding: 0 1rem;
  border-radius: 0.875rem;
  background: var(--app-text);
  color: var(--app-surface);
  font-size: 0.875rem;
  font-weight: 650;
  white-space: nowrap;
}

.ai-search-button:disabled {
  cursor: not-allowed;
  opacity: 0.42;
}

.ai-search-button__spinner {
  width: 0.875rem;
  height: 0.875rem;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: spin 700ms linear infinite;
}

.prompt-results-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  color: var(--app-muted);
  font-size: 0.8125rem;
}

.refresh-button {
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 50%;
  color: var(--app-muted);
}

.refresh-button:hover {
  background: var(--app-surface-muted);
  color: var(--app-text);
}

.prompt-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
}

.prompt-skeleton {
  min-height: 13.5rem;
  padding: 1.125rem;
  border: 1px solid var(--app-border);
  border-radius: 0.875rem;
  background: var(--app-surface);
}

.prompt-skeleton span {
  display: block;
  height: 0.75rem;
  margin-bottom: 0.75rem;
  border-radius: 0.25rem;
  background: var(--app-surface-muted);
  animation: pulse 1.4s ease-in-out infinite;
}

.prompt-skeleton span:nth-child(1) { width: 30%; }
.prompt-skeleton span:nth-child(2) { width: 78%; height: 1rem; }
.prompt-skeleton span:nth-child(3) { width: 100%; }

.prompt-state {
  display: flex;
  min-height: 16rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  color: var(--app-muted);
  text-align: center;
}

.prompt-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  color: var(--app-muted);
  font-size: 0.8125rem;
}

.pagination-button {
  min-height: 2.5rem;
  padding: 0 0.75rem;
  border-radius: 0.625rem;
  color: var(--app-text);
  font-weight: 600;
}

.pagination-button:hover:not(:disabled) {
  background: var(--app-surface-muted);
}

.pagination-button:disabled {
  opacity: 0.35;
}

@media (max-width: 1024px) {
  .prompt-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .prompt-search {
    grid-template-columns: minmax(0, 1fr);
  }

  .prompt-search__actions {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .model-trigger {
    width: 100%;
    max-width: none;
    justify-content: flex-start;
  }

  .model-trigger svg:last-child {
    margin-left: auto;
  }

  .prompt-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 0.625rem;
  }

  .prompt-pagination {
    justify-content: space-between;
    gap: 0.5rem;
  }

  .pagination-button {
    padding: 0 0.5rem;
  }
}

@media (max-width: 360px) {
  .prompt-search__actions {
    grid-template-columns: minmax(0, 1fr) 2.75rem;
  }

  .ai-search-button {
    width: 2.75rem;
    padding: 0;
    gap: 0;
  }

  .ai-search-button__label {
    display: none;
  }

  .ai-search-button svg,
  .ai-search-button__spinner {
    width: 1rem;
    height: 1rem;
  }
}

@keyframes spin { to { transform: rotate(360deg); } }
@keyframes pulse { 50% { opacity: 0.45; } }
</style>
