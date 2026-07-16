<template>
  <span class="ui-icon-button-wrap">
    <button
      v-bind="$attrs"
      type="button"
      class="ui-icon-button"
      :class="[`ui-icon-button--${size}`, `ui-icon-button--${tone}`]"
      :aria-label="label"
    >
      <slot />
    </button>
    <span class="ui-icon-button-tooltip" role="tooltip">{{ label }}</span>
  </span>
</template>

<script setup lang="ts">
defineOptions({ inheritAttrs: false })

withDefaults(defineProps<{
  label: string
  size?: 'sm' | 'md'
  tone?: 'default' | 'danger'
}>(), {
  size: 'md',
  tone: 'default'
})
</script>

<style scoped>
.ui-icon-button-wrap {
  position: relative;
  display: inline-flex;
}

.ui-icon-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: var(--ui-radius-md);
  background: transparent;
  color: var(--ui-text-secondary);
  transition: background-color 150ms ease, color 150ms ease;
}

.ui-icon-button--sm {
  width: 2rem;
  height: 2rem;
}

.ui-icon-button--md {
  width: 2.5rem;
  height: 2.5rem;
}

.ui-icon-button:hover,
.ui-icon-button:focus-visible {
  background: var(--ui-surface-hover);
  color: var(--ui-text);
}

.ui-icon-button:focus-visible {
  outline: 2px solid var(--ui-focus);
  outline-offset: 2px;
}

.ui-icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.ui-icon-button--danger {
  color: var(--ui-danger);
}

.ui-icon-button-tooltip {
  position: absolute;
  z-index: 100;
  top: calc(100% + 0.375rem);
  left: 50%;
  width: max-content;
  max-width: 12rem;
  transform: translateX(-50%) translateY(-2px);
  border-radius: var(--ui-radius-sm);
  background: var(--ui-text);
  color: var(--ui-bg);
  padding: 0.3rem 0.45rem;
  font-size: 0.6875rem;
  line-height: 1.2;
  opacity: 0;
  pointer-events: none;
  transition: opacity 120ms ease, transform 120ms ease;
}

.ui-icon-button-wrap:hover .ui-icon-button-tooltip,
.ui-icon-button:focus-visible + .ui-icon-button-tooltip {
  opacity: 1;
  transform: translateX(-50%) translateY(0);
}
</style>
