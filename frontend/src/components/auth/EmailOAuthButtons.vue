<template>
  <div v-if="hasProviders" class="space-y-4">
    <div v-if="showDivider" class="flex items-center gap-3">
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
      <span class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('auth.oauthOrContinue') }}
      </span>
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
    </div>

    <div class="grid grid-cols-1 gap-3">
      <button
        v-for="provider in visibleProviders"
        :key="provider"
        type="button"
        :disabled="disabled"
        class="btn btn-secondary auth-provider-button relative h-12 w-full justify-center"
        @click="startLogin(provider)"
      >
        <GitHubMark
          v-if="provider === 'github'"
          class="auth-provider-mark h-5 w-5 text-gray-800 dark:text-gray-100"
        />
        <GoogleMark v-else class="auth-provider-mark h-5 w-5" />
        <span class="font-medium">{{ providerLabel(provider) }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import GitHubMark from './GitHubMark.vue'
import GoogleMark from './GoogleMark.vue'
import { resolveAffiliateReferralCode, storeOAuthAffiliateCode } from '@/utils/oauthAffiliate'

type EmailOAuthProvider = 'github' | 'google'
const EMAIL_OAUTH_PENDING_PROVIDER_KEY = 'email_oauth_pending_provider'

const props = withDefaults(defineProps<{
  disabled?: boolean
  affCode?: string
  loginAgreementRevision?: string
  githubEnabled?: boolean
  googleEnabled?: boolean
  showDivider?: boolean
  beforeStart?: () => boolean
}>(), {
  showDivider: true
})

const route = useRoute()
const { t } = useI18n()

const visibleProviders = computed<EmailOAuthProvider[]>(() => {
  const providers: EmailOAuthProvider[] = []
  if (props.googleEnabled) providers.push('google')
  if (props.githubEnabled) providers.push('github')
  return providers
})

const hasProviders = computed(() => visibleProviders.value.length > 0)
function providerLabel(provider: EmailOAuthProvider): string {
  const name = provider === 'github' ? 'GitHub' : 'Google'
  return t('auth.emailOAuth.signIn', { providerName: name })
}

function startLogin(provider: EmailOAuthProvider): void {
  if (props.beforeStart && !props.beforeStart()) {
    return
  }
  const redirectTo = (route.query.redirect as string) || '/dashboard'
  const affiliateCode = resolveAffiliateReferralCode(props.affCode, route.query.aff, route.query.aff_code)
  storeOAuthAffiliateCode(affiliateCode)
  window.sessionStorage.setItem(EMAIL_OAUTH_PENDING_PROVIDER_KEY, provider)
  const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) || '/api/v1'
  const normalized = apiBase.replace(/\/$/, '')
  const params = new URLSearchParams({ redirect: redirectTo })
  if (affiliateCode) {
    params.set('aff_code', affiliateCode)
  }
  if (props.loginAgreementRevision?.trim()) {
    params.set('login_agreement_revision', props.loginAgreementRevision.trim())
  }
  const startURL = `${normalized}/auth/oauth/${provider}/start?${params.toString()}`
  window.location.href = startURL
}
</script>

<style scoped>
.auth-provider-button {
  padding-inline: 3rem;
}

.auth-provider-mark {
  position: absolute;
  left: 1rem;
}
</style>
