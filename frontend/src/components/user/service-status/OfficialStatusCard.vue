<template>
  <section class="official-card" :aria-labelledby="`${cardId}-title`">
    <header class="card-header">
      <div class="header-copy">
        <h2 :id="`${cardId}-title`">{{ title }}</h2>
        <p v-if="updatedAt" class="section-meta">
          {{ t('serviceStatus.official.updatedAt', { time: formatDateTime(updatedAt) }) }}
          <span v-if="stale" class="stale-label">{{ t('serviceStatus.official.stale') }}</span>
        </p>
        <p v-else-if="error" class="section-meta error-label">{{ loadFailedText }}</p>
      </div>

      <div class="header-actions">
        <span v-if="!loading || updatedAt" class="status-pill" :class="`status-${status}`">
          <span class="status-dot" />
          {{ t(`serviceStatus.status.${status}`) }}
        </span>
        <a
          v-if="sourceUrl"
          :href="sourceUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="icon-button"
          :aria-label="t('serviceStatus.official.openSource')"
          :title="t('serviceStatus.official.openSource')"
        >
          <Icon name="externalLink" size="sm" />
        </a>
        <button
          type="button"
          class="icon-button"
          :disabled="loading"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          @click="$emit('refresh')"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
        </button>
      </div>
    </header>

    <div v-if="loading && !updatedAt" class="service-grid" aria-busy="true">
      <div v-for="item in items" :key="item.id" class="service-skeleton">
        <span />
        <span />
      </div>
    </div>

    <div v-else-if="error && !updatedAt" class="empty-state">
      <Icon name="exclamationCircle" size="lg" />
      <p>{{ loadFailedText }}</p>
      <button type="button" class="text-button" @click="$emit('refresh')">
        {{ t('serviceStatus.endpoints.test') }}
      </button>
    </div>

    <template v-else>
      <div class="service-grid">
        <article v-for="item in items" :key="item.id" class="service-row">
          <span class="service-icon" aria-hidden="true">
            <PlatformIcon v-if="item.platform" :platform="item.platform" size="md" />
            <Icon v-else :name="item.icon || 'infoCircle'" size="md" />
          </span>
          <strong>{{ item.name }}</strong>
          <span class="service-status" :class="`service-${item.status}`">
            <span class="service-dot" />
            {{ t(`serviceStatus.status.${item.status}`) }}
          </span>
        </article>
      </div>

      <details v-if="activeIncidents.length > 0" class="incident-block">
        <summary>
          <span>
            <Icon name="exclamationCircle" size="sm" />
            {{ t('serviceStatus.providers.activeIncidents', { count: activeIncidents.length }) }}
            <span class="incident-count">{{ activeIncidents.length }}</span>
          </span>
          <Icon name="chevronDown" size="sm" class="disclosure-chevron" />
        </summary>
        <div class="incident-list">
          <component
            :is="incident.sourceUrl ? 'a' : 'article'"
            v-for="incident in activeIncidents"
            :key="incident.id"
            :href="incident.sourceUrl || undefined"
            :target="incident.sourceUrl ? '_blank' : undefined"
            :rel="incident.sourceUrl ? 'noopener noreferrer' : undefined"
            class="incident-row"
          >
            <div>
              <strong>{{ incident.name }}</strong>
              <p v-if="incident.latestBody">{{ incident.latestBody }}</p>
            </div>
            <time v-if="incident.updatedAt" :datetime="incident.updatedAt">
              {{ formatDateTime(incident.updatedAt) }}
            </time>
          </component>
        </div>
      </details>

      <div v-else class="no-incidents">
        <Icon name="checkCircle" size="sm" />
        {{ t('serviceStatus.providers.noActive') }}
      </div>

      <details v-if="recentResolved.length > 0" class="resolved-block">
        <summary>
          {{ t('serviceStatus.official.recentResolved', { count: recentResolved.length }) }}
          <Icon name="chevronDown" size="sm" />
        </summary>
        <a
          v-for="incident in recentResolved"
          :key="incident.id"
          :href="incident.sourceUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="resolved-row"
        >
          <span>{{ incident.name }}</span>
          <time :datetime="incident.updatedAt">{{ formatDateTime(incident.updatedAt) }}</time>
        </a>
      </details>
    </template>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { formatDateTime } from '@/utils/format'
import type { GroupPlatform } from '@/types'
import type { ProviderHealthStatus } from '@/api/serviceStatus'

interface StatusItem {
  id: string
  name: string
  status: ProviderHealthStatus
  icon?: 'terminal' | 'chatBubble' | 'infoCircle'
  platform?: GroupPlatform
}

interface StatusIncident {
  id: string
  name: string
  latestBody: string
  updatedAt: string
  sourceUrl?: string
}

withDefaults(defineProps<{
  cardId: string
  title: string
  updatedAt: string
  stale: boolean
  status: ProviderHealthStatus
  items: StatusItem[]
  activeIncidents: StatusIncident[]
  recentResolved?: StatusIncident[]
  sourceUrl?: string
  loading: boolean
  error: boolean
  loadFailedText: string
}>(), {
  recentResolved: () => [],
  sourceUrl: '',
})

defineEmits<{ refresh: [] }>()

const { t } = useI18n()
</script>

<style scoped>
.official-card {
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 16px;
  background: var(--app-surface);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.125rem;
  border-bottom: 1px solid var(--app-border);
}

.header-copy { min-width: 0; }
.card-header h2 { color: var(--app-text); font-size: 0.9375rem; font-weight: 600; }
.section-meta { margin-top: 0.2rem; color: var(--app-muted); font-size: 0.75rem; }
.stale-label { color: #b7791f; }
.error-label { color: #b45309; }
.header-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 0.375rem; }

.icon-button {
  display: inline-flex;
  width: 2.25rem;
  height: 2.25rem;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  color: var(--app-muted-strong);
  transition: background 150ms ease, color 150ms ease;
}

.icon-button:hover { background: var(--app-surface-muted); color: var(--app-text); }
.icon-button:disabled { cursor: not-allowed; opacity: 0.45; }

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  border-radius: 999px;
  padding: 0.375rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 500;
  white-space: nowrap;
}

.status-dot,
.service-dot { flex: 0 0 auto; border-radius: 999px; background: currentColor; }
.status-dot { width: 0.375rem; height: 0.375rem; }
.service-dot { width: 0.3rem; height: 0.3rem; }
.status-operational { background: #ecfdf5; color: #047857; }
.status-degraded { background: #fffbeb; color: #b45309; }
.status-partial_outage { background: #fff7ed; color: #c2410c; }
.status-major_outage { background: #fef2f2; color: #b91c1c; }
.status-unknown { background: var(--app-surface-muted); color: var(--app-muted-strong); }
:global(.dark) .status-operational { background: rgb(6 78 59 / 28%); color: #6ee7b7; }
:global(.dark) .status-degraded { background: rgb(120 53 15 / 28%); color: #fcd34d; }
:global(.dark) .status-partial_outage { background: rgb(124 45 18 / 28%); color: #fdba74; }
:global(.dark) .status-major_outage { background: rgb(127 29 29 / 28%); color: #fca5a5; }

.service-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 1px;
  background: var(--app-border);
}

.service-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
  padding: 1.125rem;
  background: var(--app-surface);
}

.service-icon {
  display: grid;
  width: 2.5rem;
  height: 2.5rem;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 12px;
  background: var(--app-surface-muted);
  color: var(--app-text);
}

.service-row strong {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  color: var(--app-text);
  font-size: 0.875rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.service-status { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 0.3rem; font-size: 0.6875rem; white-space: nowrap; }
.service-operational { color: #059669; }
.service-degraded { color: #d97706; }
.service-partial_outage { color: #ea580c; }
.service-major_outage { color: #dc2626; }
.service-unknown { color: var(--app-muted); }

.no-incidents,
.incident-block,
.resolved-block { border-top: 1px solid var(--app-border); }

.no-incidents { display: flex; align-items: center; gap: 0.375rem; padding: 0.75rem 1.125rem; color: var(--app-muted); font-size: 0.75rem; }
.incident-block summary,
.resolved-block summary { display: flex; cursor: pointer; list-style: none; align-items: center; justify-content: space-between; gap: 0.5rem; padding: 0.875rem 1.125rem; font-size: 0.8125rem; font-weight: 500; }
.incident-block summary { color: #b45309; }
.resolved-block summary { color: var(--app-muted-strong); }
.incident-block summary::-webkit-details-marker,
.resolved-block summary::-webkit-details-marker { display: none; }
.incident-block summary > span { display: inline-flex; min-width: 0; align-items: center; gap: 0.375rem; }
.incident-block[open] .disclosure-chevron,
.resolved-block[open] summary svg { transform: rotate(180deg); }
.disclosure-chevron { flex: 0 0 auto; transition: transform 150ms ease; }

.incident-count { display: inline-grid; min-width: 1.25rem; height: 1.25rem; place-items: center; border-radius: 999px; background: #fffbeb; padding-inline: 0.375rem; color: #92400e; font-size: 0.6875rem; font-variant-numeric: tabular-nums; }
:global(.dark) .incident-count { background: rgb(120 53 15 / 28%); color: #fcd34d; }
.incident-list { display: grid; border-top: 1px solid var(--app-border); }
.incident-row,
.resolved-row { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 1rem; padding: 0.75rem 1.125rem; }
.incident-row + .incident-row,
.resolved-row { border-top: 1px solid var(--app-border); }
.incident-row:hover,
.resolved-row:hover { background: var(--app-surface-muted); }
.incident-row > div { min-width: 0; }
.incident-row strong,
.resolved-row span { color: var(--app-text); font-size: 0.8125rem; font-weight: 550; overflow-wrap: anywhere; }
.incident-row p { display: -webkit-box; margin-top: 0.2rem; overflow: hidden; color: var(--app-muted); font-size: 0.75rem; line-height: 1.5; overflow-wrap: anywhere; -webkit-box-orient: vertical; -webkit-line-clamp: 3; }
.incident-row time,
.resolved-row time { flex: 0 0 auto; color: var(--app-muted); font-size: 0.6875rem; }

.empty-state { display: flex; min-height: 8rem; flex-direction: column; align-items: center; justify-content: center; gap: 0.5rem; color: var(--app-muted); font-size: 0.8125rem; }
.text-button { color: var(--app-text); font-size: 0.8125rem; font-weight: 600; }
.service-skeleton { display: flex; gap: 0.75rem; padding: 1.125rem; background: var(--app-surface); }
.service-skeleton span { height: 2.5rem; border-radius: 10px; background: var(--app-surface-muted); animation: pulse 1.5s ease-in-out infinite; }
.service-skeleton span:first-child { width: 2.5rem; }
.service-skeleton span:last-child { flex: 1; }
@keyframes pulse { 50% { opacity: 0.55; } }

@media (max-width: 640px) {
  .card-header { align-items: flex-start; padding: 0.875rem 1rem; }
  .header-actions { gap: 0.25rem; }
  .status-pill { margin-top: 0.125rem; padding-inline: 0.5rem; }
  .service-grid { grid-template-columns: 1fr; }
  .service-row { padding: 1rem; }
  .no-incidents, .incident-block summary, .resolved-block summary, .incident-row, .resolved-row { padding-inline: 1rem; }
  .incident-row { display: block; }
  .incident-row time { display: block; margin-top: 0.375rem; }
}
</style>
