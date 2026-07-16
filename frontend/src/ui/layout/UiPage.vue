<template>
  <div class="ui-page" :class="[`ui-page--${width}`, `ui-page--${density}`]">
    <slot />
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  width?: 'narrow' | 'standard' | 'wide' | 'full'
  density?: 'comfortable' | 'compact'
}>(), {
  width: 'standard',
  density: 'comfortable'
})
</script>

<style scoped>
.ui-page {
  width: 100%;
  min-width: 0;
  margin-inline: auto;
}

.ui-page--narrow {
  max-width: var(--ui-page-narrow);
}

.ui-page--standard {
  max-width: var(--ui-page-standard);
}

.ui-page--wide {
  max-width: var(--ui-page-wide);
}

.ui-page--full {
  max-width: none;
}

.ui-page--comfortable {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.ui-page--compact {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

@media (max-width: 640px) {
  .ui-page--comfortable,
  .ui-page--compact {
    gap: 1rem;
  }
}
</style>
