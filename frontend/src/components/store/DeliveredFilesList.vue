<template>
  <div v-if="normalizedFiles.length > 0">
    <div class="mb-2 flex flex-wrap items-center justify-between gap-3">
      <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('store.deliveredFiles') }}</label>
      <button
        v-if="normalizedFiles.length > 1"
        type="button"
        class="btn btn-secondary btn-sm min-h-[2.5rem]"
        @click="handleDownloadAllFiles"
      >
        <Icon name="download" size="sm" />
        <span>{{ t('store.downloadAllFiles') }}</span>
      </button>
    </div>
    <div class="space-y-2 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
      <div
        v-for="file in normalizedFiles"
        :key="file.id"
        class="flex flex-col gap-3 rounded-md bg-white px-3 py-2 dark:bg-dark-900 sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="min-w-0">
          <div class="break-all text-sm font-medium text-gray-900 dark:text-dark-100">{{ file.filename }}</div>
          <div class="text-xs text-gray-500 dark:text-dark-400">{{ formatBytes(file.byte_size) }}</div>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm min-h-[2.5rem] shrink-0 justify-center"
          @click="handleDownloadFile(file.id, file.filename)"
        >
          <Icon name="download" size="sm" />
          <span>{{ t('store.downloadFile') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeAPI } from '@/api/store'
import type { StoreDeliveredFile } from '@/types/store'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  orderId: number
  files?: StoreDeliveredFile[]
  downloadFile?: (orderId: number, cardId: number, filename: string) => Promise<void>
  downloadAllFiles?: (orderId: number) => Promise<void>
}>()

const { t } = useI18n()
const normalizedFiles = computed(() => props.files || [])

async function handleDownloadFile(cardId: number, filename: string): Promise<void> {
  if (props.downloadFile) {
    await props.downloadFile(props.orderId, cardId, filename)
    return
  }
  await storeAPI.downloadOrderFile(props.orderId, cardId, filename)
}

async function handleDownloadAllFiles(): Promise<void> {
  if (props.downloadAllFiles) {
    await props.downloadAllFiles(props.orderId)
    return
  }
  await storeAPI.downloadOrderFilesZip(props.orderId)
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB']
  let value = bytes
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}
</script>
