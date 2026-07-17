<template>
  <div class="table-page-layout" :class="{ 'mobile-mode': isMobile }">
    <!-- 固定区域：操作按钮 -->
    <div v-if="$slots.actions" class="layout-section-fixed">
      <slot name="actions" />
    </div>

    <!-- 固定区域：搜索和过滤器 -->
    <div v-if="$slots.filters" class="layout-section-fixed">
      <slot name="filters" />
    </div>

    <!-- 滚动区域：表格 -->
    <div class="layout-section-scrollable">
      <div class="table-scroll-container">
        <slot name="table" />
      </div>
    </div>

    <!-- 固定区域：分页器 -->
    <div v-if="$slots.pagination" class="layout-section-fixed">
      <slot name="pagination" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const isMobile = ref(false)

const checkMobile = () => {
  isMobile.value = window.innerWidth < 1024
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
/* 桌面端：Flexbox 布局 */
.table-page-layout {
  @apply flex flex-col gap-5;
  min-height: calc(100vh - 64px - 3.5rem);
}

.layout-section-fixed {
  @apply flex-shrink-0;
}

.layout-section-scrollable {
  @apply min-h-0;
}

/* 表格滚动容器 - 增强版表体滚动方案 */
.table-scroll-container {
  @apply overflow-hidden;
  border-radius: 0;
  border-color: var(--app-border);
  background: transparent;
  box-shadow: none;
}

.table-scroll-container :deep(.table-wrapper) {
  @apply overflow-x-auto overflow-y-visible;
  /* 确保横向滚动条显示在最底部 */
  scrollbar-gutter: stable;
}

.table-scroll-container :deep(table) {
  @apply w-full;
  min-width: max-content; /* 关键：确保表格宽度根据内容撑开，从而触发横向滚动 */
  display: table; /* 使用标准 table 布局以支持 sticky 列 */
}

.table-scroll-container :deep(thead) {
  background: var(--app-bg);
}

.table-scroll-container :deep(tbody) {
  /* 保持默认 table-row-group 显示，不使用 block */
}

.table-scroll-container :deep(th) {
  @apply px-5 py-3 text-left text-sm font-medium;
  border-bottom: 1px solid var(--app-border);
  color: var(--app-muted-strong);
}

.table-scroll-container :deep(td) {
  @apply px-5 py-3 text-sm;
  border-bottom: 1px solid var(--app-border);
  color: var(--app-text);
}

.dark .table-scroll-container {
  border-color: var(--app-border);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.03);
}

.dark .table-scroll-container :deep(thead) {
  background: var(--app-bg);
}

.dark .table-scroll-container :deep(th) {
  border-bottom-color: var(--app-border);
  color: var(--app-muted-strong);
}

.dark .table-scroll-container :deep(td) {
  border-bottom-color: var(--app-border);
  color: var(--app-text);
}

/* 移动端：恢复正常滚动 */
.table-page-layout.mobile-mode .table-scroll-container {
  @apply h-auto overflow-visible border-none bg-transparent;
  box-shadow: none;
}

.table-page-layout.mobile-mode .layout-section-scrollable {
  @apply flex-none min-h-fit;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(table) {
  @apply flex-none;
  display: table;
  min-width: 100%;
}
</style>
