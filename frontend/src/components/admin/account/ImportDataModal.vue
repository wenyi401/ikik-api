<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.dataImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form
      id="import-data-form"
      class="space-y-4"
      @submit.prevent="handleImport"
    >
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.dataImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-600 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ t('admin.accounts.dataImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{
          t('admin.accounts.dataImportFile')
        }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed px-4 py-3 transition-colors"
          :class="dragActive
            ? 'border-primary-400 bg-primary-50/70 dark:border-primary-500 dark:bg-primary-900/20'
            : 'border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800'"
          @dragenter.prevent="handleDragEnter"
          @dragover.prevent
          @dragleave.prevent="handleDragLeave"
          @drop.prevent="handleDrop"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200" :title="fileListTitle">
              {{ fileLabel || t('admin.accounts.dataImportSelectFile') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">
              JSON/TXT/ZIP (.json, .txt, .zip)
              <span v-if="files.length > 1"> · {{ fileListTitle }}</span>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-secondary shrink-0"
            @click="openFilePicker"
          >
            {{ t('common.chooseFile') }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          accept="application/json,text/plain,application/zip,.json,.txt,.zip"
          multiple
          @change="handleFileChange"
        />
      </div>

      <div>
        <label class="input-label">{{
          t('admin.accounts.dataImportURL')
        }}</label>
        <textarea
          v-model="sourceURLs"
          class="input min-h-20"
          rows="3"
          :placeholder="t('admin.accounts.dataImportURLPlaceholder')"
        ></textarea>
        <p class="input-hint">{{ t('admin.accounts.dataImportURLHint') }}</p>
      </div>

      <div>
        <label class="input-label">
          {{ t('admin.accounts.dataImportTargetGroups') }}
          <span class="font-normal text-gray-400">
            {{ t('common.selectedCount', { count: selectedGroupIds.length }) }}
          </span>
        </label>
        <div
          class="grid max-h-44 grid-cols-1 gap-1 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-2 dark:border-dark-600 dark:bg-dark-800 sm:grid-cols-2"
        >
          <label
            v-for="group in importTargetGroups"
            :key="group.id"
            class="flex min-w-0 cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm transition-colors hover:bg-white dark:hover:bg-dark-700"
          >
            <input
              v-model="selectedGroupIds"
              type="checkbox"
              :value="group.id"
              :disabled="importing || extracting"
              class="h-3.5 w-3.5 shrink-0 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
            />
            <span
              class="min-w-0 flex-1 truncate text-gray-700 dark:text-dark-200"
            >
              {{ group.name }}
            </span>
            <span
              class="shrink-0 rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-dark-300"
            >
              {{ group.platform }}
            </span>
            <span
              v-if="group.status === 'inactive'"
              class="shrink-0 rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
            >
              {{ t('admin.accounts.dataImportGroupInactive') }}
            </span>
          </label>
          <div
            v-if="importTargetGroups.length === 0"
            class="py-2 text-center text-sm text-gray-500 dark:text-gray-400 sm:col-span-2"
          >
            {{ t('common.noGroupsAvailable') }}
          </div>
        </div>
        <p class="input-hint">
          {{ t('admin.accounts.dataImportTargetGroupsHint') }}
        </p>
      </div>

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label
          class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700"
        >
          <input
            v-model="compatibilityMode"
            type="checkbox"
            class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          <span class="min-w-0">
            <span
              class="block text-sm font-medium text-gray-900 dark:text-white"
            >
              {{ t('admin.accounts.dataImportCompatibilityMode') }}
            </span>
            <span
              class="block text-xs leading-5 text-gray-500 dark:text-dark-400"
            >
              {{ t('admin.accounts.dataImportCompatibilityModeHint') }}
            </span>
          </span>
        </label>

        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <label
            class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-dark-200"
          >
            <input
              v-model="overrideDefaults"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
            {{ t('admin.accounts.dataImportBatchSettings') }}
          </label>
          <div
            class="mt-3 grid grid-cols-2 gap-2"
            :class="{ 'opacity-50': !overrideDefaults }"
          >
            <label class="block">
              <span class="input-label">{{
                t('admin.accounts.concurrency')
              }}</span>
              <input
                v-model.number="defaults.concurrency"
                :disabled="!overrideDefaults"
                type="number"
                min="0"
                class="input"
              />
            </label>
            <label class="block">
              <span class="input-label">{{
                t('admin.accounts.priority')
              }}</span>
              <input
                v-model.number="defaults.priority"
                :disabled="!overrideDefaults"
                type="number"
                min="0"
                class="input"
              />
            </label>
            <label class="block">
              <span class="input-label">{{
                t('admin.accounts.dataImportRateMultiplier')
              }}</span>
              <input
                v-model.number="defaults.rate_multiplier"
                :disabled="!overrideDefaults"
                type="number"
                min="0"
                step="0.001"
                class="input"
              />
            </label>
            <label
              class="mt-6 flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200"
            >
              <input
                v-model="defaults.auto_pause_on_expired"
                :disabled="!overrideDefaults"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              />
              {{ t('admin.accounts.dataImportAutoPause') }}
            </label>
          </div>
        </div>
      </div>

      <div
        v-if="importing || importProgress.total > 0"
        class="space-y-2 rounded-lg border border-gray-200 p-3 dark:border-dark-700"
      >
        <div
          class="flex items-center justify-between text-xs text-gray-600 dark:text-dark-300"
        >
          <span>{{ t('admin.accounts.dataImportProgress') }}</span>
          <span
            >{{ importProgress.completed }} / {{ importProgress.total }}</span
          >
        </div>
        <div
          class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"
        >
          <div
            class="h-full rounded-full bg-primary-600 transition-all"
            :style="{ width: `${importProgressPercent}%` }"
          ></div>
        </div>
        <div class="truncate text-xs text-gray-500 dark:text-dark-400">
          {{
            importProgress.current || t('admin.accounts.dataImportProgressIdle')
          }}
        </div>
      </div>

      <div
        v-if="result"
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.dataImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{
            result.credential_import
              ? t('userAccounts.importResultSummary', {
                  created: result.account_created,
                  skipped: result.account_skipped || 0,
                  failed: result.account_failed
                })
              : t('admin.accounts.dataImportResultSummary', result)
          }}
        </div>

        <div v-if="errorItems.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('admin.accounts.dataImportErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
          >
            <div
              v-for="(item, idx) in errorItems"
              :key="idx"
              class="whitespace-pre-wrap"
            >
              {{ item.kind }} {{ item.name || item.proxy_key || '-' }} -
              {{ item.message }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          class="btn btn-secondary"
          type="button"
          :disabled="importing || extracting"
          @click="handleClose"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="import-data-form"
          :disabled="importing || extracting"
        >
          {{
            importing || extracting
              ? t('admin.accounts.dataImporting')
              : t('admin.accounts.dataImportButton')
          }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type {
  AdminDataImportPayload,
  AdminDataImportResult,
  AdminDataPayload,
  AdminGroup
} from '@/types'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'imported'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

const importing = ref(false)
const extracting = ref(false)
const dragActive = ref(false)
const files = ref<File[]>([])
const sourceURLs = ref('')
const importTargetGroups = ref<AdminGroup[]>([])
const selectedGroupIds = ref<number[]>([])
const compatibilityMode = ref(false)
const overrideDefaults = ref(false)
const defaults = reactive({
  concurrency: 10,
  priority: 1,
  rate_multiplier: 1,
  auto_pause_on_expired: true
})
const importProgress = reactive({
  completed: 0,
  total: 0,
  current: ''
})

type ImportResult = AdminDataImportResult & {
  credential_import?: boolean
  account_skipped?: number
}

const result = ref<ImportResult | null>(null)

const fileInput = ref<HTMLInputElement | null>(null)
const fileLabel = computed(() => {
  if (files.value.length === 0) return ''
  if (files.value.length === 1) return files.value[0].name
  return t('admin.accounts.dataImportSelectedFiles', {
    count: files.value.length
  })
})
const fileListTitle = computed(() => files.value.map((file) => file.name).join(', '))

const errorItems = computed(() => result.value?.errors || [])
const selectedImportTargetGroups = computed(() =>
  importTargetGroups.value.filter((group) =>
    selectedGroupIds.value.includes(group.id)
  )
)
const importProgressPercent = computed(() => {
  if (importProgress.total <= 0) return 0
  return Math.min(
    100,
    Math.round((importProgress.completed / importProgress.total) * 100)
  )
})

watch(
  () => props.show,
  (open) => {
    if (open) {
      files.value = []
      sourceURLs.value = ''
      result.value = null
      selectedGroupIds.value = []
      compatibilityMode.value = false
      overrideDefaults.value = false
      resetImportProgress()
      void loadImportTargetGroups()
      if (fileInput.value) {
        fileInput.value.value = ''
      }
    }
  },
  { immediate: true }
)

const openFilePicker = () => {
  fileInput.value?.click()
}

const setSelectedFiles = async (selectedFiles: File[]) => {
  extracting.value = true
  try {
    const expandedFiles = await expandSelectedImportFiles(selectedFiles)
    files.value = expandedFiles
    if (selectedFiles.length > 0 && expandedFiles.length === 0) {
      appStore.showError(t('admin.accounts.dataImportZipNoImportableFiles'))
    }
  } catch (error: any) {
    files.value = []
    appStore.showError(
      error?.message || t('admin.accounts.dataImportZipFailed')
    )
  } finally {
    extracting.value = false
  }
}

const handleFileChange = async (event: Event) => {
  const target = event.target as HTMLInputElement
  await setSelectedFiles(Array.from(target.files || []))
}

const handleDragEnter = () => {
  dragActive.value = true
}

const handleDragLeave = (event: DragEvent) => {
  const current = event.currentTarget as HTMLElement | null
  const related = event.relatedTarget as Node | null
  if (!current || !related || !current.contains(related)) {
    dragActive.value = false
  }
}

const handleDrop = async (event: DragEvent) => {
  dragActive.value = false
  await setSelectedFiles(Array.from(event.dataTransfer?.files || []))
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

async function loadImportTargetGroups() {
  try {
    const pageSize = 1000
    let page = 1
    let totalPages = 1
    const groups: AdminGroup[] = []

    do {
      const response = await adminAPI.groups.list(page, pageSize, {
        scope: 'public'
      })
      groups.push(...response.items)
      totalPages = response.pages || page
      page += 1
    } while (page <= totalPages)

    importTargetGroups.value = groups
  } catch {
    importTargetGroups.value = []
  }
}

function resetImportProgress() {
  importProgress.completed = 0
  importProgress.total = 0
  importProgress.current = ''
}

const handleClose = () => {
  if (importing.value || extracting.value) return
  emit('close')
}

const readFileAsText = async (sourceFile: File): Promise<string> => {
  if (typeof sourceFile.text === 'function') {
    return sourceFile.text()
  }

  if (typeof sourceFile.arrayBuffer === 'function') {
    const buffer = await sourceFile.arrayBuffer()
    return new TextDecoder().decode(buffer)
  }

  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () =>
      reject(reader.error || new Error('Failed to read file'))
    reader.readAsText(sourceFile)
  })
}

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

const normalizeDataPayload = (value: unknown): AdminDataPayload | null => {
  if (!isRecord(value)) return null

  const directAccounts = Array.isArray(value.accounts)
  const directProxies = Array.isArray(value.proxies)
  if (directAccounts && directProxies) {
    return value as unknown as AdminDataPayload
  }

  const nestedData = value.data
  if (
    isRecord(nestedData) &&
    Array.isArray(nestedData.accounts) &&
    Array.isArray(nestedData.proxies)
  ) {
    return nestedData as unknown as AdminDataPayload
  }

  return null
}

const isLikelyCodexImportPayload = (value: unknown): boolean => {
  if (!isRecord(value)) return false
  const type = String(value.type || '')
    .trim()
    .toLowerCase()
  const hasToken = [
    'refresh_token',
    'refreshToken',
    'access_token',
    'accessToken',
    'id_token',
    'idToken',
    'session_token',
    'sessionToken'
  ].some(
    (key) => typeof value[key] === 'string' && String(value[key]).trim() !== ''
  )
  if (['codex', 'openai', 'chatgpt'].includes(type) && hasToken) return true
  return (
    ('account_id' in value ||
      'account' in value ||
      'chatgpt_account_id' in value) &&
    hasToken
  )
}

const importAsCredentialContents = async (
  contents: string[],
  groupIds: number[]
): Promise<ImportResult> => {
  const credentialResult = await adminAPI.accounts.importCredentialContents({
    contents,
    priority: 50,
    group_ids: groupIds,
    auto_pause_on_expired: true,
    skip_default_group_bind: true
  })

  return {
    credential_import: true,
    account_skipped: 0,
    proxy_created: 0,
    proxy_reused: 0,
    proxy_failed: 0,
    account_created: credentialResult.created,
    account_failed: credentialResult.failed,
    errors: (credentialResult.errors ?? []).map((item) => ({
      kind: 'account' as const,
      name: item.name || `#${item.index}`,
      message: item.message
    }))
  }
}

const validateSelectedTargetGroups = (
  dataPayload: AdminDataPayload | null
): boolean => {
  const selectedGroups = selectedImportTargetGroups.value
  if (selectedGroups.length === 0) return true

  const selectedPlatforms = new Set(
    selectedGroups.map((group) => group.platform)
  )
  if (selectedPlatforms.size > 1) {
    appStore.showError(t('admin.accounts.dataImportTargetGroupMixedPlatforms'))
    return false
  }

  if (!dataPayload) return true

  const expectedPlatform = selectedGroups[0]?.platform
  if (!expectedPlatform) return true

  const mismatchedAccounts = (dataPayload.accounts || []).filter(
    (account) => account.platform !== expectedPlatform
  )
  if (mismatchedAccounts.length === 0) return true

  const examples = mismatchedAccounts
    .slice(0, 5)
    .map((account) => `${account.name || '-'} (${account.platform || '-'})`)
    .join(', ')
  appStore.showError(
    t('admin.accounts.dataImportAccountPlatformMismatch', {
      expected_platform: expectedPlatform,
      mismatch_count: mismatchedAccounts.length,
      examples
    })
  )
  return false
}

const isImportableDataFileName = (name: string) => /\.(json|txt)$/i.test(name)

const isZipFile = (file: File) => {
  const name = file.name.toLowerCase()
  return (
    name.endsWith('.zip') ||
    file.type === 'application/zip' ||
    file.type === 'application/x-zip-compressed'
  )
}

const expandSelectedImportFiles = async (
  selectedFiles: File[]
): Promise<File[]> => {
  const expanded: File[] = []
  let ignoredCount = 0
  for (const selectedFile of selectedFiles) {
    if (isZipFile(selectedFile)) {
      expanded.push(...(await extractImportableFilesFromZip(selectedFile)))
      continue
    }
    if (isImportableDataFileName(selectedFile.name)) {
      expanded.push(selectedFile)
      continue
    }
    ignoredCount++
  }
  if (ignoredCount > 0) {
    appStore.showWarning(t('admin.accounts.dataImportIgnoredFiles', { count: ignoredCount }))
  }
  return expanded
}

const readUint16 = (view: DataView, offset: number) =>
  view.getUint16(offset, true)
const readUint32 = (view: DataView, offset: number) =>
  view.getUint32(offset, true)

const decodeZipName = (bytes: Uint8Array) => new TextDecoder().decode(bytes)

const findZipEndOfCentralDirectory = (view: DataView) => {
  const minOffset = Math.max(0, view.byteLength - 0xffff - 22)
  for (let offset = view.byteLength - 22; offset >= minOffset; offset--) {
    if (readUint32(view, offset) === 0x06054b50) {
      return offset
    }
  }
  return -1
}

const inflateRaw = async (bytes: Uint8Array): Promise<Uint8Array> => {
  const DecompressionStreamCtor = (globalThis as any).DecompressionStream
  if (!DecompressionStreamCtor) {
    throw new Error(t('admin.accounts.dataImportZipUnsupportedCompression'))
  }
  const stream = new Blob([bytes])
    .stream()
    .pipeThrough(new DecompressionStreamCtor('deflate-raw'))
  return new Uint8Array(await new Response(stream).arrayBuffer())
}

const extractImportableFilesFromZip = async (
  zipFile: File
): Promise<File[]> => {
  const buffer = await zipFile.arrayBuffer()
  const bytes = new Uint8Array(buffer)
  const view = new DataView(buffer)
  const eocdOffset = findZipEndOfCentralDirectory(view)
  if (eocdOffset < 0) {
    throw new Error(t('admin.accounts.dataImportZipFailed'))
  }

  const entryCount = readUint16(view, eocdOffset + 10)
  let offset = readUint32(view, eocdOffset + 16)
  const extracted: File[] = []

  for (let index = 0; index < entryCount; index++) {
    if (
      offset + 46 > view.byteLength ||
      readUint32(view, offset) !== 0x02014b50
    ) {
      throw new Error(t('admin.accounts.dataImportZipFailed'))
    }

    const flags = readUint16(view, offset + 8)
    const method = readUint16(view, offset + 10)
    const compressedSize = readUint32(view, offset + 20)
    const nameLength = readUint16(view, offset + 28)
    const extraLength = readUint16(view, offset + 30)
    const commentLength = readUint16(view, offset + 32)
    const localHeaderOffset = readUint32(view, offset + 42)
    const nameStart = offset + 46
    const name = decodeZipName(bytes.slice(nameStart, nameStart + nameLength))
    offset = nameStart + nameLength + extraLength + commentLength

    if (!isImportableDataFileName(name) || name.endsWith('/')) {
      continue
    }
    if ((flags & 0x1) !== 0) {
      throw new Error(t('admin.accounts.dataImportZipEncrypted'))
    }
    if (
      localHeaderOffset + 30 > view.byteLength ||
      readUint32(view, localHeaderOffset) !== 0x04034b50
    ) {
      throw new Error(t('admin.accounts.dataImportZipFailed'))
    }

    const localNameLength = readUint16(view, localHeaderOffset + 26)
    const localExtraLength = readUint16(view, localHeaderOffset + 28)
    const dataStart =
      localHeaderOffset + 30 + localNameLength + localExtraLength
    const compressedData = bytes.slice(dataStart, dataStart + compressedSize)
    let content: Uint8Array
    if (method === 0) {
      content = compressedData
    } else if (method === 8) {
      content = await inflateRaw(compressedData)
    } else {
      throw new Error(t('admin.accounts.dataImportZipUnsupportedCompression'))
    }

    const pathParts = name.split('/').filter(Boolean)
    const fileName = pathParts[pathParts.length - 1] || name
    extracted.push(
      new File([content], fileName, {
        type: fileName.toLowerCase().endsWith('.json')
          ? 'application/json'
          : 'text/plain'
      })
    )
  }

  return extracted
}

const emptyResult = (): ImportResult => ({
  proxy_created: 0,
  proxy_reused: 0,
  proxy_failed: 0,
  account_created: 0,
  account_failed: 0,
  account_skipped: 0,
  errors: []
})

const mergeResult = (target: ImportResult, source: ImportResult) => {
  target.proxy_created += source.proxy_created
  target.proxy_reused += source.proxy_reused
  target.proxy_failed += source.proxy_failed
  target.account_created += source.account_created
  target.account_failed += source.account_failed
  target.account_skipped =
    (target.account_skipped || 0) + (source.account_skipped || 0)
  target.credential_import = Boolean(
    target.credential_import || source.credential_import
  )
  target.errors = [...(target.errors || []), ...(source.errors || [])]
}

const accountDefaultsPayload = () => {
  if (!overrideDefaults.value) return undefined
  return {
    concurrency: Math.max(0, Number(defaults.concurrency) || 0),
    priority: Math.max(0, Number(defaults.priority) || 0),
    rate_multiplier: Math.max(0, Number(defaults.rate_multiplier) || 0),
    auto_pause_on_expired: defaults.auto_pause_on_expired
  }
}

const selectedImportGroupIds = () =>
  selectedGroupIds.value
    .map((id) => Number(id))
    .filter((id) => Number.isInteger(id) && id > 0)

const commonImportOptions = () => {
  const groupIds = selectedImportGroupIds()
  return {
    skip_default_group_bind: true,
    ...(groupIds.length > 0 ? { group_ids: groupIds } : {}),
    ...(compatibilityMode.value ? { compatibility_mode: true } : {}),
    ...(overrideDefaults.value
      ? { account_defaults: accountDefaultsPayload() }
      : {})
  }
}

const parseSourceURLs = () => {
  return sourceURLs.value
    .split(/\r?\n/)
    .map((url) => url.trim())
    .filter(Boolean)
}

const importLocalFile = async (
  sourceFile: File,
  groupIds: number[]
): Promise<ImportResult> => {
  const text = await readFileAsText(sourceFile)
  let parsed: AdminDataImportPayload | null = null
  let dataPayload: AdminDataPayload | null = null

  try {
    parsed = JSON.parse(text)
    dataPayload = normalizeDataPayload(parsed)
  } catch (error) {
    if (
      !compatibilityMode.value &&
      sourceFile.name.toLowerCase().endsWith('.json')
    ) {
      throw error
    }
  }

  if (dataPayload && !validateSelectedTargetGroups(dataPayload)) {
    throw new Error('__IKIK_IMPORT_VALIDATION_STOP__')
  }

  if (
    compatibilityMode.value ||
    dataPayload ||
    (parsed && isLikelyCodexImportPayload(parsed))
  ) {
    return await adminAPI.accounts.importData({
      data: parsed ?? text,
      ...commonImportOptions()
    })
  }

  return await importAsCredentialContents([text], groupIds)
}

const handleImport = async () => {
  const remoteURLs = parseSourceURLs()
  if (files.value.length === 0 && remoteURLs.length === 0) {
    appStore.showError(t('admin.accounts.dataImportSelectFile'))
    return
  }

  importing.value = true
  try {
    if (!validateSelectedTargetGroups(null)) {
      return
    }

    const merged = emptyResult()
    const groupIds = selectedImportGroupIds()
    resetImportProgress()
    importProgress.total = remoteURLs.length + files.value.length

    for (const remoteURL of remoteURLs) {
      importProgress.current = remoteURL
      const res = await adminAPI.accounts.importData({
        source_url: remoteURL,
        ...commonImportOptions()
      })
      mergeResult(merged, res)
      importProgress.completed++
    }

    for (const sourceFile of files.value) {
      importProgress.current = sourceFile.name
      const res = await importLocalFile(sourceFile, groupIds)
      mergeResult(merged, res)
      importProgress.completed++
    }

    importProgress.current = ''
    result.value = merged

    const msgParams: Record<string, unknown> = {
      account_created: merged.account_created,
      account_failed: merged.account_failed,
      proxy_created: merged.proxy_created,
      proxy_reused: merged.proxy_reused,
      proxy_failed: merged.proxy_failed
    }
    if (
      merged.account_failed > 0 ||
      merged.proxy_failed > 0 ||
      (merged.account_skipped || 0) > 0
    ) {
      appStore.showWarning(
        t('admin.accounts.dataImportCompletedWithErrors', msgParams)
      )
    } else {
      appStore.showSuccess(t('admin.accounts.dataImportSuccess', msgParams))
      emit('imported')
    }
  } catch (error: any) {
    if (error?.message === '__IKIK_IMPORT_VALIDATION_STOP__') {
      return
    }
    if (error instanceof SyntaxError) {
      appStore.showError(t('admin.accounts.dataImportParseFailed'))
    } else {
      appStore.showError(error?.message || t('admin.accounts.dataImportFailed'))
    }
  } finally {
    importing.value = false
  }
}
</script>
