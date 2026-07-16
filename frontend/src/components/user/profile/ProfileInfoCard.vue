<template>
  <div class="min-w-0 max-w-full space-y-5 overflow-hidden">
    <section
      data-testid="profile-overview-hero"
      class="card overflow-hidden border border-[var(--claude-border)] bg-[var(--claude-surface)] shadow-sm dark:border-[var(--claude-border)] dark:bg-[var(--claude-surface)]"
    >
      <div class="flex flex-col gap-4 px-4 py-4 md:px-6 md:py-5 lg:px-7">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0 flex-1">
            <h2 class="text-lg font-semibold text-[var(--claude-text)] md:text-xl">
              {{ t('profile.title') }}
            </h2>
            <p class="mt-1 hidden text-sm text-[var(--claude-muted)] md:block">
              {{ t('profile.description') }}
            </p>
          </div>

          <div class="flex shrink-0 items-center justify-end gap-2">
            <button
              data-testid="profile-share-action"
              type="button"
              class="inline-flex h-9 w-9 items-center justify-center gap-2 rounded-full border border-[var(--claude-border)] bg-[var(--claude-surface-muted)] p-0 text-sm font-medium text-[var(--claude-text)] transition hover:border-[var(--claude-accent)] hover:text-[var(--claude-accent)] sm:h-auto sm:w-auto sm:px-3.5 sm:py-2"
              :aria-label="t('profile.share.action')"
              @click="shareDialogOpen = true"
            >
              <Icon name="upload" size="sm" />
              <span class="hidden sm:inline">{{ t('profile.share.action') }}</span>
            </button>
            <button
              data-testid="profile-edit-action"
              type="button"
              class="inline-flex h-9 w-9 items-center justify-center gap-2 rounded-full bg-[var(--claude-text)] p-0 text-sm font-medium text-[var(--claude-bg)] transition hover:bg-[var(--claude-accent)] sm:h-auto sm:w-auto sm:px-3.5 sm:py-2"
              :aria-label="t('profile.editShort')"
              @click="editDialogOpen = true"
            >
              <Icon name="edit" size="sm" />
              <span class="hidden sm:inline">{{ t('profile.editShort') }}</span>
            </button>
          </div>
        </div>

        <div class="flex min-w-0 flex-col items-center text-center">
          <div
            class="flex h-20 w-20 shrink-0 items-center justify-center overflow-hidden rounded-[1.65rem] bg-[var(--claude-text)] text-2xl font-semibold text-[var(--claude-bg)] shadow-lg shadow-black/10 md:h-28 md:w-28 md:rounded-[2rem] md:text-3xl"
          >
            <img
              v-if="avatarUrl"
              :src="avatarUrl"
              :alt="displayName"
              class="h-full w-full object-cover"
            >
            <span v-else>{{ avatarInitial }}</span>
          </div>

          <div class="mt-3 min-w-0 max-w-full md:mt-4">
            <div class="flex flex-wrap items-center justify-center gap-2">
              <h1 class="max-w-full truncate text-2xl font-semibold text-[var(--claude-text)] md:text-3xl">
                {{ displayName }}
              </h1>
              <span :class="['badge', user?.role === 'admin' ? 'badge-primary' : 'badge-gray']">
                {{ user?.role === 'admin' ? t('profile.administrator') : t('profile.user') }}
              </span>
              <span :class="['badge', user?.status === 'active' ? 'badge-success' : 'badge-danger']">
                {{ user?.status === 'active' ? t('common.active') : t('common.disabled') }}
              </span>
            </div>
            <p
              v-if="primaryEmailDisplay"
              class="mt-2 truncate text-sm text-[var(--claude-muted)]"
            >
              {{ primaryEmailDisplay }}
            </p>
          </div>
        </div>

        <div class="grid min-w-0 grid-cols-2 gap-2 md:gap-3 xl:grid-cols-4">
          <div
            data-testid="profile-overview-metric-balance"
            class="min-w-0 rounded-xl border border-[var(--claude-border)] bg-white/55 px-3 py-2.5 dark:bg-[var(--claude-surface-muted)] md:rounded-2xl md:px-4 md:py-3"
            >
              <p class="text-xs font-medium text-[var(--claude-muted)]">
                {{ t('profile.accountBalance') }}
              </p>
              <p class="mt-1 truncate text-base font-semibold text-[var(--claude-text)] md:text-lg">
                {{ formatCurrency(user?.balance || 0) }}
              </p>
            </div>
          <div
            data-testid="profile-overview-metric-points"
            class="min-w-0 rounded-xl border border-[var(--claude-border)] bg-white/55 px-3 py-2.5 dark:bg-[var(--claude-surface-muted)] md:rounded-2xl md:px-4 md:py-3"
            >
              <p class="text-xs font-medium text-[var(--claude-muted)]">
                {{ t('profile.points') }}
              </p>
              <p class="mt-1 truncate text-base font-semibold text-[var(--claude-text)] md:text-lg">
                {{ formatPoints(user?.points_balance || 0) }}
              </p>
            </div>
          <div
            data-testid="profile-overview-metric-concurrency"
            class="min-w-0 rounded-xl border border-[var(--claude-border)] bg-white/55 px-3 py-2.5 dark:bg-[var(--claude-surface-muted)] md:rounded-2xl md:px-4 md:py-3"
            >
              <p class="text-xs font-medium text-[var(--claude-muted)]">
                {{ t('profile.concurrencyLimit') }}
              </p>
              <p class="mt-1 truncate text-base font-semibold text-[var(--claude-text)] md:text-lg">
                {{ user?.concurrency || 0 }}
              </p>
            </div>
          <div
            data-testid="profile-overview-metric-member-since"
            class="min-w-0 rounded-xl border border-[var(--claude-border)] bg-white/55 px-3 py-2.5 dark:bg-[var(--claude-surface-muted)] md:rounded-2xl md:px-4 md:py-3"
            >
              <p class="text-xs font-medium text-[var(--claude-muted)]">
                {{ t('profile.memberSince') }}
              </p>
              <p class="mt-1 truncate text-base font-semibold text-[var(--claude-text)] md:text-lg">
                {{ memberSinceLabel }}
              </p>
            </div>
          </div>

        <div
          v-if="sourceHints.length"
          class="flex flex-wrap justify-center gap-2 text-xs text-[var(--claude-muted)]"
        >
          <span
            v-for="hint in sourceHints"
            :key="hint.key"
            class="inline-flex min-w-0 max-w-full items-center gap-1 rounded-full bg-white/60 px-3 py-1 ring-1 ring-[var(--claude-border)] dark:bg-[var(--claude-surface-muted)]"
          >
            <Icon name="link" size="sm" class="shrink-0" />
            <span class="min-w-0 truncate">{{ hint.text }}</span>
          </span>
        </div>
      </div>
    </section>

    <ProfileTokenActivityHeatmap @summary="activitySummary = $event" />

    <div class="grid min-w-0 items-start gap-5 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,0.85fr)] 2xl:grid-cols-[minmax(0,1.75fr)_minmax(0,0.85fr)]">
      <div data-testid="profile-main-column" class="min-w-0 space-y-5">
        <slot name="main-after" />
      </div>

      <div data-testid="profile-side-column" class="min-w-0 space-y-5">
        <section
          data-testid="profile-auth-bindings-panel"
          class="card border border-[var(--claude-border)] bg-[var(--claude-surface)] p-5 shadow-sm dark:border-[var(--claude-border)] dark:bg-[var(--claude-surface)] md:p-6"
        >
          <ProfileIdentityBindingsSection
            :user="user"
            :linuxdo-enabled="linuxdoEnabled"
            :oidc-enabled="oidcEnabled"
            :oidc-provider-name="oidcProviderName"
            :wechat-enabled="wechatEnabled"
            :wechat-open-enabled="wechatOpenEnabled"
            :wechat-mp-enabled="wechatMpEnabled"
            embedded
            compact
          />
        </section>

        <section
          v-if="sourceHints.length"
          class="card border border-[var(--claude-border)] bg-[var(--claude-surface)] p-5 shadow-sm dark:border-[var(--claude-border)] dark:bg-[var(--claude-surface)] md:p-6"
        >
          <h3 class="text-lg font-semibold text-[var(--claude-text)]">
            {{ t('profile.linkedProfileSources') }}
          </h3>
          <p class="mt-1 text-sm text-[var(--claude-muted)]">
            {{ t('profile.linkedProfileSourcesDescription') }}
          </p>

          <div class="mt-5 grid gap-3">
            <div
              v-for="hint in sourceHints"
              :key="hint.key"
              class="flex min-w-0 items-start gap-3 rounded-2xl border border-[var(--claude-border)] bg-white/55 px-4 py-3 text-sm text-[var(--claude-muted)] dark:bg-[var(--claude-surface-muted)]"
            >
              <Icon name="link" size="sm" class="mt-0.5 shrink-0" />
              <span class="min-w-0 break-words">{{ hint.text }}</span>
            </div>
          </div>
        </section>

        <slot name="side-after" />
      </div>
    </div>

    <Teleport to="body">
      <div
        v-if="editDialogOpen"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/35 p-4 backdrop-blur-sm"
        @click.self="editDialogOpen = false"
      >
        <section
          data-testid="profile-basics-panel"
          class="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl border border-[var(--claude-border)] bg-[var(--claude-surface)] p-5 shadow-2xl dark:border-[var(--claude-border)] dark:bg-[var(--claude-surface)] md:p-6"
        >
          <div class="mb-5 flex items-start justify-between gap-4">
            <div class="min-w-0">
              <h3 class="text-lg font-semibold text-[var(--claude-text)]">
                {{ t('profile.basicsTitle') }}
              </h3>
              <p class="mt-1 text-sm text-[var(--claude-muted)]">
                {{ t('profile.basicsDescription') }}
              </p>
            </div>
            <button
              type="button"
              class="rounded-full border border-[var(--claude-border)] p-2 text-[var(--claude-muted)] transition hover:border-[var(--claude-accent)] hover:text-[var(--claude-accent)]"
              :aria-label="t('common.close')"
              @click="editDialogOpen = false"
            >
              <Icon name="x" size="sm" />
            </button>
          </div>

          <div class="grid min-w-0 gap-5 lg:grid-cols-2">
            <div class="min-w-0 rounded-2xl border border-[var(--claude-border)] bg-white/55 p-5 dark:bg-[var(--claude-surface-muted)]">
              <ProfileAvatarCard
                :user="user"
                embedded
              />
            </div>

            <div class="min-w-0 rounded-2xl border border-[var(--claude-border)] bg-white/55 p-5 dark:bg-[var(--claude-surface-muted)]">
              <ProfileEditForm
                :initial-username="user?.username || ''"
                :initial-prefer-points-billing="user?.prefer_points_billing === true"
                embedded
              />
            </div>
          </div>
        </section>
      </div>

      <div
        v-if="shareDialogOpen"
        class="fixed inset-0 z-50 overflow-y-auto bg-black/35 p-4 backdrop-blur-sm"
        @click.self="shareDialogOpen = false"
      >
        <div
          class="flex min-h-full items-start justify-center py-4 sm:items-center"
          @click.self="shareDialogOpen = false"
        >
        <section class="w-full max-w-3xl rounded-2xl border border-[var(--claude-border)] bg-[var(--claude-surface)] p-5 shadow-2xl dark:border-[var(--claude-border)] dark:bg-[var(--claude-surface)] md:p-6">
          <div class="mb-5 flex items-start justify-between gap-4">
            <div class="min-w-0">
              <h3 class="text-lg font-semibold text-[var(--claude-text)]">
                {{ t('profile.share.title') }}
              </h3>
              <p class="mt-1 text-sm text-[var(--claude-muted)]">
                {{ t('profile.share.description') }}
              </p>
            </div>
            <button
              type="button"
              class="rounded-full border border-[var(--claude-border)] p-2 text-[var(--claude-muted)] transition hover:border-[var(--claude-accent)] hover:text-[var(--claude-accent)]"
              :aria-label="t('common.close')"
              @click="shareDialogOpen = false"
            >
              <Icon name="x" size="sm" />
            </button>
          </div>

          <div class="overflow-hidden rounded-2xl border border-[var(--claude-border)] bg-[#ffffff] p-5 text-[#171717] shadow-sm">
            <div class="flex items-start justify-between gap-4">
              <div class="flex min-w-0 items-center gap-3">
                <div class="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-[#171717] text-xl font-semibold text-[#f7f7f8]">
                  <img
                    v-if="avatarUrl"
                    :src="avatarUrl"
                    :alt="displayName"
                    class="h-full w-full object-cover"
                  >
                  <span v-else>{{ avatarInitial }}</span>
                </div>
                <div class="min-w-0">
                  <p class="truncate text-base font-semibold">{{ displayName }}</p>
                  <p
                    v-if="primaryEmailDisplay"
                    class="truncate text-sm text-[#6e6e80]"
                  >
                    {{ primaryEmailDisplay }}
                  </p>
                </div>
              </div>
              <p class="text-sm font-semibold uppercase tracking-[0.18em] text-[#6e6e80]">
                ikik
              </p>
            </div>

            <div class="mt-6">
              <p class="mb-3 text-sm font-medium text-[#6e6e80]">{{ t('profile.activity.title') }}</p>
              <div class="grid grid-flow-col grid-rows-7 gap-1 overflow-hidden">
                <span
                  v-for="cell in previewCells"
                  :key="`preview-${cell.date}`"
                  class="h-2.5 w-2.5 rounded-[3px]"
                  :style="{ backgroundColor: levelColor(cell.level) }"
                />
              </div>
            </div>

            <div class="mt-6 grid gap-3 sm:grid-cols-4">
              <div
                v-for="metric in shareMetrics"
                :key="metric.label"
                class="rounded-xl bg-[#f3f3f6] px-3 py-3"
              >
                <p class="text-[11px] font-medium text-[#6e6e80]">{{ metric.label }}</p>
                <p class="mt-1 truncate text-base font-semibold">{{ metric.value }}</p>
              </div>
            </div>
          </div>

          <div class="mt-5 flex flex-col gap-3 sm:flex-row sm:justify-end">
            <button
              type="button"
              class="inline-flex items-center justify-center gap-2 rounded-full border border-[var(--claude-border)] bg-[var(--claude-surface-muted)] px-4 py-2 text-sm font-medium text-[var(--claude-text)] transition hover:border-[var(--claude-accent)] hover:text-[var(--claude-accent)]"
              @click="shareWebsite"
            >
              <Icon name="link" size="sm" />
              {{ t('profile.share.website') }}
            </button>
            <button
              type="button"
              class="inline-flex items-center justify-center gap-2 rounded-full bg-[var(--claude-text)] px-4 py-2 text-sm font-medium text-[var(--claude-bg)] transition hover:bg-[var(--claude-accent)]"
              @click="shareImage"
            >
              <Icon name="download" size="sm" />
              {{ t('profile.share.image') }}
            </button>
          </div>
        </section>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ProfileAvatarCard from '@/components/user/profile/ProfileAvatarCard.vue'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import ProfileIdentityBindingsSection from '@/components/user/profile/ProfileIdentityBindingsSection.vue'
import ProfileTokenActivityHeatmap from '@/components/user/profile/ProfileTokenActivityHeatmap.vue'
import { useAppStore } from '@/stores/app'
import type { User, UserAuthBindingStatus, UserAuthProvider, UserProfileSourceContext } from '@/types'

type HeatmapCell = {
  date: string
  tokens: number
  level: number
  inRange: boolean
}

type ActivitySummary = {
  cells: HeatmapCell[]
  totalTokens: number
  peakDayTokens: number
  currentStreak: number
  longestStreak: number
}

type ProfileShareData = {
  files?: File[]
  title?: string
  text?: string
  url?: string
}

const props = withDefaults(defineProps<{
  user: User | null
  linuxdoEnabled?: boolean
  oidcEnabled?: boolean
  oidcProviderName?: string
  wechatEnabled?: boolean
  wechatOpenEnabled?: boolean
  wechatMpEnabled?: boolean
}>(), {
  linuxdoEnabled: false,
  oidcEnabled: false,
  oidcProviderName: 'OIDC',
  wechatEnabled: false,
  wechatOpenEnabled: undefined,
  wechatMpEnabled: undefined,
})

const { t } = useI18n()
const appStore = useAppStore()
const editDialogOpen = ref(false)
const shareDialogOpen = ref(false)
const activitySummary = ref<ActivitySummary>({
  cells: [],
  totalTokens: 0,
  peakDayTokens: 0,
  currentStreak: 0,
  longestStreak: 0
})

function normalizeBindingStatus(binding: boolean | UserAuthBindingStatus | undefined): boolean | null {
  if (typeof binding === 'boolean') {
    return binding
  }
  if (!binding) {
    return null
  }
  if (typeof binding.bound === 'boolean') {
    return binding.bound
  }
  return Boolean(binding.provider_subject || binding.issuer || binding.provider_key)
}

function isEmailBound(user: User | null | undefined): boolean {
  if (typeof user?.email_bound === 'boolean') {
    return user.email_bound
  }

  const nested = user?.auth_bindings?.email ?? user?.identity_bindings?.email
  const normalized = normalizeBindingStatus(nested)
  return normalized ?? false
}

const avatarUrl = computed(() => props.user?.avatar_url?.trim() || '')
const displayName = computed(() => props.user?.username?.trim() || props.user?.email?.trim() || t('profile.user'))
const primaryEmailDisplay = computed(() => {
  const email = props.user?.email?.trim() || ''
  if (!email) {
    return ''
  }
  if (email.endsWith('.invalid') && !isEmailBound(props.user)) {
    return ''
  }
  return email
})
const avatarInitial = computed(() => displayName.value.charAt(0).toUpperCase() || 'U')
const memberSinceLabel = computed(() => {
  const raw = props.user?.created_at?.trim()
  if (!raw) {
    return '-'
  }

  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }

  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
  }).format(date)
})

const shareMetrics = computed(() => [
  {
    label: t('profile.activity.totalTokens'),
    value: formatCompactNumber(activitySummary.value.totalTokens)
  },
  {
    label: t('profile.activity.peakDay'),
    value: formatCompactNumber(activitySummary.value.peakDayTokens)
  },
  {
    label: t('profile.activity.currentStreak'),
    value: t('profile.activity.days', { count: activitySummary.value.currentStreak })
  },
  {
    label: t('profile.activity.longestStreak'),
    value: t('profile.activity.days', { count: activitySummary.value.longestStreak })
  }
])

const previewCells = computed(() => {
  if (activitySummary.value.cells.length > 0) {
    return activitySummary.value.cells.slice(-154)
  }

  return Array.from({ length: 154 }, (_, index) => ({
    date: `empty-${index}`,
    tokens: 0,
    level: 0,
    inRange: true
  }))
})

const providerLabels = computed<Record<UserAuthProvider, string>>(() => ({
  email: t('profile.authBindings.providers.email'),
  linuxdo: t('profile.authBindings.providers.linuxdo'),
  oidc: t('profile.authBindings.providers.oidc', { providerName: props.oidcProviderName }),
  wechat: t('profile.authBindings.providers.wechat'),
  github: 'GitHub',
  google: 'Google'
}))

function formatCurrency(value: number): string {
  return `$${value.toFixed(2)}`
}

function formatPoints(value: number): string {
  return Number(value || 0).toFixed(10).replace(/\.?0+$/, '') || '0'
}

function normalizeProvider(value: string): UserAuthProvider | null {
  const normalized = value.trim().toLowerCase()
  if (
    normalized === 'email' ||
    normalized === 'linuxdo' ||
    normalized === 'wechat' ||
    normalized === 'github' ||
    normalized === 'google'
  ) {
    return normalized
  }
  if (normalized === 'oidc' || normalized.startsWith('oidc:') || normalized.startsWith('oidc/')) {
    return 'oidc'
  }
  return null
}

function readObjectString(source: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = source[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}

function resolveThirdPartySource(
  rawSource: string | UserProfileSourceContext | null | undefined
): { provider: UserAuthProvider; label: string } | null {
  if (!rawSource) {
    return null
  }

  if (typeof rawSource === 'string') {
    const provider = normalizeProvider(rawSource)
    if (!provider || provider === 'email') {
      return null
    }
    return {
      provider,
      label: providerLabels.value[provider]
    }
  }

  const sourceRecord = rawSource as Record<string, unknown>
  const provider = normalizeProvider(
    readObjectString(sourceRecord, 'provider', 'source', 'provider_type', 'auth_provider')
  )
  if (!provider || provider === 'email') {
    return null
  }

  const explicitLabel = readObjectString(
    sourceRecord,
    'provider_label',
    'label',
    'provider_name',
    'providerName'
  )

  return {
    provider,
    label: explicitLabel || providerLabels.value[provider]
  }
}

const sourceHints = computed(() => {
  const currentUser = props.user
  if (!currentUser) {
    return []
  }

  const hints: Array<{ key: string; text: string }> = []
  const avatarSource = resolveThirdPartySource(
    currentUser.profile_sources?.avatar ?? currentUser.avatar_source
  )
  const usernameSource = resolveThirdPartySource(
    currentUser.profile_sources?.username ??
      currentUser.profile_sources?.display_name ??
      currentUser.profile_sources?.nickname ??
      currentUser.display_name_source ??
      currentUser.username_source ??
      currentUser.nickname_source
  )

  if (avatarSource) {
    hints.push({
      key: 'avatar',
      text: t('profile.authBindings.source.avatar', { providerName: avatarSource.label })
    })
  }

  if (usernameSource) {
    hints.push({
      key: 'username',
      text: t('profile.authBindings.source.username', { providerName: usernameSource.label })
    })
  }

  return hints
})

async function shareWebsite() {
  const url = typeof window !== 'undefined' ? window.location.href : ''
  const title = `${displayName.value} - ikik`
  try {
    if (typeof navigator !== 'undefined' && typeof navigator.share === 'function') {
      await navigator.share({
        title,
        text: t('profile.share.websiteText', { name: displayName.value }),
        url
      })
      return
    }

    await navigator.clipboard.writeText(url)
    appStore.showSuccess(t('profile.share.linkCopied'))
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    console.error('Failed to share profile link:', error)
    appStore.showError(t('profile.share.failed'))
  }
}

async function shareImage() {
  try {
    const canvas = await renderShareCanvas()
    const blob = await canvasToBlob(canvas, 'image/png')
    const filename = `ikik-profile-${props.user?.id || 'user'}.png`
    const file = new File([blob], filename, { type: 'image/png' })
    const navWithCanShare = navigator as Navigator & {
      canShare?: (data: ProfileShareData) => boolean
    }

    if (
      typeof navigator.share === 'function' &&
      (!navWithCanShare.canShare || navWithCanShare.canShare({ files: [file] }))
    ) {
      await navigator.share({
        title: `${displayName.value} - ikik`,
        files: [file]
      })
      return
    }

    downloadBlob(blob, filename)
    appStore.showSuccess(t('profile.share.imageDownloaded'))
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    console.error('Failed to share profile image:', error)
    appStore.showError(t('profile.share.failed'))
  }
}

async function renderShareCanvas(): Promise<HTMLCanvasElement> {
  const canvas = document.createElement('canvas')
  const scale = 1
  const width = 1000
  const height = 520
  canvas.width = width * scale
  canvas.height = height * scale
  canvas.style.width = `${width}px`
  canvas.style.height = `${height}px`

  const ctx = canvas.getContext('2d')
  if (!ctx) {
    throw new Error('canvas_context_unavailable')
  }

  ctx.scale(scale, scale)
  ctx.fillStyle = '#f7f7f8'
  ctx.fillRect(0, 0, width, height)

  roundedRect(ctx, 54, 44, width - 108, height - 88, 34)
  ctx.fillStyle = '#ffffff'
  ctx.fill()
  ctx.strokeStyle = '#d9d9e3'
  ctx.lineWidth = 2
  ctx.stroke()

  await drawAvatar(ctx, 102, 88, 72)

  ctx.fillStyle = '#171717'
  ctx.font = '600 30px Inter, "Microsoft YaHei", sans-serif'
  drawTruncatedText(ctx, displayName.value, 196, 116, 450)
  ctx.fillStyle = '#6e6e80'
  ctx.font = '18px Inter, "Microsoft YaHei", sans-serif'
  if (primaryEmailDisplay.value) {
    drawTruncatedText(ctx, primaryEmailDisplay.value, 196, 146, 450)
  }
  ctx.font = '700 18px Inter, "Microsoft YaHei", sans-serif'
  ctx.fillText('IKIK', 820, 116)

  ctx.fillStyle = '#6e6e80'
  ctx.font = '600 18px Inter, "Microsoft YaHei", sans-serif'
  ctx.fillText(t('profile.activity.title'), 102, 198)

  const cellsForImage = activitySummary.value.cells.length
    ? activitySummary.value.cells.slice(-371)
    : Array.from({ length: 371 }, () => ({ level: 0 }))
  const cellSize = 10
  const gap = 4
  const startX = 102
  const startY = 226
  cellsForImage.forEach((cell, index) => {
    const week = Math.floor(index / 7)
    const day = index % 7
    ctx.fillStyle = levelColor(cell.level)
    roundedRect(ctx, startX + week * (cellSize + gap), startY + day * (cellSize + gap), cellSize, cellSize, 3)
    ctx.fill()
  })

  const metricY = 382
  shareMetrics.value.forEach((metric, index) => {
    const x = 102 + index * 212
    roundedRect(ctx, x, metricY, 184, 84, 18)
    ctx.fillStyle = '#f3f3f6'
    ctx.fill()
    ctx.fillStyle = '#6e6e80'
    ctx.font = '500 15px Inter, "Microsoft YaHei", sans-serif'
    drawTruncatedText(ctx, metric.label, x + 18, metricY + 30, 148)
    ctx.fillStyle = '#171717'
    ctx.font = '700 24px Inter, "Microsoft YaHei", sans-serif'
    drawTruncatedText(ctx, metric.value, x + 18, metricY + 62, 148)
  })

  return canvas
}

async function drawAvatar(ctx: CanvasRenderingContext2D, x: number, y: number, size: number) {
  if (!avatarUrl.value) {
    drawAvatarFallback(ctx, x, y, size)
    return
  }

  try {
    const image = await loadImage(avatarUrl.value)
    ctx.save()
    roundedRect(ctx, x, y, size, size, 20)
    ctx.clip()
    ctx.drawImage(image, x, y, size, size)
    ctx.restore()
  } catch {
    drawAvatarFallback(ctx, x, y, size)
  }
}

function drawAvatarFallback(ctx: CanvasRenderingContext2D, x: number, y: number, size: number) {
  roundedRect(ctx, x, y, size, size, 20)
  ctx.fillStyle = '#171717'
  ctx.fill()
  ctx.fillStyle = '#f7f7f8'
  ctx.font = '700 30px Inter, "Microsoft YaHei", sans-serif'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(avatarInitial.value, x + size / 2, y + size / 2)
  ctx.textAlign = 'start'
  ctx.textBaseline = 'alphabetic'
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.crossOrigin = 'anonymous'
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error('image_load_failed'))
    image.src = src
  })
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (!blob) {
        reject(new Error('canvas_blob_failed'))
        return
      }
      resolve(blob)
    }, type)
  })
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}

function roundedRect(ctx: CanvasRenderingContext2D, x: number, y: number, width: number, height: number, radius: number) {
  const r = Math.min(radius, width / 2, height / 2)
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + width, y, x + width, y + height, r)
  ctx.arcTo(x + width, y + height, x, y + height, r)
  ctx.arcTo(x, y + height, x, y, r)
  ctx.arcTo(x, y, x + width, y, r)
  ctx.closePath()
}

function drawTruncatedText(ctx: CanvasRenderingContext2D, text: string, x: number, y: number, maxWidth: number) {
  const value = String(text || '')
  if (ctx.measureText(value).width <= maxWidth) {
    ctx.fillText(value, x, y)
    return
  }

  let next = value
  while (next.length > 1 && ctx.measureText(`${next}...`).width > maxWidth) {
    next = next.slice(0, -1)
  }
  ctx.fillText(`${next}...`, x, y)
}

function levelColor(level: number): string {
  const colors = ['#ebedf0', '#9be9a8', '#40c463', '#30a14e', '#216e39']
  return colors[Math.min(Math.max(level, 0), colors.length - 1)]
}

function formatCompactNumber(value: number): string {
  const abs = Math.abs(value || 0)
  if (abs >= 1_000_000_000) {
    return `${trimNumber(value / 1_000_000_000)}B`
  }
  if (abs >= 1_000_000) {
    return `${trimNumber(value / 1_000_000)}M`
  }
  if (abs >= 1_000) {
    return `${trimNumber(value / 1_000)}K`
  }
  return `${Math.round(value || 0)}`
}

function trimNumber(value: number): string {
  return value.toFixed(value >= 10 ? 1 : 2).replace(/\.?0+$/, '')
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}
</script>
