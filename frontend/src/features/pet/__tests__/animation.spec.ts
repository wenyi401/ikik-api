import { describe, expect, it } from 'vitest'
import {
  getPetAnimationPause,
  getPetPlaybackFrames,
  petAnimationSequences,
} from '../animation'

describe('pet animation sequences', () => {
  it('uses only populated Codex Pet cells for standard states', () => {
    expect(petAnimationSequences.idle.durations).toHaveLength(6)
    expect(petAnimationSequences.waving.durations).toHaveLength(4)
    expect(petAnimationSequences.failed.durations).toHaveLength(8)
    expect(petAnimationSequences.waiting.durations).toHaveLength(6)
    expect(petAnimationSequences.running.durations).toHaveLength(6)
    expect(petAnimationSequences.review.durations).toHaveLength(6)
  })

  it('maps work and support states to their standard atlas rows', () => {
    expect(petAnimationSequences.running.row).toBe(7)
    expect(petAnimationSequences.review.row).toBe(8)
    expect(petAnimationSequences.failed.row).toBe(5)
  })

  it('gives ambient actions a natural rest instead of looping continuously', () => {
    const idle = petAnimationSequences.idle
    expect(idle.playback).toBe('ambient')
    expect(idle.initialPause).toBe(true)
    expect(getPetAnimationPause(idle, 0)).toBe(2800)
    expect(getPetAnimationPause(idle, 1)).toBe(5600)
  })

  it('returns non-cyclic poses smoothly and plays reactions only once', () => {
    expect(getPetPlaybackFrames(petAnimationSequences.idle)).toEqual([
      0, 1, 2, 3, 4, 5, 4, 3, 2, 1,
    ])
    expect(getPetPlaybackFrames(petAnimationSequences.waving)).toEqual([
      0, 1, 2, 3, 2, 1, 0,
    ])
    expect(petAnimationSequences.waving.playback).toBe('once')
    expect(petAnimationSequences.failed.playback).toBe('once')
  })

  it('keeps every playback frame inside its populated row cells', () => {
    for (const sequence of Object.values(petAnimationSequences)) {
      expect(getPetPlaybackFrames(sequence).every((frame) => (
        frame >= 0 && frame < sequence.durations.length
      ))).toBe(true)
    }
  })
})
