<template>
  <div class="ui-toolbar">
    <div v-if="$slots.default" class="ui-toolbar-main">
      <slot />
    </div>
    <div v-if="$slots.actions" class="ui-toolbar-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<style scoped>
.ui-toolbar {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding-block: 0.25rem;
}

.ui-toolbar-main,
.ui-toolbar-actions {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}

.ui-toolbar-main {
  flex: 1 1 auto;
}

.ui-toolbar-actions {
  flex: 0 0 auto;
  justify-content: flex-end;
}

@media (max-width: 640px) {
  .ui-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .ui-toolbar-actions {
    justify-content: flex-start;
  }
}
</style>
