<template>
  <BaseDialog
    :show="show"
    :title="provider ? t('freeModels.connectDialogTitle', { provider: provider.name }) : t('freeModels.connect')"
    width="wide"
    @close="emit('close')"
  >
    <div v-if="provider" class="space-y-5">
      <div class="rounded-2xl border border-[var(--app-border)] bg-[var(--app-surface-muted)] p-4">
        <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <div class="min-w-0">
            <div class="text-sm font-semibold text-[var(--app-text)]">{{ provider.name }}</div>
            <p class="mt-1 text-sm leading-6 text-[var(--app-text-muted)]">{{ provider.note }}</p>
            <div class="mt-3 rounded-xl bg-[var(--app-surface)] px-3 py-2 font-mono text-xs text-[var(--app-text-muted)]">
              {{ provider.baseUrl }}
            </div>
          </div>
          <div class="flex shrink-0 flex-wrap gap-2">
            <a
              :href="provider.keyUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="btn btn-secondary btn-sm"
            >
              <Icon name="key" size="xs" />
              {{ t('freeModels.openKeyPage') }}
              <Icon name="externalLink" size="xs" />
            </a>
            <a
              :href="provider.docsUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="btn btn-secondary btn-sm"
            >
              <Icon name="externalLink" size="xs" />
              {{ t('freeModels.openDocs') }}
            </a>
          </div>
        </div>
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        <label class="block">
          <span class="input-label">{{ t('freeModels.accountName') }}</span>
          <input
            class="input"
            type="text"
            :value="accountName"
            :placeholder="t('freeModels.accountNamePlaceholder', { provider: provider.name })"
            @input="updateAccountName"
          />
        </label>

        <label class="block">
          <span class="input-label">{{ t('freeModels.baseUrl') }}</span>
          <input
            class="input"
            type="url"
            :value="provider.baseUrlEditable ? baseUrlInput : provider.baseUrl"
            :disabled="!provider.baseUrlEditable"
            @input="updateBaseUrl"
          />
          <span class="input-hint">
            {{ provider.baseUrlEditable ? t('freeModels.editBaseUrlHint') : t('freeModels.fixedBaseUrlHint') }}
          </span>
        </label>

        <label class="block md:col-span-2">
          <span class="input-label">{{ t('freeModels.apiKeys') }}</span>
          <textarea
            class="input"
            autocomplete="off"
            rows="4"
            :value="apiKeysInput"
            :placeholder="t('freeModels.apiKeysPlaceholder')"
            @input="updateApiKeys"
          />
          <span class="input-hint">{{ t('freeModels.apiKeysHint') }}</span>
        </label>

        <label class="block md:col-span-2">
          <span class="input-label">{{ t('freeModels.models') }}</span>
          <textarea
            class="input min-h-[128px] resize-y font-mono text-sm"
            :value="modelsInput"
            :placeholder="t('freeModels.modelsPlaceholder')"
            @input="updateModels"
          />
          <span class="input-hint">{{ t('freeModels.modelsHint') }}</span>
        </label>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="creating || !provider"
        @click="emit('create')"
      >
        <Icon name="plus" size="sm" />
        {{ creating ? t('freeModels.adding') : apiKeyCount > 0 ? t('freeModels.addWithCount', { count: apiKeyCount }) : t('freeModels.add') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { FreeModelProvider } from './types'

defineProps<{
  show: boolean
  provider: FreeModelProvider | null
  accountName: string
  baseUrlInput: string
  apiKeysInput: string
  modelsInput: string
  creating: boolean
  apiKeyCount: number
}>()

const emit = defineEmits<{
  close: []
  create: []
  'update:accountName': [value: string]
  'update:baseUrlInput': [value: string]
  'update:apiKeysInput': [value: string]
  'update:modelsInput': [value: string]
}>()

const { t } = useI18n()

function updateAccountName(event: Event) {
  emit('update:accountName', (event.target as HTMLInputElement).value)
}

function updateBaseUrl(event: Event) {
  emit('update:baseUrlInput', (event.target as HTMLInputElement).value)
}

function updateApiKeys(event: Event) {
  emit('update:apiKeysInput', (event.target as HTMLTextAreaElement).value)
}

function updateModels(event: Event) {
  emit('update:modelsInput', (event.target as HTMLTextAreaElement).value)
}
</script>
