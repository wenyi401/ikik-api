/**
 * Onboarding Store
 * Manages onboarding tour state and control methods
 */

import { defineStore } from 'pinia'
import { computed, markRaw, ref, shallowRef } from 'vue'
import type { Driver } from 'driver.js'

type VoidCallback = () => void
type NextStepCallback = (delay?: number) => Promise<void>
type IsCurrentStepCallback = (selector: string) => boolean

export const beginnerMissionIds = [
  'groups',
  'monitor',
  'key',
  'client',
  'first_call'
] as const

export type BeginnerMissionId = typeof beginnerMissionIds[number]

export const useOnboardingStore = defineStore('onboarding', () => {
  const replayCallback = ref<VoidCallback | null>(null)
  const nextStepCallback = ref<NextStepCallback | null>(null)
  const isCurrentStepCallback = ref<IsCurrentStepCallback | null>(null)
  const journeyUserID = ref<number | null>(null)
  const completedMissions = ref<BeginnerMissionId[]>([])
  const missionPanelOpen = ref(false)

  // 全局 driver 实例，跨组件保持
  const driverInstance = shallowRef<Driver | null>(null)
  const currentMission = computed<BeginnerMissionId | null>(() => (
    beginnerMissionIds.find((id) => !completedMissions.value.includes(id)) ?? null
  ))
  const journeyProgress = computed(() => completedMissions.value.length / beginnerMissionIds.length)
  const journeyComplete = computed(() => currentMission.value === null)

  function journeyStorageKey(userID: number): string {
    return `ikik_beginner_journey_${userID}_v1`
  }

  function persistJourney(): void {
    if (journeyUserID.value === null) return
    localStorage.setItem(journeyStorageKey(journeyUserID.value), JSON.stringify(completedMissions.value))
  }

  function initializeJourney(userID: number): void {
    if (journeyUserID.value === userID) return
    journeyUserID.value = userID
    const validMissionIDs = new Set<BeginnerMissionId>(beginnerMissionIds)
    try {
      const stored = JSON.parse(localStorage.getItem(journeyStorageKey(userID)) ?? '[]') as unknown
      completedMissions.value = Array.isArray(stored)
        ? stored.filter((id): id is BeginnerMissionId => validMissionIDs.has(id as BeginnerMissionId))
        : []
    } catch {
      completedMissions.value = []
    }
  }

  function completeMission(id: BeginnerMissionId): void {
    if (completedMissions.value.includes(id)) return
    completedMissions.value = [...completedMissions.value, id]
    persistJourney()
  }

  function resetJourney(): void {
    completedMissions.value = []
    missionPanelOpen.value = true
    persistJourney()
  }

  function setMissionPanelOpen(open: boolean): void {
    missionPanelOpen.value = open
  }

  function setReplayCallback(callback: VoidCallback | null): void {
    replayCallback.value = callback
  }

  function setControlMethods(methods: {
    nextStep: NextStepCallback,
    isCurrentStep: IsCurrentStepCallback
  }): void {
    nextStepCallback.value = methods.nextStep
    isCurrentStepCallback.value = methods.isCurrentStep
  }

  function clearControlMethods(): void {
    nextStepCallback.value = null
    isCurrentStepCallback.value = null
  }

  function setDriverInstance(driver: Driver | null): void {
    driverInstance.value = driver ? markRaw(driver) : null
  }

  function getDriverInstance(): Driver | null {
    return driverInstance.value
  }

  function isDriverActive(): boolean {
    return driverInstance.value?.isActive?.() ?? false
  }

  function replay(): void {
    if (replayCallback.value) {
      replayCallback.value()
    }
  }

  /**
   * Manually advance to the next step
   * @param delay Optional delay in ms (useful for waiting for animations)
   */
  async function nextStep(delay = 0): Promise<void> {
    if (nextStepCallback.value) {
      await nextStepCallback.value(delay)
    }
  }

  /**
   * Check if the tour is currently highlighting a specific element
   */
  function isCurrentStep(selector: string): boolean {
    if (isCurrentStepCallback.value) {
      return isCurrentStepCallback.value(selector)
    }
    return false
  }

  return {
    setReplayCallback,
    setControlMethods,
    clearControlMethods,
    setDriverInstance,
    getDriverInstance,
    isDriverActive,
    replay,
    nextStep,
    isCurrentStep,
    beginnerMissionIds,
    completedMissions,
    currentMission,
    journeyProgress,
    journeyComplete,
    missionPanelOpen,
    initializeJourney,
    completeMission,
    resetJourney,
    setMissionPanelOpen
  }
})
