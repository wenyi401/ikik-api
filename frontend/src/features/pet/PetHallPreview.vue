<template>
  <div ref="root" class="pet-hall-preview" :class="{ 'pet-hall-preview--loaded': Boolean(src) }">
    <PetSprite
      v-if="src"
      :asset="asset"
      :src="src"
      state="idle"
      size="medium"
      :reduced-motion="true"
    />
    <span v-else-if="loading" class="pet-hall-preview__skeleton" aria-hidden="true" />
    <Icon v-else-if="failed" name="cube" size="lg" class="pet-hall-preview__fallback" />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import type { PetAsset } from '@/api/pet'
import { petAPI } from '@/api/pet'
import Icon from '@/components/icons/Icon.vue'
import PetSprite from './PetSprite.vue'

const props = defineProps<{ asset: PetAsset }>()

const root = ref<HTMLElement | null>(null)
const src = ref('')
const loading = ref(false)
const failed = ref(false)
let observer: IntersectionObserver | undefined
let disposed = false

async function load() {
  if (src.value || loading.value || failed.value) return
  loading.value = true
  try {
    const blob = await petAPI.fetchSpritesheet(props.asset)
    if (disposed) return
    src.value = URL.createObjectURL(blob)
  } catch {
    if (!disposed) failed.value = true
  } finally {
    if (!disposed) loading.value = false
  }
}

onMounted(() => {
  if (typeof IntersectionObserver === 'undefined') {
    void load()
    return
  }
  observer = new IntersectionObserver((entries) => {
    if (!entries.some((entry) => entry.isIntersecting)) return
    observer?.disconnect()
    void load()
  }, { rootMargin: '180px 0px' })
  if (root.value) observer.observe(root.value)
})

onBeforeUnmount(() => {
  disposed = true
  observer?.disconnect()
  if (src.value) URL.revokeObjectURL(src.value)
})
</script>

<style scoped>
.pet-hall-preview {
  display: grid;
  width: 100%;
  min-height: 7.5rem;
  place-items: center;
  overflow: hidden;
  background:
    linear-gradient(var(--ui-border) 1px, transparent 1px),
    linear-gradient(90deg, var(--ui-border) 1px, transparent 1px),
    var(--ui-surface-subtle);
  background-size: 1.25rem 1.25rem;
  color: var(--ui-text-tertiary);
}

.pet-hall-preview--loaded {
  background-color: color-mix(in srgb, var(--ui-surface-subtle) 82%, var(--app-primary-soft));
}

.pet-hall-preview__skeleton {
  width: 4.75rem;
  height: 5.25rem;
  border-radius: 0.375rem;
  background: linear-gradient(90deg, var(--ui-border) 25%, var(--ui-surface-hover) 50%, var(--ui-border) 75%);
  background-size: 200% 100%;
  animation: pet-preview-loading 1.2s ease-in-out infinite;
}

.pet-hall-preview__fallback {
  opacity: 0.65;
}

@keyframes pet-preview-loading {
  from { background-position: 200% 0; }
  to { background-position: -200% 0; }
}

@media (prefers-reduced-motion: reduce) {
  .pet-hall-preview__skeleton { animation: none; }
}
</style>
