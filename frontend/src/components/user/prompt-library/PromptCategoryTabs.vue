<template>
  <nav class="prompt-category-tabs" role="tablist" :aria-label="t('promptLibrary.categories.label')">
    <button
      v-for="category in promptLibraryCategories"
      :key="category.key"
      type="button"
      role="tab"
      class="prompt-category-tab"
      :class="{ 'prompt-category-tab--active': category.key === selected }"
      :aria-selected="category.key === selected"
      @click="emit('select', category.key)"
    >
      {{ t(`promptLibrary.categories.${category.key}`) }}
    </button>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { promptLibraryCategories, type PromptLibraryCategoryKey } from './categories'

defineProps<{ selected: PromptLibraryCategoryKey }>()
const emit = defineEmits<{ select: [key: PromptLibraryCategoryKey] }>()
const { t } = useI18n()
</script>

<style scoped>
.prompt-category-tabs {
  display: flex;
  width: 100%;
  min-width: 0;
  gap: 0.375rem;
  overflow-x: auto;
  padding: 0.125rem 0 0.25rem;
  scrollbar-width: none;
  scroll-snap-type: x proximity;
}

.prompt-category-tabs::-webkit-scrollbar {
  display: none;
}

.prompt-category-tab {
  flex: 0 0 auto;
  min-height: 2.25rem;
  padding: 0 0.75rem;
  border: 1px solid transparent;
  border-radius: 0.625rem;
  color: var(--app-muted);
  font-size: 0.8125rem;
  font-weight: 600;
  scroll-snap-align: start;
  transition: background-color 160ms ease, color 160ms ease, border-color 160ms ease;
}

.prompt-category-tab:hover {
  background: var(--app-surface-muted);
  color: var(--app-text);
}

.prompt-category-tab--active,
.prompt-category-tab--active:hover {
  border-color: var(--app-text);
  background: var(--app-text);
  color: var(--app-surface);
}

@media (max-width: 640px) {
  .prompt-category-tabs {
    margin-right: -1rem;
    padding-right: 1rem;
  }
}
</style>
