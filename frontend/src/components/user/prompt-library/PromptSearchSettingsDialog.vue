<template>
  <BaseDialog
    :show="show"
    :title="t('promptLibrary.settings.title')"
    width="normal"
    panel-class="prompt-search-settings-dialog"
    @close="emit('close')"
  >
    <div class="settings-stack">
      <div class="field-group">
        <label class="field-label">{{ t('promptLibrary.settings.apiKey') }}</label>
        <Select
          v-model="draftKeyId"
          :options="keyOptions"
          :placeholder="t('promptLibrary.settings.selectApiKey')"
          :disabled="loadingKeys"
          searchable
          :search-placeholder="t('promptLibrary.settings.searchApiKey')"
        />
      </div>

      <div class="field-group">
        <label class="field-label">{{ t('promptLibrary.settings.model') }}</label>
        <Select
          v-model="draftModel"
          :options="modelOptions"
          :placeholder="modelPlaceholder"
          :disabled="!selectedKey || loadingModels"
          searchable
          :search-placeholder="t('promptLibrary.settings.searchModel')"
          :empty-text="t('promptLibrary.settings.noModels')"
        />
        <p v-if="modelError" class="field-error">{{ modelError }}</p>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">
        {{ t('promptLibrary.actions.cancel') }}
      </button>
      <button type="button" class="btn btn-primary" :disabled="!canSave" @click="save">
        {{ t('promptLibrary.actions.save') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { listGatewayModels } from '@/api/promptLibrary'
import type { ApiKey } from '@/types'

const props = defineProps<{
  show: boolean
  apiKeys: ApiKey[]
  selectedKeyId: number | null
  selectedModel: string
  loadingKeys?: boolean
}>()

const emit = defineEmits<{
  close: []
  save: [value: { apiKeyId: number; model: string }]
}>()

const { t } = useI18n()
const draftKeyId = ref<number | null>(null)
const draftModel = ref('')
const models = ref<string[]>([])
const loadingModels = ref(false)
const modelError = ref('')
let modelController: AbortController | null = null

const selectedKey = computed(() => props.apiKeys.find(key => key.id === Number(draftKeyId.value)) || null)
const keyOptions = computed<SelectOption[]>(() => props.apiKeys.map(key => ({
  value: key.id,
  label: key.group?.name ? `${key.name} · ${key.group.name}` : key.name,
  disabled: key.status !== 'active',
})))
const modelOptions = computed<SelectOption[]>(() => models.value.map(model => ({ value: model, label: model })))
const modelPlaceholder = computed(() => {
  if (!selectedKey.value) return t('promptLibrary.settings.selectApiKeyFirst')
  if (loadingModels.value) return t('promptLibrary.settings.loadingModels')
  return t('promptLibrary.settings.selectModel')
})
const canSave = computed(() => Boolean(selectedKey.value && draftModel.value && !loadingModels.value))

watch(() => props.show, (show) => {
  if (!show) return
  draftKeyId.value = props.selectedKeyId
  draftModel.value = props.selectedModel
  if (!draftKeyId.value) {
    draftKeyId.value = props.apiKeys.find(key => key.status === 'active')?.id || null
  }
}, { immediate: true })

watch(() => props.apiKeys, (keys) => {
  if (props.show && !draftKeyId.value) {
    draftKeyId.value = keys.find(key => key.status === 'active')?.id || null
  }
})

watch(draftKeyId, async () => {
  modelController?.abort()
  modelError.value = ''
  models.value = []
  const key = selectedKey.value
  if (!key) {
    draftModel.value = ''
    return
  }

  const keepModel = draftModel.value
  const controller = new AbortController()
  modelController = controller
  loadingModels.value = true
  try {
    models.value = await listGatewayModels(key.key, controller.signal)
    if (!models.value.includes(keepModel)) {
      draftModel.value = models.value[0] || ''
    }
  } catch (error) {
    if (controller.signal.aborted) return
    modelError.value = error instanceof Error ? error.message : t('promptLibrary.settings.loadModelsFailed')
    draftModel.value = ''
  } finally {
    if (modelController === controller) {
      loadingModels.value = false
      modelController = null
    }
  }
}, { immediate: true })

function save() {
  if (!canSave.value || !draftKeyId.value) return
  emit('save', { apiKeyId: draftKeyId.value, model: draftModel.value })
}
</script>

<style scoped>
.settings-stack {
  display: grid;
  gap: 1.25rem;
}

.field-group {
  min-width: 0;
}

.field-label {
  display: block;
  margin-bottom: 0.5rem;
  color: var(--app-text);
  font-size: 0.875rem;
  font-weight: 600;
}

.field-error {
  margin-top: 0.5rem;
  color: var(--color-danger, #dc2626);
  font-size: 0.8125rem;
  line-height: 1.45;
}

@media (max-width: 640px) {
  :global(.modal-overlay:has(.prompt-search-settings-dialog)) {
    align-items: flex-end;
    padding: 0;
  }

  :global(.prompt-search-settings-dialog) {
    max-width: none;
    max-height: min(82dvh, 42rem);
    border-right: 0;
    border-bottom: 0;
    border-left: 0;
    border-radius: 1.25rem 1.25rem 0 0;
  }
}
</style>
