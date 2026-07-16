<template>
  <header class="ui-page-header">
    <div class="ui-page-header-copy">
      <h1 v-if="title" class="ui-page-header-title">{{ title }}</h1>
      <p v-if="description" class="ui-page-header-description">{{ description }}</p>
      <slot />
    </div>
    <div v-if="$slots.actions" class="ui-page-header-actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
defineProps<{
  title?: string
  description?: string
}>()
</script>

<style scoped>
.ui-page-header {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ui-page-header-copy {
  min-width: 0;
}

.ui-page-header-title {
  color: var(--ui-text);
  font-size: 1.5rem;
  font-weight: 600;
  line-height: 1.3;
}

.ui-page-header-description {
  margin-top: 0.25rem;
  color: var(--ui-text-tertiary);
  font-size: 0.875rem;
  line-height: 1.5;
}

.ui-page-header-actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}

@media (max-width: 640px) {
  .ui-page-header {
    align-items: center;
  }

  .ui-page-header-title {
    font-size: 1.25rem;
  }

  .ui-page-header-description {
    display: none;
  }
}
</style>
