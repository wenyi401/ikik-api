<template>
  <BaseDialog
    :show="show"
    :title="t('onboarding.mode.title')"
    width="wide"
    :show-close-button="false"
    :close-on-escape="false"
    :close-on-click-outside="false"
    panel-class="onboarding-mode-dialog"
  >
    <div class="mode-intro">
      <span class="mode-intro__index">01</span>
      <div>
        <p>{{ t('onboarding.mode.eyebrow') }}</p>
        <strong>{{ t('onboarding.mode.subtitle') }}</strong>
      </div>
    </div>

    <div class="mode-options" role="radiogroup" :aria-label="t('onboarding.mode.title')">
      <button
        type="button"
        class="mode-option mode-option--beginner"
        :disabled="saving"
        @click="emit('select', 'beginner')"
      >
        <span class="mode-option__icon"><Icon name="sparkles" size="lg" /></span>
        <span class="mode-option__copy">
          <strong>{{ t('onboarding.mode.beginner.title') }}</strong>
          <small>{{ t('onboarding.mode.beginner.description') }}</small>
        </span>
        <span class="mode-option__meta">{{ t('onboarding.mode.beginner.meta') }}</span>
        <Icon name="arrowRight" size="md" class="mode-option__arrow" />
      </button>

      <button
        type="button"
        class="mode-option"
        :disabled="saving"
        @click="emit('select', 'expert')"
      >
        <span class="mode-option__icon"><Icon name="terminal" size="lg" /></span>
        <span class="mode-option__copy">
          <strong>{{ t('onboarding.mode.expert.title') }}</strong>
          <small>{{ t('onboarding.mode.expert.description') }}</small>
        </span>
        <span class="mode-option__meta">{{ t('onboarding.mode.expert.meta') }}</span>
        <Icon name="arrowRight" size="md" class="mode-option__arrow" />
      </button>
    </div>

    <p class="mode-footnote">
      <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
      <Icon v-else name="infoCircle" size="sm" />
      {{ saving ? t('common.saving') : t('onboarding.mode.footnote') }}
    </p>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { OnboardingMode } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  show: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  (event: 'select', mode: Exclude<OnboardingMode, 'unset'>): void
}>()

const { t } = useI18n()
</script>

<style scoped>
:global(.onboarding-mode-dialog) {
  --guide-control: #08775c;
  --guide-control-foreground: #ffffff;
  --guide-accent-text: #08775c;
  --guide-accent-soft: #e6f6f1;
  --guide-accent-border: #77b9a6;
  --guide-focus: rgba(8, 119, 92, 0.3);
}

:global(html.dark .onboarding-mode-dialog) {
  --guide-accent-text: #78deb9;
  --guide-accent-soft: rgba(8, 119, 92, 0.24);
  --guide-accent-border: #348f72;
  --guide-focus: rgba(120, 222, 185, 0.34);
}

.mode-intro {
  display: flex;
  align-items: center;
  gap: 0.875rem;
  padding-bottom: 1.25rem;
  border-bottom: 1px solid var(--app-border);
}

.mode-intro__index {
  display: grid;
  width: 2.75rem;
  height: 2.75rem;
  place-items: center;
  border-radius: 6px;
  background: var(--guide-control);
  color: var(--guide-control-foreground);
  font-size: 0.8125rem;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}

.mode-intro p,
.mode-intro strong {
  display: block;
}

.mode-intro p {
  color: var(--guide-accent-text);
  font-size: 0.75rem;
  font-weight: 700;
}

.mode-intro strong {
  margin-top: 0.2rem;
  color: var(--app-text);
  font-size: 1rem;
}

.mode-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
  margin-top: 1.25rem;
}

.mode-option {
  position: relative;
  display: grid;
  min-height: 10.5rem;
  grid-template-columns: auto 1fr auto;
  grid-template-rows: 1fr auto;
  gap: 0.875rem;
  padding: 1.125rem;
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
  color: var(--app-text);
  text-align: left;
  transition: border-color 180ms ease, background-color 180ms ease, transform 180ms ease;
}

.mode-option:hover:not(:disabled) {
  border-color: var(--guide-accent-border);
  background: var(--guide-accent-soft);
  transform: translateY(-2px);
}

.mode-option:active:not(:disabled) {
  transform: translateY(0);
}

.mode-option:focus-visible {
  outline: 3px solid var(--guide-focus);
  outline-offset: 2px;
}

.mode-option:disabled {
  cursor: wait;
  opacity: 0.65;
}

.mode-option--beginner {
  border-color: var(--guide-accent-border);
  background: var(--guide-accent-soft);
}

.mode-option__icon {
  display: grid;
  width: 2.5rem;
  height: 2.5rem;
  place-items: center;
  border-radius: 6px;
  background: var(--app-surface);
  color: var(--guide-accent-text);
  box-shadow: inset 0 0 0 1px var(--app-border);
}

.mode-option__copy {
  min-width: 0;
}

.mode-option__copy strong,
.mode-option__copy small {
  display: block;
}

.mode-option__copy strong {
  font-size: 1rem;
  font-weight: 750;
}

.mode-option__copy small {
  margin-top: 0.4rem;
  color: var(--app-muted);
  font-size: 0.8125rem;
  line-height: 1.6;
}

.mode-option__meta {
  align-self: end;
  color: var(--guide-accent-text);
  font-size: 0.75rem;
  font-weight: 700;
}

.mode-option__arrow {
  align-self: center;
  color: var(--app-muted);
  transition: transform 180ms ease;
}

.mode-option:hover .mode-option__arrow {
  transform: translateX(3px);
}

.mode-footnote {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 1rem;
  color: var(--app-muted);
  font-size: 0.75rem;
}

@media (max-width: 640px) {
  .mode-options {
    grid-template-columns: 1fr;
  }

  .mode-option {
    min-height: 8.5rem;
  }
}
</style>
