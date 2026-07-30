<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <UiPageHeader :title="t('admin.promptSubmissions.title')">
        <template #actions>
          <button
            type="button"
            class="translation-settings-button"
            @click="translationSettingsOpen = true"
          >
            <Icon name="cog" size="sm" />
            {{ t('admin.promptSubmissions.translation.action') }}
          </button>
        </template>
      </UiPageHeader>

      <div class="submission-toolbar">
        <div class="submission-tabs" role="tablist" :aria-label="t('admin.promptSubmissions.statusFilter')">
          <button
            v-for="status in statuses"
            :key="status"
            type="button"
            role="tab"
            class="submission-tab"
            :class="{ 'submission-tab--active': activeStatus === status }"
            :aria-selected="activeStatus === status"
            @click="selectStatus(status)"
          >
            {{ t(`admin.promptSubmissions.status.${status}`) }}
          </button>
        </div>

        <div class="submission-search">
          <Icon name="search" size="sm" />
          <input
            v-model="search"
            type="search"
            :placeholder="t('admin.promptSubmissions.search')"
            @keydown.enter.prevent="load(1)"
          />
          <button type="button" :aria-label="t('common.refresh')" @click="load(page)">
            <Icon name="refresh" size="sm" :class="loading && 'animate-spin'" />
          </button>
        </div>
      </div>

      <div v-if="loading" class="submission-list" aria-busy="true">
        <div v-for="index in 4" :key="index" class="submission-card submission-card--loading"></div>
      </div>

      <div v-else-if="items.length === 0" class="submission-empty">
        <Icon name="inbox" size="lg" />
        <span>{{ t('admin.promptSubmissions.empty') }}</span>
      </div>

      <div v-else class="submission-list">
        <article v-for="item in items" :key="item.id" class="submission-card">
          <div class="submission-card__main">
            <div class="submission-card__heading">
              <h2>{{ item.title }}</h2>
              <span class="submission-status" :class="`submission-status--${item.status}`">
                {{ t(`admin.promptSubmissions.status.${item.status || 'pending'}`) }}
              </span>
            </div>
            <div class="submission-card__meta">
              <span>{{ item.user_email || item.username }}</span>
              <span>{{ t(`promptLibrary.types.${item.type.toLowerCase()}`) }}</span>
              <span>{{ t(`promptLibrary.submit.categories.${item.category}`) }}</span>
              <span>{{ formatDateTimeToMinute(item.created_at) }}</span>
            </div>
            <p>{{ item.description || item.content }}</p>
          </div>
          <button type="button" class="submission-card__review" @click="reviewing = item">
            <Icon name="eye" size="sm" />
            {{ t('admin.promptSubmissions.review') }}
          </button>
        </article>
      </div>

      <Pagination
        v-if="total > 0"
        :page="page"
        :total="total"
        :page-size="pageSize"
        @update:page="load"
        @update:pageSize="changePageSize"
      />
    </UiPage>

    <PromptSubmissionReviewDialog
      :item="reviewing"
      :saving="saving"
      @close="reviewing = null"
      @review="review"
    />

    <PromptTranslationSettingsDialog
      :show="translationSettingsOpen"
      @close="translationSettingsOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import PromptSubmissionReviewDialog from '@/components/admin/prompt-library/PromptSubmissionReviewDialog.vue'
import PromptTranslationSettingsDialog from '@/components/admin/prompt-library/PromptTranslationSettingsDialog.vue'
import { UiPage, UiPageHeader } from '@/ui'
import {
  listPromptSubmissionsAdmin,
  reviewPromptSubmission,
  type PromptSubmission,
  type PromptSubmissionStatus,
} from '@/api/promptSubmissions'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import { useAppStore } from '@/stores/app'

type StatusFilter = PromptSubmissionStatus | 'all'
const statuses: StatusFilter[] = ['pending', 'approved', 'rejected', 'all']
const { t } = useI18n()
const appStore = useAppStore()
const items = ref<PromptSubmission[]>([])
const activeStatus = ref<StatusFilter>('pending')
const search = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const loading = ref(false)
const saving = ref(false)
const reviewing = ref<PromptSubmission | null>(null)
const translationSettingsOpen = ref(false)

onMounted(() => load(1))

async function load(nextPage = 1) {
  loading.value = true
  try {
    const result = await listPromptSubmissionsAdmin(nextPage, pageSize.value, {
      status: activeStatus.value,
      search: search.value,
    })
    items.value = result.items || []
    page.value = result.page || nextPage
    total.value = result.total || 0
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.promptSubmissions.loadFailed')))
  } finally {
    loading.value = false
  }
}

function selectStatus(status: StatusFilter) {
  if (activeStatus.value === status) return
  activeStatus.value = status
  void load(1)
}

function changePageSize(size: number) {
  pageSize.value = size
  void load(1)
}

async function review(value: { status: Exclude<PromptSubmissionStatus, 'pending'>; note: string }) {
  if (!reviewing.value || saving.value) return
  saving.value = true
  try {
    await reviewPromptSubmission(reviewing.value.id, value.status, value.note)
    reviewing.value = null
    appStore.showSuccess(t(`admin.promptSubmissions.messages.${value.status}`))
    await load(page.value)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.promptSubmissions.reviewFailed')))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.translation-settings-button {
  display: inline-flex;
  min-height: 2.5rem;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0 0.75rem;
  border: 1px solid var(--app-border);
  border-radius: 0.75rem;
  background: var(--app-surface);
  color: var(--app-text);
  font-size: 0.8125rem;
  font-weight: 600;
}

.submission-toolbar {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.submission-tabs {
  display: flex;
  min-width: 0;
  gap: 0.25rem;
  overflow-x: auto;
  scrollbar-width: none;
}

.submission-tabs::-webkit-scrollbar {
  display: none;
}

.submission-tab {
  min-height: 2.25rem;
  flex: 0 0 auto;
  padding: 0 0.75rem;
  border-radius: 0.625rem;
  color: var(--app-muted);
  font-size: 0.8125rem;
  font-weight: 600;
}

.submission-tab--active {
  background: var(--app-text);
  color: var(--app-surface);
}

.submission-search {
  display: grid;
  width: min(22rem, 100%);
  min-width: 0;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.5rem;
  padding: 0 0.5rem 0 0.75rem;
  border: 1px solid var(--app-border);
  border-radius: 0.75rem;
  color: var(--app-muted);
}

.submission-search input {
  min-width: 0;
  height: 2.5rem;
  outline: none;
  background: transparent;
  color: var(--app-text);
  font-size: 0.8125rem;
}

.submission-search button {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
}

.submission-list {
  display: grid;
  gap: 0.625rem;
}

.submission-card {
  display: grid;
  min-width: 0;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  border: 1px solid var(--app-border);
  border-radius: 0.75rem;
  background: var(--app-surface);
}

.submission-card--loading {
  min-height: 8rem;
  background: var(--app-surface-muted);
  animation: pulse 1.4s ease-in-out infinite;
}

.submission-card__main {
  min-width: 0;
}

.submission-card__heading {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}

.submission-card__heading h2 {
  min-width: 0;
  overflow: hidden;
  color: var(--app-text);
  font-size: 0.9375rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.submission-status {
  flex: 0 0 auto;
  padding: 0.25rem 0.5rem;
  border-radius: 999px;
  background: var(--app-surface-muted);
  color: var(--app-muted);
  font-size: 0.6875rem;
  font-weight: 650;
}

.submission-status--approved { color: var(--color-success, #078c6c); }
.submission-status--rejected { color: var(--color-danger, #dc2626); }
.submission-status--pending { color: var(--color-warning, #b56a00); }

.submission-card__meta {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  gap: 0.375rem 0.875rem;
  margin-top: 0.5rem;
  color: var(--app-muted);
  font-size: 0.75rem;
}

.submission-card__main > p {
  display: -webkit-box;
  margin-top: 0.625rem;
  overflow: hidden;
  color: var(--app-muted);
  font-size: 0.8125rem;
  line-height: 1.55;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.submission-card__review {
  display: inline-flex;
  min-height: 2.5rem;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  padding: 0 0.75rem;
  border-radius: 0.625rem;
  background: var(--app-surface-muted);
  color: var(--app-text);
  font-size: 0.75rem;
  font-weight: 650;
}

.submission-empty {
  display: flex;
  min-height: 18rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  color: var(--app-muted);
}

@media (max-width: 640px) {
  .submission-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .submission-tabs {
    margin-right: -1rem;
    padding-right: 1rem;
  }

  .submission-search {
    width: 100%;
  }

  .submission-card {
    grid-template-columns: minmax(0, 1fr);
    gap: 0.75rem;
  }

  .submission-card__review {
    width: 100%;
  }
}

@keyframes pulse { 50% { opacity: 0.5; } }
</style>
