<template>
  <article class="provider-card">
    <div class="flex items-start justify-between gap-3">
      <div class="flex min-w-0 items-center gap-3">
        <div class="provider-mark">
          {{ provider.initials }}
        </div>
        <div class="min-w-0 flex-1">
          <h2 class="truncate text-base font-semibold text-[var(--app-text)]">
            {{ provider.name }}
          </h2>
          <p class="mt-0.5 truncate font-mono text-xs text-[var(--app-text-muted)]">
            {{ provider.baseUrl }}
          </p>
        </div>
      </div>
      <div class="provider-statuses">
        <span class="provider-status">
          <span
            class="provider-status-dot"
            :class="accounts.length > 0 ? 'provider-status-dot--normal' : 'provider-status-dot--muted'"
          ></span>
          {{ connectionLabel }}
        </span>
        <span class="provider-status">
          <span class="provider-status-dot" :class="`provider-status-dot--${health}`"></span>
          {{ healthLabel }}
        </span>
      </div>
    </div>

    <p class="provider-note">
      {{ provider.note }}
    </p>

    <div class="mt-4">
      <div class="mb-2 flex items-center justify-between gap-3">
        <span class="text-xs font-medium text-[var(--app-text-muted)]">
          {{ t('freeModels.modelIds') }}
        </span>
        <span v-if="accounts.length > 0" class="shrink-0 text-xs text-[var(--app-text-muted)]">
          {{ t('freeModels.keyCount', { count: accounts.length }) }}
        </span>
      </div>
      <div class="provider-models">
        <code
          v-for="model in provider.models"
          :key="model"
          class="provider-model"
          :title="model"
        >
          {{ model }}
        </code>
      </div>
    </div>

    <div v-if="accounts.length > 0" class="provider-connection-summary">
      <div class="flex items-center gap-2 text-xs text-[var(--app-text-muted)]">
        <Icon name="key" size="xs" />
        <span class="min-w-0 truncate">{{ connectedSummary }}</span>
      </div>
    </div>

    <div class="mt-auto flex flex-wrap items-center justify-end gap-2 pt-5">
      <button
        v-if="accounts.length > 0"
        type="button"
        class="btn btn-secondary btn-sm"
        @click="emit('status')"
      >
        <Icon name="shield" size="xs" />
        {{ t('freeModels.keyStatus') }}
      </button>
      <button type="button" class="btn btn-primary btn-sm" @click="emit('connect')">
        <Icon name="plus" size="xs" />
        {{ t('freeModels.connect') }}
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { FreeModelAccount, FreeModelProvider } from './types'

defineProps<{
  provider: FreeModelProvider
  accounts: FreeModelAccount[]
  connectionLabel: string
  healthLabel: string
  health: 'normal' | 'limited' | 'model_filtered' | 'error' | 'not_connected'
  connectedSummary: string
}>()

const emit = defineEmits<{
  status: []
  connect: []
}>()

const { t } = useI18n()
</script>

<style scoped>
.provider-card {
  display: flex;
  min-width: 0;
  min-height: 18rem;
  flex-direction: column;
  overflow: hidden;
  padding: 1.125rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.provider-mark {
  display: flex;
  width: 2.5rem;
  height: 2.5rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
  color: var(--ui-text);
  font-size: 0.75rem;
  font-weight: 600;
}

.provider-statuses {
  display: flex;
  flex: 0 0 auto;
  align-items: flex-end;
  flex-direction: column;
  gap: 0.3rem;
}

.provider-status {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--ui-text-secondary);
  font-size: 0.6875rem;
  white-space: nowrap;
}

.provider-status-dot {
  width: 0.4rem;
  height: 0.4rem;
  border-radius: 999px;
  background: var(--ui-text-tertiary);
}

.provider-status-dot--normal { background: var(--ui-success); }
.provider-status-dot--limited { background: var(--ui-warning); }
.provider-status-dot--model_filtered { background: #4f7fc2; }
.provider-status-dot--error { background: var(--ui-danger); }
.provider-status-dot--not_connected,
.provider-status-dot--muted { background: var(--ui-text-tertiary); }

.provider-note {
  min-height: 3rem;
  margin-top: 1rem;
  color: var(--ui-text-secondary);
  font-size: 0.8125rem;
  line-height: 1.6;
}

.provider-models {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.provider-model {
  overflow: hidden;
  color: var(--ui-text-secondary);
  font-size: 0.6875rem;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-connection-summary {
  margin-top: 1rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--ui-border);
}
</style>
