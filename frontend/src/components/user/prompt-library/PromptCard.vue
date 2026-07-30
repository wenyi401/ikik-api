<template>
  <article class="prompt-card" @click="emit('open')">
    <PromptMediaPreview :item="item" :title="title" />
    <div class="prompt-card__body">
      <div class="prompt-card__topline">
        <span class="prompt-card__type">{{ typeLabel }}</span>
        <button
          type="button"
          class="prompt-card__copy"
          :aria-label="t('promptLibrary.actions.copy')"
          @click.stop="emit('copy')"
        >
          <Icon name="copy" size="sm" />
        </button>
      </div>
      <h2 class="prompt-card__title">{{ title }}</h2>
      <p class="prompt-card__description">{{ description }}</p>
      <div class="prompt-card__footer">
        <span v-if="item.author?.username" class="prompt-card__author">@{{ item.author.username }}</span>
        <span v-else></span>
        <span class="prompt-card__open">
          {{ t('promptLibrary.actions.view') }}
          <Icon name="arrowRight" size="xs" />
        </span>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PromptMediaPreview from './PromptMediaPreview.vue'
import type { PromptLibraryItem } from '@/api/promptLibrary'

const props = defineProps<{
  item: PromptLibraryItem
  title: string
}>()

const emit = defineEmits<{ open: []; copy: [] }>()
const { t, te } = useI18n()
const description = computed(() => props.item.aiDescription || props.item.description || props.item.content.slice(0, 180))
const typeLabel = computed(() => {
  const type = String(props.item.type || 'TEXT').toLowerCase()
  const key = `promptLibrary.types.${type}`
  return te(key) ? t(key) : type.replace(/[_-]+/g, ' ')
})
</script>

<style scoped>
.prompt-card {
  display: flex;
  min-width: 0;
  min-height: 13.5rem;
  overflow: hidden;
  cursor: pointer;
  flex-direction: column;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 0.875rem;
  transition: border-color 180ms ease, transform 180ms ease, background-color 180ms ease;
}

.prompt-card__body {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  padding: 1.125rem;
}

.prompt-card:hover {
  border-color: color-mix(in srgb, var(--app-text) 24%, var(--app-border));
  transform: translateY(-1px);
}

.prompt-card:active {
  transform: translateY(0);
}

.prompt-card__topline,
.prompt-card__footer {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.prompt-card__type {
  color: var(--app-muted);
  font-size: 0.75rem;
  font-weight: 500;
}

.prompt-card__copy {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: var(--app-muted);
  transition: background-color 160ms ease, color 160ms ease;
}

.prompt-card__copy:hover {
  background: var(--app-surface-muted);
  color: var(--app-text);
}

.prompt-card__title {
  margin-top: 0.75rem;
  color: var(--app-text);
  font-size: 1rem;
  font-weight: 650;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.prompt-card__description {
  display: -webkit-box;
  margin-top: 0.5rem;
  overflow: hidden;
  color: var(--app-muted);
  font-size: 0.8125rem;
  line-height: 1.65;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 4;
}

.prompt-card__footer {
  margin-top: auto;
  padding-top: 1rem;
  color: var(--app-muted);
  font-size: 0.75rem;
}

.prompt-card__author {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.prompt-card__open {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.25rem;
  color: var(--app-text);
  font-weight: 600;
}

@media (max-width: 640px) {
  .prompt-card {
    min-height: 0;
  }

  .prompt-card__body {
    padding: 1rem;
  }

  .prompt-card__description {
    -webkit-line-clamp: 3;
  }
}
</style>
