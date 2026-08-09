<template>
  <section class="card min-w-0 overflow-hidden" data-testid="developer-tokens-card">
    <div class="flex flex-col gap-4 border-b border-gray-100 px-4 py-4 dark:border-dark-700 sm:flex-row sm:items-start sm:justify-between sm:px-6">
      <div class="flex min-w-0 items-start gap-3">
        <div class="rounded-lg bg-emerald-50 p-2.5 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300">
          <Icon name="key" size="lg" />
        </div>
        <div class="min-w-0">
          <h2 class="text-lg font-medium text-gray-900 dark:text-white">
            {{ t('profile.developerTokens.title') }}
          </h2>
          <p class="mt-1 text-sm leading-5 text-gray-500 dark:text-gray-400">
            {{ t('profile.developerTokens.description') }}
          </p>
        </div>
      </div>
      <button class="btn btn-primary btn-sm w-full sm:w-auto" type="button" @click="openCreateDialog">
        <Icon name="plus" size="sm" />
        <span>{{ t('profile.developerTokens.create') }}</span>
      </button>
    </div>

    <div class="px-4 py-5 sm:px-6">
      <div v-if="loading" class="space-y-3" data-testid="developer-tokens-loading">
        <div v-for="index in 2" :key="index" class="h-20 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
      </div>

      <div
        v-else-if="tokens.length === 0"
        class="flex min-h-28 flex-col items-center justify-center border border-dashed border-gray-200 px-5 py-8 text-center dark:border-dark-600"
      >
        <Icon name="shield" size="lg" class="text-gray-400" />
        <p class="mt-3 text-sm font-medium text-gray-700 dark:text-gray-200">
          {{ t('profile.developerTokens.empty') }}
        </p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('profile.developerTokens.emptyHint') }}
        </p>
      </div>

      <div v-else class="divide-y divide-gray-100 border-y border-gray-100 dark:divide-dark-700 dark:border-dark-700">
        <div
          v-for="token in tokens"
          :key="token.id"
          class="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:justify-between"
        >
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="break-words text-sm font-semibold text-gray-900 dark:text-white">{{ token.name }}</span>
              <span
                class="rounded px-2 py-0.5 text-xs font-medium"
                :class="tokenState(token) === 'active'
                  ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
                  : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
              >
                {{ t(`profile.developerTokens.status.${tokenState(token)}`) }}
              </span>
              <code class="text-xs text-gray-500 dark:text-gray-400">{{ token.token_prefix }}...</code>
            </div>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <span
                v-for="scope in token.scopes"
                :key="scope"
                class="rounded border border-gray-200 px-2 py-0.5 text-xs text-gray-600 dark:border-dark-600 dark:text-gray-300"
              >
                {{ t(`profile.developerTokens.scopes.${scopeKey(scope)}.label`) }}
              </span>
            </div>
            <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
              {{ token.last_used_at
                ? t('profile.developerTokens.lastUsed', { time: formatDateTimeToMinute(token.last_used_at) })
                : t('profile.developerTokens.neverUsed') }}
              <span v-if="token.expires_at"> · {{ t('profile.developerTokens.expires', { time: formatDateTimeToMinute(token.expires_at) }) }}</span>
            </p>
          </div>
          <button
            type="button"
            class="btn btn-ghost btn-sm self-end text-red-600 hover:text-red-700 sm:self-center"
            :title="t('profile.developerTokens.revoke')"
            :aria-label="t('profile.developerTokens.revokeNamed', { name: token.name })"
            @click="tokenToRevoke = token"
          >
            <Icon name="trash" size="sm" />
          </button>
        </div>
      </div>
    </div>
  </section>

  <BaseDialog
    :show="showCreateDialog"
    :title="t('profile.developerTokens.createTitle')"
    width="normal"
    @close="closeCreateDialog"
  >
    <form id="developer-token-create-form" class="space-y-5" @submit.prevent="handleCreate">
      <div>
        <label class="input-label" for="developer-token-name">{{ t('profile.developerTokens.name') }}</label>
        <input
          id="developer-token-name"
          v-model="createForm.name"
          class="input"
          maxlength="100"
          required
          :placeholder="t('profile.developerTokens.namePlaceholder')"
        />
      </div>

      <fieldset>
        <legend class="input-label">{{ t('profile.developerTokens.permissions') }}</legend>
        <div class="space-y-2">
          <label
            v-for="scope in availableScopes"
            :key="scope"
            class="flex cursor-pointer items-start gap-3 border border-gray-200 px-3 py-3 dark:border-dark-600"
          >
            <input v-model="createForm.scopes" class="mt-0.5 h-4 w-4" type="checkbox" :value="scope" />
            <span class="min-w-0">
              <span class="block text-sm font-medium text-gray-800 dark:text-gray-100">
                {{ t(`profile.developerTokens.scopes.${scopeKey(scope)}.label`) }}
              </span>
              <span class="mt-0.5 block text-xs leading-5 text-gray-500 dark:text-gray-400">
                {{ t(`profile.developerTokens.scopes.${scopeKey(scope)}.description`) }}
              </span>
            </span>
          </label>
        </div>
      </fieldset>

      <div>
        <label class="input-label" for="developer-token-expiry">
          {{ t('profile.developerTokens.expiry') }}
          <span class="font-normal text-gray-400">({{ t('common.optional') }})</span>
        </label>
        <input
          id="developer-token-expiry"
          v-model="createForm.expiresAt"
          class="input"
          type="datetime-local"
          :min="minimumExpiry"
        />
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" @click="closeCreateDialog">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="developer-token-create-form"
          :disabled="creating"
        >
          {{ creating ? t('common.creating') : t('common.create') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <BaseDialog
    :show="generatedToken !== null"
    :title="t('profile.developerTokens.createdTitle')"
    width="normal"
    :close-on-escape="false"
    @close="clearGeneratedToken"
  >
    <div v-if="generatedToken" class="space-y-4">
      <div class="border-l-4 border-amber-500 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
        {{ t('profile.developerTokens.oneTimeWarning') }}
      </div>
      <div>
        <label class="input-label">{{ t('profile.developerTokens.plaintextToken') }}</label>
        <div class="flex min-w-0 items-stretch border border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-800">
          <code class="min-w-0 flex-1 break-all px-3 py-3 text-xs text-gray-800 dark:text-gray-100">{{ generatedToken.token }}</code>
          <button
            type="button"
            class="border-l border-gray-200 px-3 text-gray-600 hover:bg-gray-100 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700"
            :title="t('profile.developerTokens.copy')"
            :aria-label="t('profile.developerTokens.copy')"
            @click="copyGeneratedToken"
          >
            <Icon :name="copied ? 'check' : 'copy'" size="sm" />
          </button>
        </div>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end">
        <button class="btn btn-primary" type="button" @click="clearGeneratedToken">
          {{ t('profile.developerTokens.savedToken') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="tokenToRevoke !== null"
    :title="t('profile.developerTokens.revokeTitle')"
    :message="t('profile.developerTokens.revokeConfirm', { name: tokenToRevoke?.name || '' })"
    :confirm-text="t('profile.developerTokens.revoke')"
    danger
    @confirm="handleRevoke"
    @cancel="tokenToRevoke = null"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  DEVELOPER_TOKEN_SCOPES,
  developerTokensAPI,
  type CreatedDeveloperToken,
  type DeveloperToken,
  type DeveloperTokenScope
} from '@/api/developerTokens'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const { copied, copyToClipboard } = useClipboard()

const availableScopes = DEVELOPER_TOKEN_SCOPES
const tokens = ref<DeveloperToken[]>([])
const loading = ref(true)
const creating = ref(false)
const showCreateDialog = ref(false)
const generatedToken = ref<CreatedDeveloperToken | null>(null)
const tokenToRevoke = ref<DeveloperToken | null>(null)

const createForm = reactive<{
  name: string
  scopes: DeveloperTokenScope[]
  expiresAt: string
}>({
  name: '',
  scopes: ['accounts:read', 'accounts:write'],
  expiresAt: ''
})

const minimumExpiry = computed(() => {
  const date = new Date(Date.now() + 60_000)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
})

function scopeKey(scope: DeveloperTokenScope): 'read' | 'write' | 'share' | 'access' {
  return scope.split(':')[1] as 'read' | 'write' | 'share' | 'access'
}

function tokenState(token: DeveloperToken): 'active' | 'expired' | 'revoked' {
  if (token.status !== 'active') return 'revoked'
  if (token.expires_at && new Date(token.expires_at).getTime() <= Date.now()) return 'expired'
  return 'active'
}

function resetCreateForm(): void {
  createForm.name = ''
  createForm.scopes = ['accounts:read', 'accounts:write']
  createForm.expiresAt = ''
}

function openCreateDialog(): void {
  resetCreateForm()
  showCreateDialog.value = true
}

function closeCreateDialog(): void {
  if (creating.value) return
  showCreateDialog.value = false
  resetCreateForm()
}

async function loadTokens(): Promise<void> {
  loading.value = true
  try {
    tokens.value = await developerTokensAPI.list()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('profile.developerTokens.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function handleCreate(): Promise<void> {
  const name = createForm.name.trim()
  if (!name) {
    appStore.showError(t('profile.developerTokens.nameRequired'))
    return
  }
  if (createForm.scopes.length === 0) {
    appStore.showError(t('profile.developerTokens.scopeRequired'))
    return
  }

  creating.value = true
  try {
    const result = await developerTokensAPI.create({
      name,
      scopes: [...createForm.scopes],
      ...(createForm.expiresAt ? { expires_at: new Date(createForm.expiresAt).toISOString() } : {})
    })
    tokens.value = [result.developer_token, ...tokens.value.filter(token => token.id !== result.developer_token.id)]
    showCreateDialog.value = false
    resetCreateForm()
    generatedToken.value = result
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('profile.developerTokens.createFailed')))
  } finally {
    creating.value = false
  }
}

async function copyGeneratedToken(): Promise<void> {
  if (!generatedToken.value) return
  await copyToClipboard(generatedToken.value.token, t('profile.developerTokens.copied'))
}

function clearGeneratedToken(): void {
  generatedToken.value = null
}

async function handleRevoke(): Promise<void> {
  const token = tokenToRevoke.value
  if (!token) return
  try {
    await developerTokensAPI.revoke(token.id)
    tokens.value = tokens.value.filter(item => item.id !== token.id)
    tokenToRevoke.value = null
    appStore.showSuccess(t('profile.developerTokens.revoked'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('profile.developerTokens.revokeFailed')))
  }
}

onMounted(loadTokens)
</script>
