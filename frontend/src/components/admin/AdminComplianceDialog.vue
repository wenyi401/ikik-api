<template>
  <BaseDialog
    :show="visible"
    :title="copy('title')"
    width="wide"
    :close-on-escape="false"
    :close-on-click-outside="false"
    :show-close-button="false"
    :z-index="80"
    @close="noop"
  >
    <div class="space-y-4 text-left">
      <div class="flex items-start gap-3 rounded-xl bg-[var(--app-surface-muted)] px-4 py-3.5">
        <span class="mt-0.5 flex h-8 w-8 flex-none items-center justify-center rounded-full bg-[var(--app-text)] text-[var(--app-surface)]">
          <Icon name="exclamationTriangle" size="sm" />
        </span>
        <div class="min-w-0 space-y-1">
          <p class="text-sm font-semibold text-[var(--app-text)]">{{ copy('blockingNotice') }}</p>
          <p class="text-sm leading-6 text-[var(--app-muted)]">{{ copy('riskNotice') }}</p>
        </div>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--app-border)] pb-3 text-sm">
        <span class="text-[var(--app-muted)]">
          {{ copy('version') }}
          <strong class="ml-1 font-medium text-[var(--app-text)]">{{ documentVersion }}</strong>
        </span>
        <a
          :href="documentUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 font-medium text-[var(--app-text)] hover:opacity-70"
        >
          {{ copy('openDocument') }}
          <Icon name="externalLink" size="sm" />
        </a>
      </div>

      <section class="max-h-[38vh] min-h-[220px] overflow-y-auto rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] px-4 py-4 sm:px-5">
        <div class="legal-document-content" v-html="renderedDocument"></div>
      </section>

      <p class="text-xs leading-5 text-[var(--app-muted)]">
        {{ copy('documentSource') }}
      </p>

      <div class="space-y-2.5 border-t border-[var(--app-border)] pt-4">
        <label for="admin-compliance-phrase" class="block text-sm font-semibold text-[var(--app-text)]">
          {{ copy('inputLabel') }}
        </label>
        <div class="select-all rounded-lg bg-[var(--app-surface-muted)] px-3 py-2.5 text-sm leading-6 text-[var(--app-text)]">
          {{ expectedPhrase }}
        </div>
        <Input
          id="admin-compliance-phrase"
          v-model="typedPhrase"
          :placeholder="copy('inputPlaceholder')"
          autocomplete="off"
          :disabled="complianceStore.submitting"
          :error="inputError"
          @enter="submit"
        />
      </div>

      <p class="text-xs leading-5 text-[var(--app-muted)]">
        {{ copy('legalNote') }}
      </p>
    </div>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <button
          type="button"
          class="btn btn-secondary w-full sm:w-auto"
          :disabled="complianceStore.submitting"
          @click="logout"
        >
          {{ copy('logout') }}
        </button>
        <button
          type="button"
          class="btn btn-primary w-full sm:w-auto"
          :disabled="!canSubmit || complianceStore.submitting"
          @click="submit"
        >
          <span v-if="complianceStore.submitting">{{ submittingText }}</span>
          <span v-else>{{ copy('accept') }}</span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Input from '@/components/common/Input.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAdminComplianceStore, useAppStore, useAuthStore } from '@/stores'
import {
  resolveAdminComplianceCopy,
  type AdminComplianceCopyKey,
} from './adminComplianceCopy'
import zhDocument from '../../../../docs/legal/admin-compliance.zh.md?raw'
import enDocument from '../../../../docs/legal/admin-compliance.en.md?raw'

const { t, te, locale } = useI18n()
const complianceStore = useAdminComplianceStore()
const authStore = useAuthStore()
const appStore = useAppStore()
const typedPhrase = ref('')
const attemptedSubmit = ref(false)

marked.setOptions({
  breaks: true,
  gfm: true,
})

const visible = computed(() => authStore.isAuthenticated && authStore.isAdmin && complianceStore.shouldShow)
const expectedPhrase = computed(() => complianceStore.expectedPhrase)
const canSubmit = computed(() => typedPhrase.value.trim() === expectedPhrase.value)
const isChinese = computed(() => locale.value.toLowerCase().startsWith('zh'))
const currentDocument = computed(() => isChinese.value ? zhDocument : enDocument)
const documentVersion = computed(() => complianceStore.status?.version || 'v2026.07.18')
const submittingText = computed(() => {
  return te('common.submitting') ? t('common.submitting') : (isChinese.value ? '提交中...' : 'Submitting...')
})
const documentUrl = computed(() => {
  if (isChinese.value) {
    return complianceStore.status?.document_url_zh || 'https://github.com/wenyi401/ikik-api/blob/main/docs/legal/admin-compliance.zh.md'
  }
  return complianceStore.status?.document_url_en || 'https://github.com/wenyi401/ikik-api/blob/main/docs/legal/admin-compliance.en.md'
})
const inputError = computed(() => {
  if (!attemptedSubmit.value || canSubmit.value) {
    return ''
  }
  return copy('inputMismatch')
})
const renderedDocument = computed(() => {
  const html = marked.parse(currentDocument.value) as string
  return DOMPurify.sanitize(html)
})

function copy(key: AdminComplianceCopyKey): string {
  const path = `adminCompliance.${key}`
  return resolveAdminComplianceCopy(locale.value, key, te(path) ? t(path) : undefined)
}

watch(expectedPhrase, () => {
  typedPhrase.value = ''
  attemptedSubmit.value = false
})

watch(visible, (isVisible) => {
  if (isVisible) {
    typedPhrase.value = ''
    attemptedSubmit.value = false
  }
})

function noop(): void {
  // 强制确认弹窗不允许通过关闭按钮绕过。
}

async function submit(): Promise<void> {
  attemptedSubmit.value = true
  if (!canSubmit.value) {
    return
  }

  try {
    const status = await complianceStore.accept(typedPhrase.value.trim())
    if (!status.required) {
      appStore.showSuccess(copy('accepted'))
      typedPhrase.value = ''
      attemptedSubmit.value = false
    }
  } catch (error) {
    const message = (error as { message?: string })?.message || copy('acceptFailed')
    appStore.showError(message)
  }
}

async function logout(): Promise<void> {
  await authStore.logout()
  window.location.href = '/login'
}
</script>

<style scoped>
.legal-document-content {
  line-height: 1.75;
  overflow-wrap: anywhere;
  color: inherit;
}

.legal-document-content :deep(h1) {
  @apply mb-4 text-lg font-semibold;
  color: var(--app-text);
}

.legal-document-content :deep(h2) {
  @apply mb-2 mt-6 text-base font-semibold;
  color: var(--app-text);
}

.legal-document-content :deep(p) {
  @apply mb-4 text-sm leading-6;
  color: var(--app-muted);
}

.legal-document-content :deep(ul),
.legal-document-content :deep(ol) {
  @apply mb-4 pl-5 text-sm leading-6;
  color: var(--app-muted);
}

.legal-document-content :deep(ul) {
  @apply list-disc;
}

.legal-document-content :deep(ol) {
  @apply list-decimal;
}

.legal-document-content :deep(li) {
  @apply mb-1;
}

.legal-document-content :deep(strong) {
  @apply font-semibold;
  color: var(--app-text);
}
</style>
