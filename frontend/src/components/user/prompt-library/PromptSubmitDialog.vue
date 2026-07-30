<template>
  <BaseDialog
    :show="show"
    :title="t('promptLibrary.submit.title')"
    width="wide"
    panel-class="prompt-submit-dialog"
    @close="emit('close')"
  >
    <form id="prompt-submit-form" class="prompt-submit-form" @submit.prevent="submit">
      <div class="field-group field-group--wide">
        <label class="field-label" for="prompt-submit-title">{{ t('promptLibrary.submit.fields.title') }}</label>
        <input
          id="prompt-submit-title"
          v-model="form.title"
          class="input"
          type="text"
          maxlength="200"
          required
        />
      </div>

      <div class="field-group">
        <label class="field-label">{{ t('promptLibrary.submit.fields.type') }}</label>
        <Select v-model="form.type" :options="typeOptions" />
      </div>

      <div class="field-group">
        <label class="field-label">{{ t('promptLibrary.submit.fields.category') }}</label>
        <Select v-model="form.category" :options="categoryOptions" />
      </div>

      <div class="field-group field-group--wide">
        <label class="field-label" for="prompt-submit-description">{{ t('promptLibrary.submit.fields.description') }}</label>
        <textarea
          id="prompt-submit-description"
          v-model="form.description"
          class="input prompt-submit-form__description"
          maxlength="500"
          rows="2"
        ></textarea>
      </div>

      <div v-if="showMediaURL" class="field-group field-group--wide">
        <label class="field-label" for="prompt-submit-media">{{ t('promptLibrary.submit.fields.mediaUrl') }}</label>
        <input
          id="prompt-submit-media"
          v-model="form.media_url"
          class="input"
          type="url"
          inputmode="url"
          maxlength="2000"
          placeholder="https://"
        />
      </div>

      <div class="field-group field-group--wide">
        <label class="field-label" for="prompt-submit-content">{{ t('promptLibrary.submit.fields.content') }}</label>
        <textarea
          id="prompt-submit-content"
          v-model="form.content"
          class="input prompt-submit-form__content"
          minlength="10"
          maxlength="20000"
          rows="10"
          required
        ></textarea>
        <span class="field-count">{{ form.content.length.toLocaleString() }} / 20,000</span>
      </div>
    </form>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">
        {{ t('promptLibrary.actions.cancel') }}
      </button>
      <button type="submit" form="prompt-submit-form" class="btn btn-primary" :disabled="submitting || !canSubmit">
        {{ submitting ? t('promptLibrary.submit.submitting') : t('promptLibrary.submit.submit') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import type {
  CreatePromptSubmissionRequest,
  PromptSubmissionCategory,
  PromptSubmissionType,
} from '@/api/promptSubmissions'

const props = defineProps<{ show: boolean; submitting?: boolean }>()
const emit = defineEmits<{
  close: []
  submit: [value: CreatePromptSubmissionRequest]
}>()

const { t } = useI18n()
const form = reactive<CreatePromptSubmissionRequest>({
  title: '',
  description: '',
  content: '',
  type: 'TEXT',
  category: 'other',
  media_url: '',
})

const promptTypes: PromptSubmissionType[] = ['TEXT', 'STRUCTURED', 'IMAGE', 'VIDEO', 'AUDIO']
const promptCategories: PromptSubmissionCategory[] = [
  'coding',
  'writing',
  'business',
  'creative',
  'education',
  'workflow',
  'productivity',
  'other',
]
const typeOptions = computed<SelectOption[]>(() => promptTypes.map(value => ({
  value,
  label: t(`promptLibrary.types.${value.toLowerCase()}`),
})))
const categoryOptions = computed<SelectOption[]>(() => promptCategories.map(value => ({
  value,
  label: t(`promptLibrary.submit.categories.${value}`),
})))
const showMediaURL = computed(() => form.type === 'IMAGE' || form.type === 'VIDEO')
const canSubmit = computed(() => form.title.trim().length > 0 && form.content.trim().length >= 10)

watch(() => props.show, (show) => {
  if (!show) return
  Object.assign(form, {
    title: '',
    description: '',
    content: '',
    type: 'TEXT',
    category: 'other',
    media_url: '',
  })
})

function submit() {
  if (!canSubmit.value || props.submitting) return
  emit('submit', {
    title: form.title.trim(),
    description: form.description?.trim(),
    content: form.content.trim(),
    type: form.type,
    category: form.category,
    media_url: showMediaURL.value ? form.media_url?.trim() : '',
  })
}
</script>

<style scoped>
.prompt-submit-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}

.field-group {
  min-width: 0;
}

.field-group--wide {
  grid-column: 1 / -1;
}

.field-label {
  display: block;
  margin-bottom: 0.5rem;
  color: var(--app-text);
  font-size: 0.8125rem;
  font-weight: 600;
}

.field-count {
  display: block;
  margin-top: 0.375rem;
  color: var(--app-muted);
  font-size: 0.75rem;
  text-align: right;
}

.prompt-submit-form__description,
.prompt-submit-form__content {
  height: auto;
  resize: vertical;
  line-height: 1.6;
}

@media (max-width: 640px) {
  .prompt-submit-form {
    grid-template-columns: minmax(0, 1fr);
  }

  .field-group--wide {
    grid-column: auto;
  }

  :global(.modal-overlay:has(.prompt-submit-dialog)) {
    align-items: flex-end;
    padding: 0;
  }

  :global(.prompt-submit-dialog) {
    max-width: none;
    max-height: 92dvh;
    border-right: 0;
    border-bottom: 0;
    border-left: 0;
    border-radius: 1.25rem 1.25rem 0 0;
  }
}
</style>
