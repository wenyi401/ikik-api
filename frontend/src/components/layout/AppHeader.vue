<template>
  <header class="app-header">
    <div class="app-header-inner">
      <div class="app-header-leading">
        <button
          @click="toggleMobileSidebar"
          class="btn-ghost btn-icon app-header-icon-button lg:hidden"
          :aria-label="t('nav.openMenu')"
        >
          <Icon name="menu" size="md" />
        </button>

        <div class="app-header-context">
          <h1 class="app-header-title">
            {{ pageTitle }}
          </h1>
        </div>
      </div>

      <div class="app-header-actions">
        <LocaleSwitcher class="app-header-action-item shrink-0" />
        <SubscriptionProgressMini v-if="user" class="app-header-action-item" />
        <AnnouncementBell v-if="user" class="app-header-action-item" />

        <div v-if="user" class="relative" ref="dropdownRef">
          <button
            @click="toggleDropdown"
            class="app-header-user-button"
            :aria-label="t('nav.userMenu')"
            aria-haspopup="menu"
            :aria-expanded="dropdownOpen"
          >
            <div class="app-header-avatar">
              <img
                v-if="avatarUrl"
                :src="avatarUrl"
                :alt="displayName"
                class="h-full w-full object-cover"
              >
              <span v-else>{{ userInitials }}</span>
            </div>
          </button>

          <!-- Dropdown Menu -->
          <transition name="dropdown">
            <div v-if="dropdownOpen" class="dropdown right-0 mt-2 w-60 max-w-[calc(100vw-1rem)] overflow-hidden" role="menu">
              <!-- User Info -->
              <div class="border-b border-[var(--app-border)] px-4 py-3">
                <div class="text-sm font-medium text-[var(--app-text)]">
                  {{ displayName }}
                </div>
                <div class="text-xs text-[var(--app-muted)]">{{ user.email }}</div>
              </div>

              <div class="border-b border-[var(--app-border)] px-4 py-3">
                <div class="flex items-center justify-between gap-3">
                  <span class="text-xs text-[var(--app-muted)]">
                    {{ t('common.balance') }}
                  </span>
                  <span class="text-sm font-semibold text-[var(--app-text)]">
                    ${{ user.balance?.toFixed(2) || '0.00' }}
                  </span>
                </div>
              </div>

              <div class="py-1">
                <router-link to="/profile" @click="closeDropdown" class="dropdown-item">
                  {{ t('nav.profile') }}
                </router-link>

                <router-link to="/keys" @click="closeDropdown" class="dropdown-item">
                  {{ t('nav.apiKeys') }}
                </router-link>

                <a
                  v-if="docUrl"
                  :href="docUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                  @click="closeDropdown"
                  class="dropdown-item"
                >
                  {{ t('nav.docs') }}
                </a>

                <a
                  v-if="authStore.isAdmin"
                  href="https://github.com/wenyi401/ikik-api"
                  target="_blank"
                  rel="noopener noreferrer"
                  @click="closeDropdown"
                  class="dropdown-item"
                >
                  {{ t('nav.github') }}
                </a>

              </div>
              <!-- Contact Support (only show if configured) -->
              <div
                v-if="contactInfo"
                class="border-t border-[var(--app-border)] px-4 py-2.5"
              >
                <div class="text-xs text-[var(--app-muted)]">
                  <span>{{ t('common.contactSupport') }}:</span>
                  <span class="font-medium text-[var(--app-muted-strong)]">{{
                    contactInfo
                  }}</span>
                </div>
              </div>

              <div v-if="showOnboardingButton" class="border-t border-[var(--app-border)] py-1">
                <button @click="handleReplayGuide" class="dropdown-item w-full">
                  {{ $t('onboarding.restartTour') }}
                </button>
              </div>

              <div class="border-t border-[var(--app-border)] py-1">
                <button
                  @click="handleLogout"
                  class="dropdown-item w-full text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                >
                  {{ t('nav.logout') }}
                </button>
              </div>
            </div>
          </transition>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore, useOnboardingStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import Icon from '@/components/icons/Icon.vue'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const onboardingStore = useOnboardingStore()

const user = computed(() => authStore.user)
const dropdownOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const contactInfo = computed(() => appStore.contactInfo)
const docUrl = computed(() => appStore.docUrl)
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')

// 只在标准模式的管理员下显示新手引导按钮
const showOnboardingButton = computed(() => {
  return !authStore.isSimpleMode && user.value?.role === 'admin'
})

const userInitials = computed(() => {
  if (!user.value) return ''
  // Prefer username, fallback to email
  if (user.value.username) {
    return user.value.username.substring(0, 2).toUpperCase()
  }
  if (user.value.email) {
    // Get the part before @ and take first 2 chars
    const localPart = user.value.email.split('@')[0]
    return localPart.substring(0, 2).toUpperCase()
  }
  return ''
})

const displayName = computed(() => {
  if (!user.value) return ''
  return user.value.username || user.value.email?.split('@')[0] || ''
})

const pageTitle = computed(() => {
  // For custom pages, use the menu item's label instead of generic "自定义页面"
  if (route.name === 'CustomPage') {
    const id = route.params.id as string
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const menuItem = publicItems.find((item) => item.id === id)
      ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
    if (menuItem?.label) return menuItem.label
  }
  const titleKey = route.meta.titleKey as string
  if (titleKey) {
    return t(titleKey)
  }
  return (route.meta.title as string) || ''
})

function toggleMobileSidebar() {
  appStore.toggleMobileSidebar()
}

function toggleDropdown() {
  dropdownOpen.value = !dropdownOpen.value
}

function closeDropdown() {
  dropdownOpen.value = false
}

async function handleLogout() {
  closeDropdown()
  try {
    await authStore.logout()
  } catch (error) {
    // Ignore logout errors - still redirect to login
    console.error('Logout error:', error)
  }
  await router.push('/login')
}

function handleReplayGuide() {
  closeDropdown()
  onboardingStore.replay()
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeDropdown()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
</style>
