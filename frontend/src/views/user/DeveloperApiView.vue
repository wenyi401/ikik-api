<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <div
        v-if="loading"
        data-testid="developer-api-loading"
        class="card min-h-44 animate-pulse border border-[var(--claude-border)] bg-[var(--claude-surface)] p-6"
      >
        <div class="h-10 w-10 rounded-lg bg-[var(--claude-surface-muted)]" />
        <div class="mt-4 h-5 w-40 rounded bg-[var(--claude-surface-muted)]" />
        <div class="mt-3 h-4 max-w-md rounded bg-[var(--claude-surface-muted)]" />
      </div>

      <ProfileDeveloperTokensCard v-else-if="developerApiEnabled" />

      <section
        v-else
        data-testid="developer-api-access-disabled"
        class="card min-w-0 border border-[var(--claude-border)] bg-[var(--claude-surface)] p-5 shadow-sm md:p-6"
      >
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-[var(--claude-surface-muted)] text-[var(--claude-muted)]">
            <Icon name="key" size="lg" />
          </div>
          <div class="min-w-0">
            <h1 class="text-lg font-semibold text-[var(--claude-text)]">
              {{ t('profile.developerTokens.accessDisabledTitle') }}
            </h1>
            <p class="mt-1 max-w-2xl text-sm leading-6 text-[var(--claude-muted)]">
              {{ t('profile.developerTokens.accessDisabledDescription') }}
            </p>
          </div>
        </div>
      </section>
    </UiPage>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProfileDeveloperTokensCard from '@/components/user/profile/ProfileDeveloperTokensCard.vue'
import { useAuthStore } from '@/stores/auth'
import { UiPage } from '@/ui'

const { t } = useI18n()
const authStore = useAuthStore()
const loading = ref(true)
const developerApiEnabled = computed(() => authStore.user?.developer_api_enabled === true)

onMounted(async () => {
  try {
    await authStore.refreshUser()
  } catch (error) {
    console.error('Failed to refresh developer API access:', error)
  } finally {
    loading.value = false
  }
})
</script>
