<template>
  <div class="app-shell" :class="{ 'admin-font': useAdminTypography }">
    <AppSidebar />

    <div
      class="app-workspace"
      :class="[sidebarCollapsed ? 'lg:ml-[64px]' : 'lg:ml-[260px]']"
    >
      <AppHeader />

      <main class="app-main">
        <slot />
      </main>
    </div>

    <OnboardingModeDialog
      :show="showModeDialog"
      :saving="savingMode"
      @select="selectOnboardingMode"
    />
    <BeginnerMissionDock v-if="showBeginnerJourney" />
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import type { OnboardingMode } from '@/types'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import OnboardingModeDialog from '@/components/Guide/OnboardingModeDialog.vue'
import BeginnerMissionDock from '@/components/Guide/BeginnerMissionDock.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const route = useRoute()
const { t } = useI18n()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
const useAdminTypography = computed(() => isAdmin.value || route.path.startsWith('/admin'))
const savingMode = ref(false)
const showModeDialog = computed(() => (
  Boolean(authStore.user) && !isAdmin.value && (authStore.user?.onboarding_mode ?? 'unset') === 'unset'
))
const showBeginnerJourney = computed(() => (
  !isAdmin.value && authStore.user?.onboarding_mode === 'beginner'
))

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: false
})

const onboardingStore = useOnboardingStore()

watch(
  () => authStore.user?.id,
  (userID) => {
    if (userID) onboardingStore.initializeJourney(userID)
  },
  { immediate: true }
)

async function selectOnboardingMode(mode: Exclude<OnboardingMode, 'unset'>): Promise<void> {
  if (savingMode.value) return
  savingMode.value = true
  try {
    const user = await authStore.updateOnboardingMode(mode)
    onboardingStore.initializeJourney(user.id)
    onboardingStore.setMissionPanelOpen(mode === 'beginner')
  } catch {
    appStore.showError(t('onboarding.mode.saveFailed'))
  } finally {
    savingMode.value = false
  }
}

onMounted(() => {
  onboardingStore.setReplayCallback(() => {
    if (showBeginnerJourney.value) {
      onboardingStore.setMissionPanelOpen(true)
      return
    }
    replayTour()
  })
})

defineExpose({ replayTour })
</script>
