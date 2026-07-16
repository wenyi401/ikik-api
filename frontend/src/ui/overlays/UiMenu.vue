<template>
  <div ref="rootRef" class="ui-menu">
    <slot name="trigger" :open="open" :toggle="toggle">
      <button
        type="button"
        class="btn btn-secondary"
        aria-haspopup="menu"
        :aria-expanded="open"
        @click="toggle"
      >
        {{ label }}
      </button>
    </slot>
    <Transition name="ui-menu-fade">
      <div v-if="open" class="ui-menu-content" role="menu">
        <slot :close="close" />
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

defineProps<{ label: string }>()

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)

function close() {
  open.value = false
}

function toggle() {
  open.value = !open.value
}

function handlePointerDown(event: MouseEvent) {
  if (!rootRef.value?.contains(event.target as Node)) close()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') close()
}

onMounted(() => {
  document.addEventListener('mousedown', handlePointerDown)
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('mousedown', handlePointerDown)
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<style scoped>
.ui-menu {
  position: relative;
}

.ui-menu-content {
  position: absolute;
  z-index: 60;
  top: calc(100% + 0.375rem);
  right: 0;
  display: flex;
  width: max-content;
  min-width: 12rem;
  max-width: min(20rem, calc(100vw - 1rem));
  flex-direction: column;
  padding: 0.375rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
  box-shadow: var(--ui-shadow-popover);
}

.ui-menu-content :deep(button),
.ui-menu-content :deep(a) {
  display: flex;
  width: 100%;
  min-height: 2.25rem;
  align-items: center;
  border-radius: var(--ui-radius-md);
  padding: 0.5rem 0.625rem;
  color: var(--ui-text-secondary);
  font-size: 0.875rem;
  text-align: left;
  transition: background-color 150ms ease, color 150ms ease;
}

.ui-menu-content :deep(.btn) {
  justify-content: flex-start;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.ui-menu-content :deep(button:hover),
.ui-menu-content :deep(a:hover) {
  background: var(--ui-surface-hover);
  color: var(--ui-text);
}

.ui-menu-content :deep(button:disabled) {
  cursor: not-allowed;
  opacity: 0.45;
}

.ui-menu-fade-enter-active,
.ui-menu-fade-leave-active {
  transition: opacity 120ms ease, transform 120ms ease;
}

.ui-menu-fade-enter-from,
.ui-menu-fade-leave-to {
  opacity: 0;
  transform: translateY(-2px);
}
</style>
