<template>
  <BaseDialog
    :show="show"
    :title="t('admin.promptSubmissions.translation.title')"
    width="normal"
    @close="emit('close')"
  >
    <div class="translation-form" :aria-busy="loading">
      <div class="translation-switch">
        <label for="prompt-translation-enabled">
          {{ t('admin.promptSubmissions.translation.enabled') }}
        </label>
        <Toggle
          id="prompt-translation-enabled"
          v-model="form.enabled"
          :disabled="loading || saving"
        />
      </div>

      <label class="translation-field">
        <span>{{ t('admin.promptSubmissions.translation.group') }}</span>
        <select
          v-model.number="form.group_id"
          :disabled="loading || saving"
          @change="handleGroupChange"
        >
          <option :value="0">{{ t('admin.promptSubmissions.translation.selectGroup') }}</option>
          <option v-for="group in groups" :key="group.id" :value="group.id">
            {{ group.name }} · {{ group.platform }}
          </option>
        </select>
      </label>

      <label class="translation-field">
        <span>{{ t('admin.promptSubmissions.translation.model') }}</span>
        <select v-model="form.model" :disabled="loading || saving || modelsLoading || !form.group_id">
          <option value="">{{ modelPlaceholder }}</option>
          <option v-for="model in models" :key="model" :value="model">{{ model }}</option>
        </select>
      </label>

      <div class="translation-count">
        {{ t('admin.promptSubmissions.translation.translatedCount', { count: translatedCount }) }}
      </div>
    </div>

    <template #footer>
      <button type="button" class="translation-button translation-button--quiet" @click="emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button
        type="button"
        class="translation-button translation-button--primary"
        :disabled="loading || saving || (form.enabled && (!form.group_id || !form.model))"
        @click="save"
      >
        {{ saving ? t('common.saving') : t('common.save') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { groupsAPI } from '@/api/admin/groups'
import {
  getPromptLibraryTranslationConfig,
  updatePromptLibraryTranslationConfig,
} from '@/api/promptSubmissions'
import type { AdminGroup } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const modelsLoading = ref(false)
const groups = ref<AdminGroup[]>([])
const models = ref<string[]>([])
const translatedCount = ref(0)
const form = reactive({ enabled: false, group_id: 0, model: '' })

const modelPlaceholder = computed(() => modelsLoading.value
  ? t('admin.promptSubmissions.translation.loadingModels')
  : t('admin.promptSubmissions.translation.selectModel'))

watch(() => props.show, (show) => {
  if (show) void load()
}, { immediate: true })

async function load() {
  loading.value = true
  try {
    const [config, availableGroups] = await Promise.all([
      getPromptLibraryTranslationConfig(),
      groupsAPI.getAll(undefined, 'public'),
    ])
    groups.value = availableGroups.filter(group =>
      group.status === 'active' && ['openai', 'grok', 'kiro'].includes(group.platform),
    )
    form.enabled = config.enabled
    form.group_id = config.group_id || 0
    form.model = config.model || ''
    translatedCount.value = config.translated_count || 0
    if (form.group_id) await loadModels(true)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.promptSubmissions.translation.loadFailed')))
  } finally {
    loading.value = false
  }
}

function handleGroupChange() {
  form.model = ''
  void loadModels(false)
}

async function loadModels(keepCurrent: boolean) {
  if (!form.group_id) {
    models.value = []
    if (!keepCurrent) form.model = ''
    return
  }
  const group = groups.value.find(item => item.id === form.group_id)
  if (!group) return
  modelsLoading.value = true
  const current = form.model
  try {
    const candidates = await groupsAPI.getModelsListCandidates(group.id, group.platform)
    models.value = Array.from(new Set([
      ...(keepCurrent && current ? [current] : []),
      ...candidates.map(model => model.trim()).filter(Boolean),
    ]))
    if (keepCurrent) form.model = current
  } catch (error) {
    models.value = current ? [current] : []
    appStore.showError(extractApiErrorMessage(error, t('admin.promptSubmissions.translation.modelsFailed')))
  } finally {
    modelsLoading.value = false
  }
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const config = await updatePromptLibraryTranslationConfig({ ...form })
    translatedCount.value = config.translated_count || 0
    appStore.showSuccess(t('admin.promptSubmissions.translation.saved'))
    emit('saved')
    emit('close')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.promptSubmissions.translation.saveFailed')))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.translation-form {
  display: grid;
  gap: 1rem;
}

.translation-switch {
  display: flex;
  min-height: 2.75rem;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  color: var(--app-text);
  font-size: 0.875rem;
  font-weight: 600;
}

.translation-field {
  display: grid;
  gap: 0.5rem;
  color: var(--app-muted);
  font-size: 0.75rem;
  font-weight: 600;
}

.translation-field select {
  width: 100%;
  min-width: 0;
  height: 2.75rem;
  padding: 0 2.5rem 0 0.75rem;
  border: 1px solid var(--app-border);
  border-radius: 0.75rem;
  outline: none;
  background: var(--app-surface);
  color: var(--app-text);
  font-size: 0.8125rem;
}

.translation-field select:focus {
  border-color: color-mix(in srgb, var(--app-text) 35%, var(--app-border));
}

.translation-count {
  color: var(--app-muted);
  font-size: 0.75rem;
}

.translation-button {
  display: inline-flex;
  min-height: 2.5rem;
  align-items: center;
  justify-content: center;
  padding: 0 0.875rem;
  border-radius: 0.625rem;
  font-size: 0.8125rem;
  font-weight: 650;
}

.translation-button--quiet {
  color: var(--app-text);
}

.translation-button--primary {
  background: var(--app-text);
  color: var(--app-surface);
}

.translation-button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}
</style>
