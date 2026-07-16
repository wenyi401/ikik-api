<template>
  <div class="ui-metric">
    <dt class="ui-metric-label">{{ label }}</dt>
    <dd class="ui-metric-value" :class="toneClass">{{ value }}</dd>
    <dd v-if="detail" class="ui-metric-detail">{{ detail }}</dd>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  label: string
  value: string | number
  detail?: string
  tone?: 'default' | 'success' | 'danger'
}>(), {
  detail: '',
  tone: 'default'
})

const toneClass = computed(() => `ui-metric-value--${props.tone}`)
</script>

<style scoped>
.ui-metric {
  min-width: 0;
  padding: 1rem 1.25rem;
}

.ui-metric-label {
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  line-height: 1.4;
}

.ui-metric-value {
  margin-top: 0.35rem;
  overflow: hidden;
  color: var(--ui-text);
  font-size: 1.5rem;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ui-metric-value--success {
  color: var(--ui-success);
}

.ui-metric-value--danger {
  color: var(--ui-danger);
}

.ui-metric-detail {
  margin-top: 0.25rem;
  overflow: hidden;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 520px) {
  .ui-metric {
    padding: 0.875rem 0.75rem;
  }

  .ui-metric-value {
    font-size: 1.25rem;
  }

  .ui-metric-detail {
    overflow: visible;
    text-overflow: clip;
    white-space: normal;
  }
}
</style>
