<template>
  <BaseDialog
    :show="Boolean(item)"
    :title="item?.title || t('admin.promptSubmissions.reviewTitle')"
    width="wide"
    panel-class="prompt-review-dialog"
    @close="emit('close')"
  >
    <div v-if="item" class="prompt-review">
      <div class="prompt-review__meta">
        <span>{{ item.user_email || item.username }}</span>
        <span>{{ typeLabel }}</span>
        <span>{{ categoryLabel }}</span>
        <span>{{ formatDateTimeToMinute(item.created_at) }}</span>
      </div>

      <p v-if="item.description" class="prompt-review__description">{{ item.description }}</p>

      <PromptMediaPreview
        v-if="item.media_url"
        :item="previewItem"
        :title="item.title"
        variant="detail"
      />

      <pre class="prompt-review__content">{{ item.content }}</pre>

      <div class="prompt-review__note">
        <label for="prompt-review-note">{{ t('admin.promptSubmissions.reviewNote') }}</label>
        <textarea id="prompt-review-note" v-model="note" class="input" rows="3" maxlength="500"></textarea>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button type="button" class="btn prompt-review__reject" :disabled="saving" @click="decide('rejected')">
        {{ t('admin.promptSubmissions.reject') }}
      </button>
      <button type="button" class="btn btn-primary" :disabled="saving" @click="decide('approved')">
        {{ saving ? t('common.saving') : t('admin.promptSubmissions.approve') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PromptMediaPreview from '@/components/user/prompt-library/PromptMediaPreview.vue'
import type { PromptLibraryItem } from '@/api/promptLibrary'
import type { PromptSubmission, PromptSubmissionStatus } from '@/api/promptSubmissions'
import { formatDateTimeToMinute } from '@/utils/format'

const props = defineProps<{ item: PromptSubmission | null; saving?: boolean }>()
const emit = defineEmits<{
  close: []
  review: [value: { status: Exclude<PromptSubmissionStatus, 'pending'>; note: string }]
}>()
const { t } = useI18n()
const note = ref('')

const previewItem = computed<PromptLibraryItem>(() => ({
  id: `review-${props.item?.id || 0}`,
  title: props.item?.title || '',
  description: props.item?.description || '',
  content: props.item?.content || '',
  type: props.item?.type || 'TEXT',
  slug: `review-${props.item?.id || 0}`,
  mediaUrl: props.item?.media_url || null,
}))
const typeLabel = computed(() => t(`promptLibrary.types.${String(props.item?.type || 'TEXT').toLowerCase()}`))
const categoryLabel = computed(() => t(`promptLibrary.submit.categories.${props.item?.category || 'other'}`))

watch(() => props.item, (item) => {
  note.value = item?.review_note || ''
})

function decide(status: Exclude<PromptSubmissionStatus, 'pending'>) {
  if (!props.item || props.saving) return
  emit('review', { status, note: note.value.trim() })
}
</script>

<style scoped>
.prompt-review {
  display: grid;
  gap: 1rem;
}

.prompt-review__meta {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
  color: var(--app-muted);
  font-size: 0.75rem;
}

.prompt-review__description {
  color: var(--app-muted);
  font-size: 0.875rem;
  line-height: 1.6;
}

.prompt-review__content {
  max-height: min(28rem, 46dvh);
  overflow: auto;
  padding: 1rem;
  border: 1px solid var(--app-border);
  border-radius: 0.75rem;
  background: var(--app-surface-muted);
  color: var(--app-text);
  font-family: inherit;
  font-size: 0.8125rem;
  line-height: 1.65;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.prompt-review__note label {
  display: block;
  margin-bottom: 0.5rem;
  color: var(--app-text);
  font-size: 0.8125rem;
  font-weight: 600;
}

.prompt-review__note textarea {
  height: auto;
  resize: vertical;
}

.prompt-review__reject {
  color: var(--color-danger, #dc2626);
}

@media (max-width: 640px) {
  :global(.modal-overlay:has(.prompt-review-dialog)) {
    align-items: flex-end;
    padding: 0;
  }

  :global(.prompt-review-dialog) {
    max-width: none;
    max-height: 92dvh;
    border-right: 0;
    border-bottom: 0;
    border-left: 0;
    border-radius: 1.25rem 1.25rem 0 0;
  }
}
</style>
