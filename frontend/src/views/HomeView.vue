<template>
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="sanitizedHomeContent"></div>
  </div>

  <div
    v-else
    ref="pageRef"
    class="home-page"
    @pointermove="handlePointerMove"
    @pointerleave="resetPointer"
  >
    <div class="home-noise" aria-hidden="true"></div>

    <header class="home-shell home-nav">
      <router-link class="brand" to="/home" aria-label="Home">
        <span class="brand-mark" aria-hidden="true">
          <img v-if="siteLogo" :src="siteLogo" alt="" />
          <svg v-else viewBox="0 0 100 100" fill="none">
            <path d="M 33 41 L 59 41 L 52 56 L 33 56 Z" fill="currentColor" opacity=".72" />
            <path d="M 33 56 L 52 56 L 43 83 L 33 83 Z" fill="currentColor" opacity=".48" />
            <rect x="16" y="19" width="20" height="64" rx="9" fill="currentColor" opacity=".78" />
            <path
              d="M 71 29 L 87 29 Q 91.5 29 89 34 L 61 90 Q 58.5 95 53.5 95 L 38 95 Q 33.5 95 36 90 L 64 34 Q 66.5 29 71 29 Z"
              fill="currentColor"
            />
          </svg>
        </span>
        <span>{{ siteName }}</span>
      </router-link>

      <nav class="home-nav-links" aria-label="Home navigation">
        <router-link to="/home">{{ t('home.nav.home') }}</router-link>
        <router-link to="/key-usage">{{ t('home.nav.usage') }}</router-link>
      </nav>

      <div class="home-actions">
        <LocaleSwitcher class="home-locale" />
        <button
          type="button"
          class="icon-action"
          :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          @click="toggleTheme"
        >
          <Icon v-if="isDark" name="sun" size="md" />
          <Icon v-else name="moon" size="md" />
        </button>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="button primary nav-cta"
        >
          <span v-if="isAuthenticated" class="user-dot">{{ userInitial }}</span>
          <span>{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}</span>
        </router-link>
      </div>
    </header>

    <main>
      <section class="home-shell hero-section">
        <div class="hero-copy">
          <span class="eyebrow">
            <span class="eyebrow-line"></span>
            {{ t('home.redesign.hero.eyebrow') }}
          </span>
          <h1>{{ t('home.redesign.hero.title') }}</h1>
          <p>{{ t('home.redesign.hero.lead', { siteName }) }}</p>

          <div class="hero-actions">
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="button primary">
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
            </router-link>
            <router-link class="button secondary" to="/key-usage">
              {{ t('home.hero.viewUsage') }}
            </router-link>
          </div>

          <div class="hero-proof" :aria-label="t('home.redesign.hero.proofAria')">
            <span v-for="item in heroProofItems" :key="item">{{ item }}</span>
          </div>
        </div>

        <div class="hero-product" aria-hidden="true">
          <div class="device-frame">
            <div class="device-topbar">
              <span></span>
              <span></span>
              <span></span>
              <code>{{ publicBaseUrl }}/v1</code>
            </div>
            <div class="device-body">
              <div class="route-map">
                <div class="route-node source">
                  <Icon name="terminal" size="md" />
                  <span>{{ t('home.redesign.product.client') }}</span>
                </div>
                <div class="route-line"></div>
                <div class="route-node gateway">
                  <Icon name="globe" size="lg" />
                  <span>{{ t('home.redesign.product.gateway') }}</span>
                </div>
                <div class="route-line"></div>
                <div class="route-node pool">
                  <Icon name="server" size="md" />
                  <span>{{ t('home.redesign.product.pool') }}</span>
                </div>
              </div>

              <div class="metrics-board">
                <article v-for="metric in metricItems" :key="metric.label" class="metric">
                  <span>{{ metric.label }}</span>
                  <strong>{{ metric.value }}</strong>
                </article>
              </div>

              <div class="flow-panel">
                <div class="flow-panel-head">
                  <span>{{ t('home.redesign.product.apiSurface') }}</span>
                  <b>{{ t('home.redesign.product.compatible') }}</b>
                </div>
                <div class="api-rows">
                  <span v-for="row in endpointRows" :key="row.path" class="api-row">
                    <b>{{ row.method }}</b>
                    <code>{{ row.path }}</code>
                    <em>{{ row.label }}</em>
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="home-shell home-footer">
      <section class="footer-intro">
        <div>
          <h2>{{ t('home.footer.title', { siteName }) }}</h2>
          <p>{{ t('home.footer.description') }}</p>
        </div>
      </section>

      <section class="footer-info-grid" :aria-label="t('home.footer.infoAria')">
        <article v-for="item in footerInfoItems" :key="item.title" class="footer-info-card">
          <span class="footer-info-icon" aria-hidden="true">
            <Icon :name="item.icon" size="md" />
          </span>
          <div>
            <h3>{{ item.title }}</h3>
            <p>{{ item.description }}</p>
          </div>
        </article>
      </section>

      <section class="footer-meta">
        <div class="footer-meta-group">
          <span>{{ t('home.footer.baseUrlLabel') }}</span>
          <code>{{ publicBaseUrl }}</code>
        </div>
        <div class="footer-meta-links">
          <router-link to="/key-usage">{{ t('home.nav.usage') }}</router-link>
          <router-link :to="isAuthenticated ? '/models' : '/login'">{{ t('nav.modelMarket') }}</router-link>
          <router-link :to="isAuthenticated ? '/purchase' : '/login'">{{ t('nav.buySubscription') }}</router-link>
        </div>
      </section>

      <div class="footer-legal">
        <span>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</span>
        <span>{{ t('home.footer.serviceNotice') }}</span>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { getPublicTodayStats } from '@/api/usage'

type FooterInfoIcon = 'bolt' | 'creditCard' | 'shield'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const pageRef = ref<HTMLElement | null>(null)

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'ikik-api')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const sanitizedHomeContent = computed(() => DOMPurify.sanitize(homeContent.value))
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => authStore.user?.email?.charAt(0).toUpperCase() || '')
const publicBaseUrl = computed(() => {
  if (typeof window === 'undefined') return 'https://ikik.net'
  return window.location.origin
})

const publicTodayStats = ref<{
  today_requests: number
  today_tokens: number
  success_rate: number | null
  average_duration_ms: number | null
  average_first_token_ms: number | null
} | null>(null)

const todayRequestsText = computed(() => {
  if (!publicTodayStats.value) return '--'
  return formatInteger(publicTodayStats.value.today_requests)
})

const todayTokensText = computed(() => {
  if (!publicTodayStats.value) return '--'
  return formatCompactNumber(publicTodayStats.value.today_tokens)
})

const successRateText = computed(() => {
  const successRate = publicTodayStats.value?.success_rate
  if (typeof successRate !== 'number' || !Number.isFinite(successRate)) return '--'
  return `${successRate.toFixed(1).replace(/\.0$/, '')}%`
})

const averageLatencyText = computed(() => {
  const averageFirstTokenMs = publicTodayStats.value?.average_first_token_ms
  if (typeof averageFirstTokenMs !== 'number' || !Number.isFinite(averageFirstTokenMs)) return '--'
  const seconds = averageFirstTokenMs / 1000
  return `${seconds.toFixed(seconds >= 10 ? 1 : 2).replace(/\.?0+$/, '')}s`
})

const heroProofItems = computed(() => [
  t('home.redesign.hero.proof.openai'),
  t('home.redesign.hero.proof.routing'),
  t('home.redesign.hero.proof.billing')
])

const metricItems = computed(() => [
  { label: t('home.console.todayRequests'), value: todayRequestsText.value },
  { label: t('home.console.todayTokens'), value: todayTokensText.value },
  { label: t('home.console.successRate'), value: successRateText.value },
  { label: t('home.console.avgLatency'), value: averageLatencyText.value }
])

const endpointRows = computed(() => [
  { method: 'POST', path: '/v1/chat/completions', label: 'Chat' },
  { method: 'POST', path: '/v1/responses', label: 'Responses' },
  { method: 'GET', path: '/v1/models', label: 'Models' },
  { method: 'GET', path: '/key-usage', label: 'Usage' }
])

const footerInfoItems = computed<Array<{
  icon: FooterInfoIcon
  title: string
  description: string
}>>(() => [
  {
    icon: 'bolt',
    title: t('home.footer.cards.access.title'),
    description: t('home.footer.cards.access.desc')
  },
  {
    icon: 'creditCard',
    title: t('home.footer.cards.billing.title'),
    description: t('home.footer.cards.billing.desc')
  },
  {
    icon: 'shield',
    title: t('home.footer.cards.reliability.title'),
    description: t('home.footer.cards.reliability.desc')
  }
])

const currentYear = computed(() => new Date().getFullYear())

function handlePointerMove(event: PointerEvent) {
  const target = pageRef.value
  if (!target) return
  const rect = target.getBoundingClientRect()
  const x = ((event.clientX - rect.left) / rect.width - 0.5) * 2
  const y = ((event.clientY - rect.top) / Math.max(rect.height, window.innerHeight) - 0.5) * 2
  target.style.setProperty('--pointer-x', x.toFixed(3))
  target.style.setProperty('--pointer-y', y.toFixed(3))
}

function resetPointer() {
  const target = pageRef.value
  if (!target) return
  target.style.setProperty('--pointer-x', '0')
  target.style.setProperty('--pointer-y', '0')
}

function formatInteger(value: number): string {
  if (!Number.isFinite(value)) return '--'
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 0 }).format(value)
}

function formatCompactNumber(value: number): string {
  if (!Number.isFinite(value)) return '--'
  const absValue = Math.abs(value)
  const units = [
    { value: 1_000_000_000, suffix: 'B' },
    { value: 1_000_000, suffix: 'M' },
    { value: 1_000, suffix: 'K' }
  ]
  const unit = units.find((item) => absValue >= item.value)
  if (!unit) return formatInteger(value)
  return `${(value / unit.value).toFixed(2).replace(/\.?0+$/, '')}${unit.suffix}`
}

async function fetchPublicTodayStats() {
  try {
    publicTodayStats.value = await getPublicTodayStats()
  } catch (error) {
    publicTodayStats.value = null
    console.error('Failed to fetch public today usage stats:', error)
  }
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  fetchPublicTodayStats()
})
</script>

<style scoped>
.home-page {
  --bg: #f7f3ea;
  --paper: #fffaf3;
  --paper-2: #f0e8dc;
  --text: #26211c;
  --muted: #746a60;
  --subtle: #998d80;
  --line: rgba(58, 43, 33, 0.12);
  --line-strong: rgba(58, 43, 33, 0.2);
  --accent: #c66f4a;
  --accent-soft: rgba(198, 111, 74, 0.12);
  --ink: #30241d;
  --shadow: 0 28px 80px rgba(70, 49, 35, 0.13);
  --serif: var(--font-home-display);
  --pointer-x: 0;
  --pointer-y: 0;

  position: relative;
  min-height: 100vh;
  overflow: clip;
  background:
    linear-gradient(180deg, rgba(255, 250, 243, 0.96), rgba(247, 243, 234, 0.98) 48%, #efe7dc),
    linear-gradient(115deg, transparent 0 58%, rgba(198, 111, 74, 0.08) 58% 58.2%, transparent 58.2%);
  color: var(--text);
  font-family: var(--font-app);
}

.home-page::before {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(58, 43, 33, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(58, 43, 33, 0.05) 1px, transparent 1px);
  background-size: 64px 64px;
  mask-image: linear-gradient(180deg, rgba(0, 0, 0, 0.62), transparent 72%);
  content: "";
  pointer-events: none;
}

.home-page::after {
  position: absolute;
  top: 96px;
  right: 8vw;
  width: min(52vw, 760px);
  height: min(52vw, 760px);
  border: 1px solid rgba(58, 43, 33, 0.08);
  border-radius: 42% 58% 52% 48%;
  background:
    linear-gradient(135deg, rgba(255, 250, 243, 0.42), rgba(198, 111, 74, 0.08)),
    repeating-linear-gradient(90deg, rgba(58, 43, 33, 0.04) 0 1px, transparent 1px 22px);
  content: "";
  filter: blur(0.2px);
  opacity: 0.72;
  pointer-events: none;
  transform:
    translate3d(calc(var(--pointer-x) * 10px), calc(var(--pointer-y) * 8px), 0)
    rotate(-8deg);
}

:global(html.dark .home-page) {
  --bg: #171310;
  --paper: #211a16;
  --paper-2: #2a211c;
  --text: #f4efe7;
  --muted: #b8aa9a;
  --subtle: #8f8174;
  --line: rgba(244, 239, 231, 0.12);
  --line-strong: rgba(244, 239, 231, 0.22);
  --accent: #d58b65;
  --accent-soft: rgba(213, 139, 101, 0.16);
  --ink: #f4efe7;
  --shadow: 0 32px 92px rgba(0, 0, 0, 0.34);

  background:
    linear-gradient(180deg, #171310 0%, #211915 48%, #2a201b 100%),
    linear-gradient(115deg, transparent 0 58%, rgba(213, 139, 101, 0.12) 58% 58.2%, transparent 58.2%);
}

:global(html.dark .home-page)::before {
  background-image:
    linear-gradient(rgba(244, 239, 231, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(244, 239, 231, 0.045) 1px, transparent 1px);
}

:global(html.dark .home-page)::after {
  border-color: rgba(244, 239, 231, 0.08);
  background:
    linear-gradient(135deg, rgba(244, 239, 231, 0.08), rgba(213, 139, 101, 0.08)),
    repeating-linear-gradient(90deg, rgba(244, 239, 231, 0.04) 0 1px, transparent 1px 22px);
}

.home-noise {
  position: fixed;
  inset: 0;
  z-index: 0;
  background-image: radial-gradient(rgba(58, 43, 33, 0.14) 0.7px, transparent 0.7px);
  background-size: 4px 4px;
  opacity: 0.12;
  pointer-events: none;
}

.home-shell {
  position: relative;
  z-index: 1;
  width: min(1180px, calc(100% - 44px));
  margin: 0 auto;
}

.home-nav {
  display: flex;
  min-height: 82px;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.brand {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 11px;
  color: var(--text);
  font-size: 1.08rem;
  font-weight: 680;
  letter-spacing: 0;
}

.brand-mark {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: 12px;
  background: color-mix(in srgb, var(--paper) 88%, transparent);
  color: var(--accent);
  box-shadow: 0 12px 28px rgba(70, 49, 35, 0.08);
}

.brand-mark img,
.brand-mark svg {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.home-nav-links {
  display: flex;
  align-items: center;
  gap: 30px;
  color: var(--muted);
  font-size: 0.92rem;
  font-weight: 650;
}

.home-nav-links a {
  transition: color 160ms ease;
}

.home-nav-links a:hover,
.home-nav-links a.router-link-active {
  color: var(--text);
}

.home-actions,
.hero-actions,
.code-tabs {
  display: flex;
  align-items: center;
}

.home-actions {
  gap: 10px;
}

.icon-action {
  display: inline-flex;
  min-width: 42px;
  min-height: 42px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: color-mix(in srgb, var(--paper) 72%, transparent);
  color: var(--muted);
  transition:
    background 160ms ease,
    color 160ms ease,
    transform 160ms ease;
}

.icon-action:hover {
  transform: translateY(-1px);
  background: var(--paper);
  color: var(--text);
}

.button {
  display: inline-flex;
  min-height: 46px;
  align-items: center;
  justify-content: center;
  gap: 9px;
  border: 1px solid transparent;
  border-radius: 999px;
  padding: 0 21px;
  font-size: 0.94rem;
  font-weight: 680;
  line-height: 1;
  white-space: nowrap;
  transition:
    transform 180ms ease,
    box-shadow 180ms ease,
    background 180ms ease,
    border-color 180ms ease;
}

.button:hover {
  transform: translateY(-2px);
}

.button.primary {
  background: var(--ink);
  color: var(--paper);
  box-shadow: 0 14px 32px rgba(48, 36, 29, 0.18);
}

:global(html.dark .home-page .button.primary) {
  background: #f4efe7;
  color: #171310;
  box-shadow: 0 18px 44px rgba(244, 239, 231, 0.14);
}

.button.secondary {
  border-color: var(--line);
  background: color-mix(in srgb, var(--paper) 68%, transparent);
  color: var(--text);
}

.user-dot {
  display: grid;
  width: 22px;
  height: 22px;
  place-items: center;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.16);
  font-size: 0.72rem;
}

.hero-section {
  display: grid;
  min-height: calc(100vh - 82px);
  grid-template-columns: minmax(0, 0.9fr) minmax(420px, 1.1fr);
  align-items: center;
  gap: 60px;
  padding: 42px 0 82px;
}

.hero-copy {
  max-width: 690px;
}

.eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  color: var(--accent);
  font-size: 0.78rem;
  font-weight: 780;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.eyebrow-line {
  width: 34px;
  height: 1px;
  background: var(--accent);
}

.hero-copy h1 {
  max-width: 760px;
  margin: 24px 0 22px;
  font-family: var(--serif);
  font-size: clamp(3.45rem, 8vw, 6.8rem);
  font-weight: 520;
  letter-spacing: 0;
  line-height: 0.98;
  text-wrap: balance;
}

.hero-copy p {
  color: var(--muted);
  font-size: clamp(1rem, 1.7vw, 1.18rem);
  font-weight: 450;
  line-height: 1.85;
}

.hero-copy p {
  max-width: 610px;
  margin: 0 0 32px;
}

.hero-actions {
  flex-wrap: wrap;
  gap: 14px;
}

.hero-proof {
  display: flex;
  flex-wrap: wrap;
  gap: 9px;
  margin-top: 34px;
}

.hero-proof span {
  border: 1px solid var(--line);
  border-radius: 999px;
  background: color-mix(in srgb, var(--paper) 58%, transparent);
  color: var(--muted);
  padding: 8px 11px;
  font-size: 0.8rem;
  font-weight: 650;
}

.hero-product {
  position: relative;
  z-index: 1;
  perspective: 1400px;
}

.device-frame {
  overflow: hidden;
  border: 1px solid var(--line-strong);
  border-radius: 30px;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--paper) 92%, transparent), color-mix(in srgb, var(--paper-2) 78%, transparent)),
    repeating-linear-gradient(90deg, transparent 0 28px, rgba(58, 43, 33, 0.035) 28px 29px);
  box-shadow: var(--shadow);
  transform:
    rotateX(calc(var(--pointer-y) * -2deg + 8deg))
    rotateY(calc(var(--pointer-x) * 3deg - 11deg))
    rotateZ(1.6deg);
  transform-style: preserve-3d;
  transition: transform 260ms ease-out;
}

:global(html.dark .home-page .device-frame) {
  background:
    linear-gradient(135deg, rgba(34, 27, 23, 0.94), rgba(42, 33, 28, 0.86)),
    repeating-linear-gradient(90deg, transparent 0 28px, rgba(244, 239, 231, 0.035) 28px 29px);
}

.device-topbar {
  display: flex;
  align-items: center;
  gap: 8px;
  border-bottom: 1px solid var(--line);
  padding: 16px 18px;
}

.device-topbar span {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--line-strong);
}

.device-topbar code {
  min-width: 0;
  margin-left: auto;
  overflow: hidden;
  color: var(--subtle);
  font-size: 0.78rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-body {
  padding: 28px;
}

.route-map {
  display: grid;
  grid-template-columns: 1fr 54px 1fr 54px 1fr;
  align-items: center;
  gap: 0;
}

.route-node {
  display: grid;
  min-height: 118px;
  place-items: center;
  border: 1px solid var(--line);
  border-radius: 22px;
  background: color-mix(in srgb, var(--paper) 72%, transparent);
  color: var(--text);
  text-align: center;
  transform: translateZ(30px);
}

.route-node svg {
  color: var(--accent);
}

.route-node span {
  margin-top: 8px;
  color: var(--muted);
  font-size: 0.8rem;
  font-weight: 700;
}

.route-line {
  height: 1px;
  background: linear-gradient(90deg, var(--line), var(--accent), var(--line));
}

.metrics-board {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 22px;
}

.metric {
  min-width: 0;
  border: 1px solid var(--line);
  border-radius: 18px;
  background: color-mix(in srgb, var(--paper) 64%, transparent);
  padding: 15px 14px;
}

.metric span {
  display: block;
  overflow: hidden;
  color: var(--subtle);
  font-size: 0.76rem;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.metric strong {
  display: block;
  margin-top: 10px;
  color: var(--text);
  font-size: 1.22rem;
  font-weight: 760;
}

.flow-panel {
  border: 1px solid var(--line);
  border-radius: 24px;
  background: color-mix(in srgb, var(--paper) 76%, transparent);
}

.flow-panel {
  margin-top: 14px;
  padding: 18px;
}

.flow-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.flow-panel-head span {
  color: var(--text);
  font-size: 0.9rem;
  font-weight: 760;
}

.flow-panel-head b {
  border-radius: 999px;
  background: rgba(91, 121, 74, 0.12);
  color: #587447;
  padding: 5px 9px;
  font-size: 0.72rem;
  font-weight: 800;
}

:global(html.dark .home-page .flow-panel-head b) {
  color: #c8d8ad;
}

.api-rows {
  display: grid;
  gap: 10px;
  margin-top: 16px;
}

.api-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 9px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: color-mix(in srgb, var(--paper) 52%, transparent);
  padding: 10px 11px;
  color: var(--muted);
  font-size: 0.78rem;
}

.api-row b {
  min-width: 46px;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent);
  padding: 5px 7px;
  text-align: center;
  font-size: 0.68rem;
  font-weight: 820;
}

.api-row code {
  min-width: 0;
  overflow: hidden;
  color: var(--text);
  font-size: 0.78rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.api-row em {
  color: var(--subtle);
  font-style: normal;
  font-weight: 700;
}

.home-footer {
  display: grid;
  gap: 20px;
  padding: 34px 0 24px;
  color: var(--muted);
  font-size: 0.88rem;
}

.footer-intro {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  border: 1px solid var(--line);
  border-radius: 24px;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--paper) 94%, transparent), color-mix(in srgb, var(--paper-2) 78%, transparent)),
    radial-gradient(circle at 12% 20%, rgba(198, 111, 74, 0.1), transparent 36%);
  padding: 26px;
  box-shadow: 0 18px 44px rgba(70, 49, 35, 0.06);
}

.footer-intro h2 {
  margin: 0;
  color: var(--text);
  font-size: clamp(1.45rem, 2vw, 2.15rem);
  font-weight: 720;
  letter-spacing: 0;
}

.footer-intro p {
  max-width: 720px;
  margin: 10px 0 0;
  color: var(--muted);
  font-size: 0.96rem;
  line-height: 1.7;
}

.footer-info-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.footer-info-card {
  display: flex;
  min-width: 0;
  gap: 14px;
  border: 1px solid var(--line);
  border-radius: 18px;
  background: color-mix(in srgb, var(--paper) 68%, transparent);
  padding: 18px;
}

.footer-info-icon {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  place-items: center;
  border-radius: 12px;
  background: var(--accent-soft);
  color: var(--accent);
}

.footer-info-card h3 {
  margin: 0;
  color: var(--text);
  font-size: 0.98rem;
  font-weight: 680;
}

.footer-info-card p {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 0.84rem;
  line-height: 1.62;
}

.footer-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  border-top: 1px solid var(--line);
  padding-top: 16px;
}

.footer-meta-group {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 10px;
}

.footer-meta-group span {
  color: var(--subtle);
  font-weight: 650;
}

.footer-meta-group code {
  max-width: min(54vw, 520px);
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: color-mix(in srgb, var(--paper) 76%, transparent);
  padding: 7px 11px;
  color: var(--text);
  font-size: 0.82rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.footer-meta-links {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.footer-meta-links a {
  border-radius: 999px;
  padding: 7px 11px;
  color: var(--muted);
  font-size: 0.86rem;
  font-weight: 650;
}

.footer-meta-links a:hover {
  background: var(--accent-soft);
  color: var(--text);
}

.footer-legal {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  color: var(--subtle);
  font-size: 0.82rem;
}

@media (max-width: 1180px) {
  .home-page::after {
    top: 78px;
    right: -9vw;
    width: min(76vw, 720px);
    height: min(76vw, 720px);
    opacity: 0.52;
  }

  .hero-section {
    grid-template-columns: 1fr;
    min-height: auto;
    align-items: start;
    gap: 36px;
    padding: 42px 0 72px;
  }

  .hero-copy {
    max-width: 760px;
  }

  .hero-copy h1 {
    max-width: 720px;
    margin-top: 20px;
    font-size: clamp(3.65rem, 8.2vw, 5.25rem);
    line-height: 1;
  }

  .hero-copy p {
    max-width: 720px;
    margin-bottom: 26px;
    overflow-wrap: anywhere;
  }

  .hero-actions {
    width: min(100%, 440px);
  }

  .hero-actions .button {
    min-width: 0;
  }

  .hero-proof {
    max-width: 100%;
    flex-wrap: wrap;
  }

  .hero-product {
    width: min(760px, 100%);
    margin: 0 auto;
    perspective: none;
  }

  .device-frame {
    transform: none;
  }
}

@media (orientation: portrait) and (min-width: 761px) and (max-width: 1180px) {
  .home-nav {
    min-height: 76px;
  }

  .hero-section {
    gap: 34px;
    padding-top: max(34px, 4.2vh);
  }
}

@media (max-width: 760px) {
  .home-shell {
    width: min(100% - 24px, 1180px);
    max-width: 100%;
  }

  .hero-copy,
  .hero-product,
  .device-frame {
    max-width: 100%;
  }

  .home-page::before {
    background-size: 44px 44px;
    mask-image: linear-gradient(180deg, rgba(0, 0, 0, 0.42), transparent 66%);
  }

  .home-page::after {
    top: 64px;
    right: -34vw;
    width: 104vw;
    height: 104vw;
    opacity: 0.38;
  }

  .home-nav {
    min-height: 64px;
    gap: 10px;
  }

  .brand span:last-child {
    max-width: 126px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .brand-mark {
    width: 34px;
    height: 34px;
    flex-basis: 34px;
    border-radius: 11px;
  }

  .home-actions {
    gap: 8px;
  }

  .icon-action {
    min-width: 38px;
    min-height: 38px;
  }

  .home-nav-links,
  .home-locale,
  .nav-cta {
    display: none;
  }

  .hero-section {
    gap: 24px;
    padding: 16px 0 48px;
  }

  .hero-copy h1 {
    max-width: 10em;
    margin: 15px 0 15px;
    font-size: clamp(2.7rem, 11vw, 3.55rem);
    line-height: 1.03;
  }

  .hero-copy p {
    max-width: 100%;
    margin-bottom: 20px;
    font-size: 0.96rem;
    line-height: 1.68;
    overflow-wrap: anywhere;
  }

  .eyebrow {
    font-size: 0.72rem;
    letter-spacing: 0.08em;
  }

  .eyebrow-line {
    width: 26px;
  }

  .hero-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    width: 100%;
    max-width: none;
    gap: 10px;
  }

  .hero-actions .button {
    width: 100%;
    min-width: 0;
    padding: 0 10px;
    font-size: 0.9rem;
  }

  .hero-proof {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin: 20px 0 0;
  }

  .hero-proof span {
    flex: 0 0 auto;
    padding: 7px 10px;
    font-size: 0.76rem;
  }

  .device-frame {
    border-radius: 22px;
    box-shadow: 0 18px 46px rgba(70, 49, 35, 0.1);
  }

  .device-topbar {
    padding: 12px 14px;
  }

  .route-map {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  .route-node {
    min-height: 78px;
    border-radius: 18px;
  }

  .route-line {
    width: 1px;
    height: 18px;
    justify-self: center;
  }

  .metrics-board {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 9px;
    margin-top: 14px;
  }

  .metric {
    border-radius: 15px;
    padding: 12px;
  }

  .metric strong {
    margin-top: 7px;
    font-size: 1.04rem;
  }

  .device-body {
    padding: 14px;
  }

  .flow-panel {
    border-radius: 18px;
    margin-top: 12px;
    padding: 14px;
  }

  .flow-panel-head {
    gap: 10px;
  }

  .api-rows {
    gap: 8px;
    margin-top: 12px;
  }

  .api-row {
    grid-template-columns: auto minmax(0, 1fr) auto;
    border-radius: 12px;
    padding: 9px;
  }

  .footer-intro {
    align-items: flex-start;
    flex-direction: column;
  }

  .footer-info-grid {
    grid-template-columns: 1fr;
  }

  .footer-meta,
  .footer-legal {
    align-items: flex-start;
    flex-direction: column;
  }

  .footer-meta-links {
    justify-content: flex-start;
  }

  .home-footer {
    gap: 18px;
  }
}

@media (max-width: 430px) {
  .device-topbar code {
    display: none;
  }

  .api-row {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .api-row code {
    font-size: 0.72rem;
  }

  .api-row em {
    display: none;
  }

  .code-tabs {
    overflow-x: auto;
  }
}

@media (max-width: 360px) {
  .hero-actions {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
  }
}
</style>
