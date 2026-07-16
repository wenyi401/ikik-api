<template>
  <div class="space-y-3 border-t border-[var(--app-border)] pt-4">
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <label class="input-label mb-0">{{ t('admin.accounts.headerOverride.title') }}</label>
        <p class="mt-1 text-xs text-[var(--app-muted)]">
          {{ t('admin.accounts.headerOverride.hint') }}
        </p>
      </div>
      <button
        type="button"
        :class="[
          'relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors',
          modelEnabled ? 'bg-[var(--app-primary)]' : 'bg-[var(--app-surface-muted)]'
        ]"
        @click="modelEnabled = !modelEnabled"
      >
        <span
          :class="[
            'pointer-events-none inline-block h-5 w-5 translate-y-0.5 rounded-full bg-white shadow transition-transform',
            modelEnabled ? 'translate-x-5' : 'translate-x-0.5'
          ]"
        />
      </button>
    </div>

    <div v-if="modelEnabled" class="space-y-3">
      <div class="rounded-lg bg-[var(--app-primary-soft)] p-3 text-xs text-[var(--app-primary)]">
        {{ bulk ? t('admin.accounts.headerOverride.bulkReplaceHint') : t('admin.accounts.headerOverride.info') }}
      </div>

      <div v-if="modelRows.length > 0" class="space-y-2">
        <div
          v-for="(row, index) in modelRows"
          :key="`${index}-${row.name}`"
          class="grid gap-2 sm:grid-cols-[minmax(0,0.8fr)_minmax(0,1fr)_auto]"
        >
          <input
            v-model="row.name"
            type="text"
            class="input"
            :placeholder="t('admin.accounts.headerOverride.namePlaceholder')"
          />
          <input
            v-model="row.value"
            type="text"
            class="input"
            :placeholder="t('admin.accounts.headerOverride.valuePlaceholder')"
          />
          <button
            type="button"
            class="btn btn-ghost px-2 text-red-500"
            @click="removeRow(index)"
          >
            <Icon name="x" size="sm" />
          </button>
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn btn-secondary text-sm" @click="addRow">
          {{ t('admin.accounts.headerOverride.addRow') }}
        </button>
        <button v-if="platform" type="button" class="btn btn-secondary text-sm" @click="fillTemplate">
          {{ t('admin.accounts.headerOverride.fillTemplate') }}
        </button>
      </div>
      <p class="text-xs text-[var(--app-muted)]">
        {{ t('admin.accounts.headerOverride.emptyValueHint') }}
      </p>
    </div>

    <p v-else-if="bulk" class="text-xs text-[var(--app-muted)]">
      {{ t('admin.accounts.headerOverride.bulkDisableHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import {
  getHeaderOverrideTemplate,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'

const props = withDefaults(defineProps<{
  platform: string
  enabled: boolean
  rows: HeaderOverrideRow[]
  bulk?: boolean
}>(), {
  bulk: false
})

const emit = defineEmits<{
  'update:enabled': [value: boolean]
  'update:rows': [value: HeaderOverrideRow[]]
}>()

const { t } = useI18n()

const modelEnabled = computed({
  get: () => props.enabled,
  set: (value: boolean) => emit('update:enabled', value)
})

const modelRows = computed({
  get: () => props.rows,
  set: (value: HeaderOverrideRow[]) => emit('update:rows', value)
})

const addRow = () => {
  emit('update:rows', [...modelRows.value, { name: '', value: '' }])
}

const removeRow = (index: number) => {
  emit('update:rows', modelRows.value.filter((_, i) => i !== index))
}

const fillTemplate = () => {
  const existing = new Set(modelRows.value.map((row) => row.name.trim().toLowerCase()).filter(Boolean))
  const next = modelRows.value.filter((row) => row.name.trim() || row.value.trim())
  for (const row of getHeaderOverrideTemplate(props.platform)) {
    if (!existing.has(row.name)) next.push(row)
  }
  emit('update:rows', next)
}
</script>
