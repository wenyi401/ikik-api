<template>
  <aside class="mission-dock" :class="{ 'mission-dock--open': onboardingStore.missionPanelOpen }">
    <button
      v-if="!onboardingStore.missionPanelOpen"
      type="button"
      class="mission-launcher"
      :aria-label="t('onboarding.journey.open')"
      @click="onboardingStore.setMissionPanelOpen(true)"
    >
      <span class="mission-launcher__ring" :style="progressRingStyle">
        <Icon :name="onboardingStore.journeyComplete ? 'badge' : 'bolt'" size="md" />
      </span>
      <span class="mission-launcher__copy">
        <strong>{{ t('onboarding.journey.title') }}</strong>
        <small>{{ completedCount }}/{{ missions.length }}</small>
      </span>
    </button>

    <div v-else class="mission-panel">
      <header class="mission-panel__header">
        <div>
          <span>{{ t('onboarding.journey.eyebrow') }}</span>
          <h2>{{ t('onboarding.journey.title') }}</h2>
        </div>
        <UiIconButton
          size="sm"
          :label="t('common.close')"
          @click="onboardingStore.setMissionPanelOpen(false)"
        >
          <Icon name="x" size="sm" />
        </UiIconButton>
      </header>

      <div class="mission-progress">
        <div class="mission-progress__meta">
          <span>{{ t('onboarding.journey.progress') }}</span>
          <strong>{{ Math.round(onboardingStore.journeyProgress * 100) }}%</strong>
        </div>
        <div class="mission-progress__track">
          <span :style="{ width: `${onboardingStore.journeyProgress * 100}%` }" />
        </div>
      </div>

      <ol class="mission-list">
        <li
          v-for="(mission, index) in missions"
          :key="mission.id"
          :class="{
            'mission-item--done': isCompleted(mission.id),
            'mission-item--current': onboardingStore.currentMission === mission.id
          }"
        >
          <span class="mission-item__state">
            <Icon v-if="isCompleted(mission.id)" name="check" size="sm" :stroke-width="2.5" />
            <span v-else>{{ index + 1 }}</span>
          </span>
          <span class="mission-item__copy">
            <strong>{{ t(`onboarding.journey.missions.${mission.id}.title`) }}</strong>
            <small v-if="onboardingStore.currentMission === mission.id">
              {{ t(`onboarding.journey.missions.${mission.id}.description`) }}
            </small>
          </span>
        </li>
      </ol>

      <button
        v-if="onboardingStore.currentMission"
        type="button"
        class="mission-action"
        @click="openCurrentMission"
      >
        <span>{{ t(`onboarding.journey.missions.${onboardingStore.currentMission}.action`) }}</span>
        <Icon name="arrowRight" size="sm" />
      </button>
      <div v-else class="mission-complete">
        <Icon name="badge" size="lg" />
        <span>
          <strong>{{ t('onboarding.journey.complete.title') }}</strong>
          <small>{{ t('onboarding.journey.complete.description') }}</small>
        </span>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { UiIconButton } from '@/ui'
import {
  beginnerMissionIds,
  useOnboardingStore,
  type BeginnerMissionId
} from '@/stores/onboarding'

const router = useRouter()
const { t } = useI18n()
const onboardingStore = useOnboardingStore()

const missions = beginnerMissionIds.map((id) => ({ id }))
const completedCount = computed(() => onboardingStore.completedMissions.length)
const progressRingStyle = computed(() => ({
  background: `conic-gradient(var(--guide-control) ${onboardingStore.journeyProgress * 360}deg, var(--app-border) 0deg)`
}))

function isCompleted(id: BeginnerMissionId): boolean {
  return onboardingStore.completedMissions.includes(id)
}

async function openCurrentMission(): Promise<void> {
  const targetByMission: Record<BeginnerMissionId, { path: string; query?: Record<string, string> }> = {
    groups: { path: '/keys', query: { guide: 'groups' } },
    monitor: { path: '/monitor', query: { guide: 'monitor', return: '/keys' } },
    key: { path: '/keys', query: { guide: 'key' } },
    client: { path: '/keys', query: { guide: 'client' } },
    first_call: { path: '/keys', query: { guide: 'first_call' } }
  }
  const mission = onboardingStore.currentMission
  if (!mission) return
  onboardingStore.setMissionPanelOpen(false)
  await router.push(targetByMission[mission])
}
</script>

<style scoped>
.mission-dock {
  --guide-control: #08775c;
  --guide-control-hover: #065f4a;
  --guide-control-foreground: #ffffff;
  --guide-accent-text: #08775c;
  --guide-accent-soft: #e6f6f1;
  --guide-accent-border: #77b9a6;
  --guide-focus: rgba(8, 119, 92, 0.3);
  position: fixed;
  right: 1rem;
  bottom: 1rem;
  z-index: 40;
  width: min(21rem, calc(100vw - 2rem));
  pointer-events: none;
}

:global(html.dark) .mission-dock {
  --guide-accent-text: #78deb9;
  --guide-accent-soft: rgba(8, 119, 92, 0.24);
  --guide-accent-border: #348f72;
  --guide-focus: rgba(120, 222, 185, 0.34);
}

.mission-launcher,
.mission-panel {
  pointer-events: auto;
}

.mission-launcher {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 10.75rem;
  margin-left: auto;
  padding: 0.55rem 0.75rem;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
  color: var(--app-text);
  box-shadow: 0 12px 28px rgba(15, 45, 37, 0.14);
  text-align: left;
  transition: border-color 180ms ease, transform 180ms ease;
}

.mission-launcher:hover {
  border-color: var(--guide-accent-border);
  transform: translateY(-2px);
}

.mission-launcher:focus-visible,
.mission-action:focus-visible {
  outline: 3px solid var(--guide-focus);
  outline-offset: 2px;
}

.mission-launcher__ring {
  display: grid;
  width: 2.5rem;
  height: 2.5rem;
  flex: none;
  place-items: center;
  border-radius: 50%;
  color: white;
  box-shadow: inset 0 0 0 4px var(--app-surface);
}

.mission-launcher__copy strong,
.mission-launcher__copy small {
  display: block;
}

.mission-launcher__copy strong {
  font-size: 0.8125rem;
}

.mission-launcher__copy small {
  margin-top: 0.1rem;
  color: var(--app-muted);
  font-size: 0.6875rem;
  font-variant-numeric: tabular-nums;
}

.mission-panel {
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
  box-shadow: 0 20px 42px rgba(15, 45, 37, 0.18);
  animation: mission-panel-in 220ms ease-out both;
}

.mission-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.875rem 1rem;
  border-bottom: 1px solid var(--app-border);
}

.mission-panel__header span {
  color: var(--guide-accent-text);
  font-size: 0.625rem;
  font-weight: 800;
}

.mission-panel__header h2 {
  margin-top: 0.1rem;
  color: var(--app-text);
  font-size: 0.9375rem;
  font-weight: 750;
}

.mission-progress {
  padding: 0.75rem 1rem;
  background: var(--app-surface-muted);
}

.mission-progress__meta {
  display: flex;
  justify-content: space-between;
  color: var(--app-muted);
  font-size: 0.6875rem;
}

.mission-progress__meta strong {
  color: var(--app-text);
  font-variant-numeric: tabular-nums;
}

.mission-progress__track {
  height: 0.3rem;
  margin-top: 0.45rem;
  overflow: hidden;
  border-radius: 3px;
  background: var(--app-border);
}

.mission-progress__track span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--guide-control);
  transition: width 300ms ease;
}

.mission-list {
  margin: 0;
  padding: 0.5rem 1rem;
  list-style: none;
}

.mission-list li {
  display: grid;
  grid-template-columns: 1.75rem 1fr;
  gap: 0.625rem;
  min-height: 2.5rem;
  align-items: center;
  padding: 0.4rem 0;
  color: var(--app-muted);
}

.mission-item__state {
  display: grid;
  width: 1.5rem;
  height: 1.5rem;
  place-items: center;
  border: 1px solid var(--app-border);
  border-radius: 5px;
  font-size: 0.6875rem;
  font-weight: 750;
  font-variant-numeric: tabular-nums;
}

.mission-item__copy strong,
.mission-item__copy small {
  display: block;
}

.mission-item__copy strong {
  font-size: 0.75rem;
  font-weight: 650;
}

.mission-item__copy small {
  margin-top: 0.15rem;
  font-size: 0.6875rem;
  line-height: 1.4;
}

.mission-item--current {
  color: var(--app-text) !important;
}

.mission-item--current .mission-item__state {
  border-color: var(--guide-accent-border);
  background: var(--guide-accent-soft);
  color: var(--guide-accent-text);
}

.mission-item--done .mission-item__state {
  border-color: var(--guide-control);
  background: var(--guide-control);
  color: var(--guide-control-foreground);
}

.mission-item--done .mission-item__copy strong {
  color: var(--app-muted);
  text-decoration: line-through;
}

.mission-action {
  display: flex;
  width: calc(100% - 2rem);
  min-height: 2.5rem;
  align-items: center;
  justify-content: space-between;
  margin: 0.25rem 1rem 1rem;
  padding: 0.6rem 0.75rem;
  border-radius: 6px;
  border: 1px solid var(--guide-control);
  background: var(--guide-control);
  color: var(--guide-control-foreground);
  font-size: 0.75rem;
  font-weight: 700;
  transition: background-color 180ms ease, transform 180ms ease;
}

.mission-action:hover {
  border-color: var(--guide-control-hover);
  background: var(--guide-control-hover);
}

.mission-action:active {
  transform: translateY(1px);
}

.mission-complete {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: 0.25rem 1rem 1rem;
  padding: 0.75rem;
  border-radius: 6px;
  border: 1px solid var(--guide-accent-border);
  background: var(--guide-accent-soft);
  color: var(--guide-accent-text);
}

.mission-complete strong,
.mission-complete small {
  display: block;
}

.mission-complete strong {
  font-size: 0.8125rem;
}

.mission-complete small {
  margin-top: 0.15rem;
  color: var(--app-muted);
  font-size: 0.6875rem;
}

@keyframes mission-panel-in {
  from { opacity: 0; transform: translateY(10px) scale(0.98); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

@media (max-width: 640px) {
  .mission-dock {
    right: 0.75rem;
    bottom: 0.75rem;
    width: calc(100vw - 1.5rem);
  }
}

@media (prefers-reduced-motion: reduce) {
  .mission-panel {
    animation: none;
  }
}
</style>
