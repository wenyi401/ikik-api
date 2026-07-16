<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-5xl space-y-6 p-4 sm:p-6">
      <section class="card p-5">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.store.fileStorageTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.store.fileStorageDescription') }}</p>
          </div>
          <span class="badge badge-primary">{{ t('admin.store.fileCardMaxSize', { size: formatBytes(form.max_size_bytes) }) }}</span>
        </div>
      </section>

      <section class="card p-5">
        <div v-if="loading" class="flex min-h-40 items-center justify-center">
          <div class="h-6 w-6 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
        </div>
        <form v-else class="space-y-5" @submit.prevent="saveConfig">
          <label class="flex min-h-[44px] items-center gap-3 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="form.enabled" type="checkbox" />
            <span>{{ t('admin.store.fileStorageEnabled') }}</span>
          </label>

          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.store.ossEndpoint') }}</label>
              <input v-model.trim="form.endpoint" class="input" placeholder="https://oss-cn-hangzhou.aliyuncs.com" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.ossRegion') }}</label>
              <input v-model.trim="form.region" class="input" placeholder="oss-cn-hangzhou" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.ossBucket') }}</label>
              <input v-model.trim="form.bucket" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.ossPrefix') }}</label>
              <input v-model.trim="form.prefix" class="input" placeholder="shop-file-cards/" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.ossAccessKeyId') }}</label>
              <input v-model.trim="form.access_key_id" class="input" autocomplete="off" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.ossSecretAccessKey') }}</label>
              <input v-model="form.secret_access_key" type="password" class="input" autocomplete="new-password" :placeholder="form.secret_access_key_configured ? t('admin.store.ossSecretConfigured') : ''" />
            </div>
          </div>

          <label class="flex min-h-[44px] items-center gap-3 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="form.force_path_style" type="checkbox" />
            <span>{{ t('admin.store.ossForcePathStyle') }}</span>
          </label>

          <div class="flex flex-wrap justify-end gap-2">
            <button type="button" class="btn btn-secondary min-h-[44px]" :disabled="testing || saving" @click="testConfig">
              {{ testing ? t('common.loading') : t('admin.store.testStorage') }}
            </button>
            <button type="submit" class="btn btn-primary min-h-[44px]" :disabled="saving || testing">
              {{ saving ? t('common.saving') : t('common.save') }}
            </button>
          </div>
        </form>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminStoreAPI } from '@/api/admin/store'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { StoreFileCardStorageConfig, UpdateStoreFileCardStorageConfigRequest } from '@/types/store'
import AppLayout from '@/components/layout/AppLayout.vue'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)

const form = reactive<StoreFileCardStorageConfig>({
  enabled: false,
  endpoint: '',
  region: 'oss-cn-hangzhou',
  bucket: '',
  access_key_id: '',
  secret_access_key: '',
  secret_access_key_configured: false,
  prefix: 'shop-file-cards/',
  force_path_style: false,
  max_size_bytes: 200 * 1024,
})

function applyConfig(config: StoreFileCardStorageConfig) {
  form.enabled = config.enabled
  form.endpoint = config.endpoint || ''
  form.region = config.region || 'oss-cn-hangzhou'
  form.bucket = config.bucket || ''
  form.access_key_id = config.access_key_id || ''
  form.secret_access_key = ''
  form.secret_access_key_configured = config.secret_access_key_configured
  form.prefix = config.prefix || 'shop-file-cards/'
  form.force_path_style = config.force_path_style
  form.max_size_bytes = config.max_size_bytes || 200 * 1024
}

function buildPayload(): UpdateStoreFileCardStorageConfigRequest {
  return {
    enabled: form.enabled,
    endpoint: form.endpoint.trim(),
    region: form.region.trim(),
    bucket: form.bucket.trim(),
    access_key_id: form.access_key_id.trim(),
    secret_access_key: (form.secret_access_key ?? '').trim(),
    prefix: form.prefix.trim(),
    force_path_style: form.force_path_style,
  }
}

async function loadConfig() {
  loading.value = true
  try {
    const { data } = await adminStoreAPI.getFileCardStorage()
    applyConfig(data)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.store.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  saving.value = true
  try {
    const { data } = await adminStoreAPI.updateFileCardStorage(buildPayload())
    applyConfig(data)
    appStore.showSuccess(t('common.saved'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    saving.value = false
  }
}

async function testConfig() {
  testing.value = true
  try {
    await adminStoreAPI.testFileCardStorage(buildPayload())
    appStore.showSuccess(t('admin.store.storageTestSuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    testing.value = false
  }
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  return `${Math.round(bytes / 1024)} KB`
}

onMounted(loadConfig)
</script>
