<template>
  <div v-if="mediaUrl" class="prompt-media" :class="`prompt-media--${variant}`">
    <video
      v-if="isVideo"
      class="prompt-media__asset"
      :src="videoSource"
      :aria-label="title"
      controls
      muted
      playsinline
      preload="metadata"
      referrerpolicy="no-referrer"
      @click.stop
      @pointerdown.stop
    ></video>
    <img
      v-else
      class="prompt-media__asset"
      :src="mediaUrl"
      :alt="title"
      loading="lazy"
      decoding="async"
      referrerpolicy="no-referrer"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PromptLibraryItem } from '@/api/promptLibrary'

const props = withDefaults(defineProps<{
  item: PromptLibraryItem
  title: string
  variant?: 'card' | 'detail'
}>(), {
  variant: 'card',
})

const mediaUrl = computed(() => {
  const primary = String(props.item.mediaUrl || '').trim()
  if (primary) return primary
  return String(props.item.userExamples?.find(example => example.mediaUrl)?.mediaUrl || '').trim()
})

const isVideo = computed(() => {
  if (String(props.item.type).toUpperCase() === 'VIDEO') return true
  return /\.(?:mp4|webm|mov)(?:$|[?#])/i.test(mediaUrl.value)
})

const videoSource = computed(() => {
  if (props.variant !== 'card' || mediaUrl.value.includes('#')) return mediaUrl.value
  return `${mediaUrl.value}#t=0.1`
})
</script>

<style scoped>
.prompt-media {
  position: relative;
  overflow: hidden;
  background: var(--app-surface-muted);
}

.prompt-media--card {
  aspect-ratio: 16 / 9;
  border-bottom: 1px solid var(--app-border);
}

.prompt-media--detail {
  max-height: min(26rem, 48dvh);
  margin-bottom: 1rem;
  border: 1px solid var(--app-border);
  border-radius: 0.75rem;
}

.prompt-media__asset {
  display: block;
  width: 100%;
  height: 100%;
  max-height: inherit;
  object-fit: cover;
}

.prompt-media--detail .prompt-media__asset {
  object-fit: contain;
}

</style>
