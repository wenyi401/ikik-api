<template>
  <BaseDialog :show="show" :title="t('userAccounts.proxyPool')" width="wide" @close="emit('close')">
    <div class="space-y-5">
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-stone-200 bg-stone-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-800/70">
        <div class="min-w-0">
          <p class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('userAccounts.proxyPoolLimit', { count: proxies.length, limit: limit }) }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">
            {{ t('userAccounts.proxyPoolHint') }}
          </p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm" @click="resetForm">
          <Icon name="plus" size="sm" class="mr-1.5" />
          {{ t('userAccounts.newProxy') }}
        </button>
      </div>

      <form class="grid grid-cols-1 gap-3 md:grid-cols-6" @submit.prevent="submit">
        <div class="md:col-span-2">
          <label class="input-label">{{ t('admin.proxies.name') }}</label>
          <input v-model="form.name" class="input" type="text" maxlength="100" required />
        </div>
        <div class="md:col-span-1">
          <label class="input-label">{{ t('admin.proxies.protocol') }}</label>
          <select v-model="form.protocol" class="input">
            <option value="http">HTTP</option>
            <option value="https">HTTPS</option>
            <option value="socks5">SOCKS5</option>
            <option value="socks5h">SOCKS5H</option>
          </select>
        </div>
        <div class="md:col-span-3">
          <div class="flex gap-2">
            <div class="min-w-0 flex-1">
              <label class="input-label">{{ t('admin.proxies.host') }}</label>
              <input v-model="form.host" class="input" type="text" required />
            </div>
            <div class="w-28 shrink-0">
              <label class="input-label">{{ t('admin.proxies.port') }}</label>
              <input v-model.number="form.port" class="input" type="number" inputmode="numeric" min="1" max="65535" required />
            </div>
          </div>
        </div>
        <div class="md:col-span-2">
          <label class="input-label">{{ t('admin.proxies.username') }}</label>
          <input v-model="form.username" class="input" type="text" autocomplete="off" />
        </div>
        <div class="md:col-span-2">
          <label class="input-label">{{ t('admin.proxies.password') }}</label>
          <input v-model="form.password" class="input" type="password" autocomplete="new-password" />
        </div>
        <div v-if="editingId" class="md:col-span-1">
          <label class="input-label">{{ t('admin.proxies.status') }}</label>
          <select v-model="form.status" class="input">
            <option value="active">{{ t('common.active') }}</option>
            <option value="inactive">{{ t('common.inactive') }}</option>
          </select>
        </div>
        <div class="flex items-end gap-2 md:col-span-1">
          <button type="submit" class="btn btn-primary w-full" :disabled="saving || (!editingId && proxies.length >= limit)">
            {{ editingId ? t('common.save') : t('common.create') }}
          </button>
        </div>
      </form>

      <div v-if="proxies.length === 0" class="rounded-lg border border-dashed border-gray-200 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-300">
        {{ t('userAccounts.noProxies') }}
      </div>

      <div v-else class="space-y-2">
        <div
          v-for="proxy in proxies"
          :key="proxy.id"
          class="flex flex-col gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between"
        >
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="break-words text-sm font-medium text-gray-900 dark:text-white">{{ proxy.name }}</span>
              <span class="rounded-md bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-200">
                {{ proxy.status }}
              </span>
              <span class="rounded-md bg-stone-100 px-2 py-0.5 text-xs text-stone-700 dark:bg-stone-900/30 dark:text-stone-200">
                {{ t('userAccounts.boundAccounts', { count: proxy.account_count ?? 0 }) }}
              </span>
            </div>
            <p class="mt-1 break-all text-xs text-gray-500 dark:text-dark-300">
              {{ proxy.protocol }}://{{ proxy.host }}:{{ proxy.port }}
            </p>
            <p v-if="testResults[proxy.id]" class="mt-1 break-words text-xs" :class="testResults[proxy.id].success ? 'text-emerald-600 dark:text-emerald-300' : 'text-red-600 dark:text-red-300'">
              {{ testResults[proxy.id].success ? testSuccessLabel(testResults[proxy.id]) : testResults[proxy.id].message }}
            </p>
            <div
              v-if="qualityResults[proxy.id]"
              class="mt-2 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-300"
              :title="qualityResults[proxy.id].summary"
            >
              <span>{{ t('admin.proxies.qualityInline', { grade: qualityResults[proxy.id].grade || '-', score: qualityResults[proxy.id].score ?? '-' }) }}</span>
              <span class="badge" :class="qualityOverallClass(qualityOverallStatus(qualityResults[proxy.id]))">
                {{ qualityOverallLabel(qualityOverallStatus(qualityResults[proxy.id])) }}
              </span>
              <span class="min-w-0 break-words">{{ qualityResults[proxy.id].summary }}</span>
            </div>
          </div>
          <div class="grid grid-cols-2 gap-2 sm:flex sm:flex-wrap sm:items-center sm:justify-end">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="testingIds.has(proxy.id)" @click="test(proxy.id)">
              <Icon name="play" size="sm" class="mr-1.5" />
              {{ t('admin.proxies.testConnection') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="qualityCheckingIds.has(proxy.id)" @click="checkQuality(proxy.id)">
              <Icon name="shield" size="sm" class="mr-1.5" :class="qualityCheckingIds.has(proxy.id) ? 'animate-pulse' : ''" />
              {{ t('admin.proxies.qualityCheck') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" @click="edit(proxy)">
              <Icon name="edit" size="sm" class="mr-1.5" />
              {{ t('common.edit') }}
            </button>
            <button type="button" class="btn btn-danger btn-sm" :disabled="deletingId === proxy.id || (proxy.account_count ?? 0) > 0" @click="remove(proxy.id)">
              <Icon name="trash" size="sm" class="mr-1.5" />
              {{ t('common.delete') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { accountsAPI, type ProxyTestResult } from '@/api/accounts'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Proxy, ProxyProtocol, ProxyQualityCheckResult } from '@/types'

const props = defineProps<{
  show: boolean
  proxies: Proxy[]
  limit: number
}>()

const emit = defineEmits<{
  close: []
  changed: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const editingId = ref<number | null>(null)
const saving = ref(false)
const deletingId = ref<number | null>(null)
const testingIds = reactive(new Set<number>())
const qualityCheckingIds = reactive(new Set<number>())
const testResults = reactive<Record<number, ProxyTestResult>>({})
const qualityResults = reactive<Record<number, ProxyQualityCheckResult>>({})

const form = reactive({
  name: '',
  protocol: 'socks5' as ProxyProtocol,
  host: '',
  port: 443,
  username: '',
  password: '',
  status: 'active' as 'active' | 'inactive'
})

function resetForm(): void {
  editingId.value = null
  form.name = ''
  form.protocol = 'socks5'
  form.host = ''
  form.port = 443
  form.username = ''
  form.password = ''
  form.status = 'active'
}

function edit(proxy: Proxy): void {
  editingId.value = proxy.id
  form.name = proxy.name
  form.protocol = proxy.protocol
  form.host = proxy.host
  form.port = proxy.port
  form.username = proxy.username || ''
  form.password = ''
  form.status = proxy.status === 'active' ? 'active' : 'inactive'
}

async function submit(): Promise<void> {
  if (saving.value) return
  if (!editingId.value && props.proxies.length >= props.limit) {
    appStore.showError(t('userAccounts.proxyPoolLimitReached', { limit: props.limit }))
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      const payload = {
        name: form.name.trim(),
        protocol: form.protocol,
        host: form.host.trim(),
        port: Number(form.port),
        username: form.username.trim() || null,
        status: form.status
      }
      if (form.password.trim()) {
        await accountsAPI.updateProxy(editingId.value, { ...payload, password: form.password.trim() })
      } else {
        await accountsAPI.updateProxy(editingId.value, payload)
      }
      appStore.showSuccess(t('userAccounts.proxyUpdated'))
    } else {
      await accountsAPI.createProxy({
        name: form.name.trim(),
        protocol: form.protocol,
        host: form.host.trim(),
        port: Number(form.port),
        username: form.username.trim() || null,
        password: form.password.trim() || null
      })
      appStore.showSuccess(t('userAccounts.proxyCreated'))
    }
    resetForm()
    emit('changed')
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || error?.message || t('common.error'))
  } finally {
    saving.value = false
  }
}

async function remove(id: number): Promise<void> {
  if (deletingId.value) return
  deletingId.value = id
  try {
    await accountsAPI.deleteProxy(id)
    appStore.showSuccess(t('userAccounts.proxyDeleted'))
    if (editingId.value === id) resetForm()
    emit('changed')
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || error?.message || t('common.error'))
  } finally {
    deletingId.value = null
  }
}

async function test(id: number): Promise<void> {
  if (testingIds.has(id)) return
  testingIds.add(id)
  try {
    testResults[id] = await accountsAPI.testProxy(id)
  } catch (error: any) {
    testResults[id] = {
      success: false,
      message: error?.response?.data?.message || error?.message || 'Test failed'
    }
  } finally {
    testingIds.delete(id)
  }
}

async function checkQuality(id: number): Promise<void> {
  if (qualityCheckingIds.has(id)) return
  qualityCheckingIds.add(id)
  try {
    const result = await accountsAPI.checkProxyQuality(id)
    qualityResults[id] = result
    const baseStep = result.items.find((item) => item.target === 'base_connectivity')
    if (baseStep?.status === 'pass') {
      testResults[id] = {
        success: true,
        message: result.summary,
        latency_ms: result.base_latency_ms,
        ip_address: result.exit_ip,
        country: result.country,
        country_code: result.country_code
      }
    }
    appStore.showSuccess(t('admin.proxies.qualityCheckDone', { score: result.score, grade: result.grade }))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || error?.message || t('admin.proxies.qualityCheckFailed'))
  } finally {
    qualityCheckingIds.delete(id)
  }
}

function testSuccessLabel(result: ProxyTestResult): string {
  const parts = [result.country, result.ip_address, result.latency_ms ? `${result.latency_ms}ms` : '']
    .filter(Boolean)
  return parts.length > 0 ? parts.join(' / ') : result.message
}

function qualityOverallStatus(result: ProxyQualityCheckResult): Proxy['quality_status'] {
  if (result.challenge_count > 0) return 'challenge'
  if (result.failed_count > 0) return 'failed'
  if (result.warn_count > 0) return 'warn'
  return 'healthy'
}

function qualityOverallClass(status?: Proxy['quality_status']): string {
  if (status === 'healthy') return 'badge-success'
  if (status === 'warn') return 'badge-warning'
  if (status === 'challenge') return 'badge-danger'
  return 'badge-danger'
}

function qualityOverallLabel(status?: Proxy['quality_status']): string {
  if (status === 'healthy') return t('admin.proxies.qualityStatusHealthy')
  if (status === 'warn') return t('admin.proxies.qualityStatusWarn')
  if (status === 'challenge') return t('admin.proxies.qualityStatusChallenge')
  return t('admin.proxies.qualityStatusFail')
}

watch(
  () => props.show,
  (show) => {
    if (!show) resetForm()
  }
)
</script>
