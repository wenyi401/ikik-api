<template>
  <div class="pet-sprite-stage" :style="stageStyle" aria-hidden="true">
    <Transition name="pet-action" :css="!props.reducedMotion">
      <div :key="spriteKey" class="pet-sprite-frame" :style="frameStyle" />
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { PetAsset } from '@/api/pet'
import type { PetAnimationState } from '@/stores/pet'
import {
  getPetAnimationPause,
  getPetPlaybackFrames,
  petAnimationSequences,
} from './animation'

const props = defineProps<{
  asset: PetAsset
  src: string
  state: PetAnimationState
  size: 'small' | 'medium' | 'large'
  reducedMotion: boolean
}>()

const frame = ref(0)
let animationFrame = 0
let frameTimer: ReturnType<typeof setTimeout> | undefined
let playbackGeneration = 0

const scale = computed(() => ({ small: 0.4, medium: 0.5, large: 0.65 })[props.size])
const sequence = computed(() => petAnimationSequences[props.state])
const spriteKey = computed(() => `${props.src}:${props.state}`)

const stageStyle = computed(() => ({
  width: `${192 * scale.value}px`,
  height: `${208 * scale.value}px`,
}))

const frameStyle = computed(() => {
  const cellWidth = 192 * scale.value
  const cellHeight = 208 * scale.value
  return {
    backgroundImage: `url("${props.src}")`,
    backgroundSize: `${1536 * scale.value}px ${props.asset.height * scale.value}px`,
    backgroundPosition: `${-frame.value * cellWidth}px ${-sequence.value.row * cellHeight}px`,
  }
})

function clearScheduler() {
  if (frameTimer) clearTimeout(frameTimer)
  if (animationFrame) cancelAnimationFrame(animationFrame)
  frameTimer = undefined
  animationFrame = 0
}

function schedule(delay: number, callback: () => void, generation: number) {
  frameTimer = setTimeout(() => {
    frameTimer = undefined
    animationFrame = requestAnimationFrame(() => {
      animationFrame = 0
      if (generation === playbackGeneration) callback()
    })
  }, delay)
}

function playCycle(playbackIndex: number, generation: number) {
  if (generation !== playbackGeneration) return
  const currentSequence = sequence.value
  const playbackFrames = getPetPlaybackFrames(currentSequence)
  const currentFrame = playbackFrames[playbackIndex] ?? 0
  frame.value = currentFrame
  const duration = currentSequence.durations[currentFrame] || currentSequence.durations[0]
  schedule(duration, () => {
    if (playbackIndex + 1 < playbackFrames.length) {
      playCycle(playbackIndex + 1, generation)
      return
    }
    if (currentSequence.playback === 'once') {
      frame.value = currentSequence.settleFrame ?? currentFrame
      return
    }
    frame.value = currentSequence.restFrame ?? 0
    const pause = getPetAnimationPause(currentSequence)
    if (pause > 0) schedule(pause, () => playCycle(0, generation), generation)
    else playCycle(0, generation)
  }, generation)
}

function start() {
  playbackGeneration += 1
  clearScheduler()
  const generation = playbackGeneration
  const currentSequence = sequence.value
  frame.value = currentSequence.restFrame ?? 0
  if (props.reducedMotion) return
  const initialPause = currentSequence.initialPause ? getPetAnimationPause(currentSequence) : 0
  if (initialPause > 0) schedule(initialPause, () => playCycle(0, generation), generation)
  else playCycle(0, generation)
}

watch(() => [props.state, props.reducedMotion, props.src] as const, start, { flush: 'sync' })
onMounted(start)
onBeforeUnmount(() => {
  playbackGeneration += 1
  clearScheduler()
})
</script>

<style scoped>
.pet-sprite-stage {
  position: relative;
  flex: none;
  contain: strict;
}
.pet-sprite-frame {
  position: absolute;
  inset: 0;
  contain: strict;
  background-repeat: no-repeat;
  image-rendering: auto;
  filter: drop-shadow(0 7px 7px rgba(0, 0, 0, 0.16));
  transform: translateZ(0);
  backface-visibility: hidden;
  will-change: background-position;
}
.pet-action-enter-active,
.pet-action-leave-active {
  transition: opacity 140ms cubic-bezier(0.2, 0.8, 0.2, 1);
}
.pet-action-enter-from,
.pet-action-leave-to {
  opacity: 0;
}
</style>
