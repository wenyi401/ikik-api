<template>
  <BaseDialog
    :show="Boolean(item)"
    :title="title"
    width="wide"
    panel-class="prompt-detail-dialog"
    @close="emit('close')"
    >
    <div v-if="item" class="prompt-detail">
      <PromptMediaPreview :item="item" :title="title" variant="detail" />
      <div class="prompt-detail__meta">
        <span>{{ typeLabel }}</span>
        <span v-if="item.author?.username">@{{ item.author.username }}</span>
      </div>
      <p v-if="description" class="prompt-detail__description">{{ description }}</p>
      <pre class="prompt-detail__content">{{ item.content }}</pre>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">
        {{ t('promptLibrary.actions.close') }}
      </button>
      <button type="button" class="btn btn-primary inline-flex items-center gap-2" @click="emit('copy')">
        <Icon name="copy" size="sm" />
        {{ t('promptLibrary.actions.copy') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PromptMediaPreview from './PromptMediaPreview.vue'
import type { PromptLibraryItem } from '@/api/promptLibrary'

const props = defineProps<{
  item: PromptLibraryItem | null
  title: string
}>()

const emit = defineEmits<{ close: []; copy: [] }>()
const { t } = useI18n()
const typeLabel = computed(() => t(`promptLibrary.types.${String(props.item?.type || 'TEXT').toLowerCase()}`))
const description = computed(() => props.item?.aiDescription || props.item?.description || '')
</script>

<style scoped>
.prompt-detail {
  min-width: 0;
}

.prompt-detail__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
  color: var(--app-muted);
  font-size: 0.75rem;
}

.prompt-detail__description {
  margin-top: 1rem;
  color: var(--app-muted);
  font-size: 0.875rem;
  line-height: 1.65;
}

.prompt-detail__content {
  margin-top: 1rem;
  min-width: 0;
  overflow: visible;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: var(--app-text);
  font-family: inherit;
  font-size: 0.875rem;
  line-height: 1.75;
}

@media (max-width: 640px) {
  :global(.prompt-detail-dialog) {
    max-height: calc(100dvh - 1rem);
  }
}
</style>
