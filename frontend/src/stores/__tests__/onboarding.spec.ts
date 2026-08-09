import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useOnboardingStore } from '@/stores/onboarding'

describe('useOnboardingStore beginner journey', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  it('persists completed missions per user', () => {
    const store = useOnboardingStore()
    store.initializeJourney(7)
    store.completeMission('groups')
    store.completeMission('monitor')

    expect(store.completedMissions).toEqual(['groups', 'monitor'])
    expect(store.currentMission).toBe('key')
    expect(store.journeyProgress).toBe(0.4)

    setActivePinia(createPinia())
    const restored = useOnboardingStore()
    restored.initializeJourney(7)
    expect(restored.completedMissions).toEqual(['groups', 'monitor'])
  })

  it('keeps journey progress isolated between users', () => {
    const store = useOnboardingStore()
    store.initializeJourney(7)
    store.completeMission('groups')
    store.initializeJourney(8)

    expect(store.completedMissions).toEqual([])
    expect(store.currentMission).toBe('groups')
  })
})
