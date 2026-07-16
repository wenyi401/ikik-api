<template>
  <section class="ui-section" :class="`ui-section--${surface}`">
    <header v-if="title || description || $slots.actions" class="ui-section-header">
      <div class="min-w-0">
        <h2 v-if="title" class="ui-section-title">{{ title }}</h2>
        <p v-if="description" class="ui-section-description">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="ui-section-actions">
        <slot name="actions" />
      </div>
    </header>
    <div class="ui-section-body">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  title?: string
  description?: string
  surface?: 'plain' | 'panel'
}>(), {
  surface: 'plain'
})
</script>

<style scoped>
.ui-section {
  min-width: 0;
}

.ui-section--panel {
  position: relative;
  padding: 1rem 1.125rem 0;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.ui-section-header {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 0 0 0.875rem;
}

.ui-section-title {
  color: var(--ui-text);
  font-size: 0.9375rem;
  font-weight: 600;
  line-height: 1.4;
}

.ui-section-description {
  margin-top: 0.2rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  line-height: 1.45;
}

.ui-section-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.5rem;
}

.ui-section-body {
  min-width: 0;
  padding-bottom: 1rem;
}

@media (max-width: 640px) {
  .ui-section--panel {
    padding: 0.875rem 0.875rem 0;
  }
}
</style>
