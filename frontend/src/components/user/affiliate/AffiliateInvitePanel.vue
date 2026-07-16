<template>
  <section class="affiliate-panel">
    <header class="affiliate-panel-header">
      <h2>{{ t('affiliate.title') }}</h2>
    </header>

    <div class="affiliate-invite-grid">
      <div class="affiliate-invite-field">
        <span class="affiliate-field-label">{{ t('affiliate.yourCode') }}</span>
        <div class="affiliate-field-value">
          <code>{{ code }}</code>
          <button class="affiliate-copy-button" :title="t('affiliate.copyCode')" @click="emit('copy-code')">
            <Icon name="copy" size="sm" />
            <span class="sr-only">{{ t('affiliate.copyCode') }}</span>
          </button>
        </div>
      </div>

      <div class="affiliate-invite-field">
        <span class="affiliate-field-label">{{ t('affiliate.inviteLink') }}</span>
        <div class="affiliate-field-value">
          <code>{{ inviteLink }}</code>
          <button class="affiliate-copy-button" :title="t('affiliate.copyLink')" @click="emit('copy-link')">
            <Icon name="copy" size="sm" />
            <span class="sr-only">{{ t('affiliate.copyLink') }}</span>
          </button>
        </div>
      </div>
    </div>

    <div class="affiliate-rules">
      <span>{{ t('affiliate.tips.line1') }}</span>
      <span>{{ t('affiliate.tips.line2', { rate: `${rebateRate}%` }) }}</span>
      <span>{{ t('affiliate.tips.line3') }}</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  code: string
  inviteLink: string
  rebateRate: string
}>()

const emit = defineEmits<{
  'copy-code': []
  'copy-link': []
}>()

const { t } = useI18n()
</script>

<style scoped>
.affiliate-panel {
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.affiliate-panel-header {
  padding: 0.875rem 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.affiliate-panel-header h2 {
  color: var(--ui-text);
  font-size: 0.9375rem;
  font-weight: 600;
}

.affiliate-invite-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1.25rem;
  padding: 1.25rem;
}

.affiliate-invite-field {
  min-width: 0;
}

.affiliate-field-label {
  display: block;
  margin-bottom: 0.45rem;
  color: var(--ui-text-secondary);
  font-size: 0.75rem;
  font-weight: 500;
}

.affiliate-field-value {
  display: flex;
  min-width: 0;
  height: 2.75rem;
  align-items: center;
  gap: 0.5rem;
  padding: 0 0.4rem 0 0.75rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
}

.affiliate-field-value code {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  color: var(--ui-text);
  font-size: 0.8125rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.affiliate-copy-button {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border-radius: var(--ui-radius-md);
  color: var(--ui-text-secondary);
}

.affiliate-copy-button:hover {
  background: var(--ui-surface-subtle);
  color: var(--ui-text);
}

.affiliate-rules {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
  padding: 0.875rem 1.25rem;
  border-top: 1px solid var(--ui-border);
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  line-height: 1.45;
}

@media (max-width: 900px) {
  .affiliate-invite-grid,
  .affiliate-rules {
    grid-template-columns: 1fr;
  }

  .affiliate-rules {
    gap: 0.35rem;
  }
}

@media (max-width: 640px) {
  .affiliate-invite-grid {
    gap: 1rem;
    padding: 1rem;
  }

  .affiliate-rules {
    padding: 0.75rem 1rem;
  }
}
</style>
