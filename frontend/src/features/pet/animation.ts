import type { PetAnimationState } from '@/stores/pet'

export interface PetAnimationSequence {
  row: number
  durations: readonly number[]
  frames?: readonly number[]
  playback: 'ambient' | 'loop' | 'once'
  pauseRange?: readonly [number, number]
  initialPause?: boolean
  restFrame?: number
  settleFrame?: number
}

export const petAnimationSequences: Record<PetAnimationState, PetAnimationSequence> = {
  idle: {
    row: 0,
    durations: [360, 180, 180, 210, 240, 420],
    frames: [0, 1, 2, 3, 4, 5, 4, 3, 2, 1],
    playback: 'ambient',
    pauseRange: [2800, 5600],
    initialPause: true,
    restFrame: 0,
  },
  waving: {
    row: 3,
    durations: [220, 180, 180, 320],
    frames: [0, 1, 2, 3, 2, 1, 0],
    playback: 'once',
    settleFrame: 0,
  },
  failed: {
    row: 5,
    durations: [200, 180, 180, 180, 180, 180, 180, 320],
    playback: 'once',
    settleFrame: 7,
  },
  waiting: {
    row: 6,
    durations: [220, 200, 200, 200, 200, 360],
    frames: [0, 1, 2, 3, 4, 5, 4, 3, 2, 1],
    playback: 'loop',
    pauseRange: [700, 1300],
    restFrame: 0,
  },
  running: {
    row: 7,
    durations: [160, 160, 160, 160, 160, 280],
    playback: 'loop',
    pauseRange: [260, 520],
    restFrame: 0,
  },
  review: {
    row: 8,
    durations: [220, 200, 200, 200, 200, 360],
    frames: [0, 1, 2, 3, 4, 5, 4, 3, 2, 1],
    playback: 'loop',
    pauseRange: [650, 1150],
    restFrame: 0,
  },
}

export function getPetPlaybackFrames(sequence: PetAnimationSequence): readonly number[] {
  return sequence.frames || sequence.durations.map((_, index) => index)
}

export function getPetAnimationPause(
  sequence: PetAnimationSequence,
  sample = Math.random(),
): number {
  if (!sequence.pauseRange) return 0
  const [minimum, maximum] = sequence.pauseRange
  const normalizedSample = Math.min(1, Math.max(0, sample))
  return Math.round(minimum + (maximum - minimum) * normalizedSample)
}
