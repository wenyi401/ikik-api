<template>
  <div class="auth-shell">
    <header class="auth-topbar">
      <div class="auth-topbar-actions">
        <LocaleSwitcher />
        <UiIconButton
          :label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
          size="sm"
          @click="toggleTheme"
        >
          <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
        </UiIconButton>
      </div>
    </header>

    <main class="auth-main">
      <section class="auth-content">
        <router-link to="/home" class="auth-brand" :aria-label="siteName">
          <span class="auth-logo">
            <img
              v-if="settingsLoaded"
              :src="siteLogo || '/logo.svg'"
              alt=""
              class="h-full w-full object-contain"
            />
          </span>
          <span class="auth-brand-name">{{ siteName }}</span>
        </router-link>

        <div class="auth-card">
          <slot />
        </div>

        <div v-if="$slots.footer" class="auth-footer">
          <slot name="footer" />
        </div>
      </section>
    </main>

    <footer class="auth-copyright">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { UiIconButton } from '@/ui'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'ikik-api')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = computed(() => new Date().getFullYear())
const isDark = ref(document.documentElement.classList.contains('dark'))

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

onMounted(() => {
  void appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-shell {
  display: grid;
  min-height: 100dvh;
  grid-template-rows: auto 1fr auto;
  overflow-x: hidden;
  background: var(--ui-surface);
  color: var(--ui-text);
}

.auth-topbar {
  display: flex;
  min-width: 0;
  height: 4rem;
  align-items: center;
  justify-content: flex-end;
  gap: 1rem;
  padding-inline: clamp(1rem, 3vw, 2rem);
}

.auth-brand {
  display: flex;
  min-width: 0;
  width: fit-content;
  align-items: center;
  gap: 0.625rem;
  margin: 0 auto 2.25rem;
  color: var(--ui-text);
  text-decoration: none;
}

.auth-logo {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: var(--ui-radius-md);
}

.auth-brand-name {
  overflow: hidden;
  font-size: 1rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.auth-topbar-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.25rem;
}

.auth-main {
  display: flex;
  min-width: 0;
  width: 100%;
  box-sizing: border-box;
  align-items: flex-start;
  justify-content: center;
  padding: clamp(2.5rem, 8vh, 6rem) 1rem 3rem;
}

.auth-content {
  min-width: 0;
  width: 100%;
  max-width: 25rem;
}

.auth-card {
  min-width: 0;
  width: 100%;
}

.auth-footer {
  margin-top: 2rem;
  color: var(--ui-text-tertiary);
  text-align: center;
  font-size: 0.875rem;
}

.auth-copyright {
  padding: 1rem;
  color: var(--ui-text-tertiary);
  text-align: center;
  font-size: 0.75rem;
}

.auth-card :deep(h2) {
  color: var(--ui-text);
  font-size: 2rem;
  font-weight: 600;
  line-height: 1.2;
}

.auth-card :deep(p),
.auth-card :deep(.text-gray-500),
.auth-card :deep(.dark\:text-dark-400) {
  color: var(--ui-text-tertiary);
}

.auth-card :deep(.input-label) {
  color: var(--ui-text-secondary);
}

.auth-card :deep(.input) {
  min-height: 3.25rem;
  border-radius: 1.125rem;
  border-color: var(--ui-border-strong);
  background: var(--ui-bg);
  color: var(--ui-text);
}

.auth-card :deep(.input:focus) {
  border-color: var(--ui-text-secondary);
  box-shadow: 0 0 0 3px var(--ui-focus);
}

.auth-card :deep(.btn-primary) {
  min-height: 3.25rem;
  border-radius: 1.125rem;
  background: var(--ui-brand);
  color: var(--ui-brand-contrast);
}

.auth-card :deep(.btn-primary:hover) {
  background: var(--ui-brand-hover);
}

.auth-card :deep(.text-primary-600),
.auth-footer :deep(.text-primary-600),
.auth-card :deep(.dark\:text-primary-400),
.auth-footer :deep(.dark\:text-primary-400) {
  color: var(--ui-text);
}

.auth-card :deep(.hover\:text-primary-500:hover),
.auth-footer :deep(.hover\:text-primary-500:hover),
.auth-card :deep(.dark\:hover\:text-primary-300:hover),
.auth-footer :deep(.dark\:hover\:text-primary-300:hover) {
  color: var(--ui-text-secondary);
}

.auth-card :deep(.btn-secondary) {
  min-height: 3.25rem;
  border-radius: 1.125rem;
  border-color: var(--ui-border-strong);
  background: var(--ui-surface);
}

.auth-card :deep(.btn-secondary:hover) {
  background: var(--ui-surface-hover);
}

.auth-card :deep(.auth-password-toggle) {
  color: var(--ui-text-tertiary);
}

.auth-card :deep(.auth-password-toggle:hover) {
  color: var(--ui-text);
}

@media (max-width: 640px) {
  .auth-topbar {
    height: 3.5rem;
    padding-inline: 0.875rem;
  }

  .auth-main {
    align-items: flex-start;
    padding: 2rem 1.5rem;
  }

  .auth-brand {
    margin-bottom: 1.75rem;
  }

  .auth-card :deep(h2) {
    font-size: 1.75rem;
  }
}
</style>
