<template>
  <dl class="ui-metric-strip">
    <slot />
  </dl>
</template>

<style scoped>
.ui-metric-strip {
  display: grid;
  grid-template-columns: repeat(var(--metric-columns, 4), minmax(0, 1fr));
  column-gap: clamp(1rem, 3vw, 2.5rem);
  row-gap: 1.25rem;
}

@media (max-width: 900px) {
  .ui-metric-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    column-gap: 1rem;
    row-gap: 1rem;
  }
}

@media (max-width: 520px) {
  .ui-metric-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .ui-metric-strip :deep(.ui-metric:last-child:nth-child(odd)) {
    grid-column: 1 / -1;
  }
}
</style>
