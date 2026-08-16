<template>
  <aside
    v-if="pet.initialized && pet.preferences.enabled"
    class="pet-widget"
    :class="[
      `pet-widget--${pet.preferences.anchor}`,
      {
        'pet-widget--avoid-right-dock': props.avoidRightDock && pet.preferences.anchor === 'bottom-right',
        'pet-widget--sidebar-collapsed': props.sidebarCollapsed,
        'pet-widget--dragging': dragging,
      },
    ]"
    :style="widgetStyle"
    :aria-label="t('pet.playgroundTitle')"
  >
    <section v-if="pet.panelOpen" class="pet-chat-panel">
      <header class="pet-chat-header">
        <div class="min-w-0">
          <h2>{{ t('pet.playgroundTitle') }}</h2>
          <p>{{ currentContextLabel }}</p>
        </div>
        <div class="pet-header-actions">
          <a class="pet-icon-button" href="https://docs.ikik.net" target="_blank" rel="noreferrer" :title="t('pet.openDocs')">
            <Icon name="book" size="sm" />
          </a>
          <button v-if="pet.messages.length" type="button" class="pet-icon-button" :title="t('pet.clearConversation')" :disabled="pet.asking" @click="pet.clearMessages">
            <Icon name="trash" size="sm" />
          </button>
          <RouterLink to="/pet" class="pet-icon-button" :title="t('pet.settings')">
            <Icon name="cog" size="sm" />
          </RouterLink>
          <button type="button" class="pet-icon-button" :title="t('common.close')" @click="pet.panelOpen = false">
            <Icon name="x" size="sm" />
          </button>
        </div>
      </header>

      <div class="pet-model-bar">
        <label>
          <span>{{ t('pet.group') }}</span>
          <select
            data-testid="pet-playground-group"
            :value="pet.preferences.assistant_group_id || ''"
            :disabled="groupsLoading || pet.asking"
            @change="selectGroup"
          >
            <option value="">{{ t('pet.selectAssistantGroup') }}</option>
            <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
          </select>
        </label>
        <label>
          <span>{{ t('pet.model') }}</span>
          <select
            data-testid="pet-playground-model"
            :value="pet.preferences.assistant_model"
            :disabled="modelsLoading || pet.asking || !pet.preferences.assistant_group_id"
            @change="selectModel"
          >
            <option value="">{{ modelsLoading ? t('common.loading') : t('pet.selectModel') }}</option>
            <option v-for="model in models" :key="model" :value="model">{{ model }}</option>
          </select>
        </label>
      </div>

      <div ref="messageList" class="pet-message-list" aria-live="polite">
        <div v-if="pet.messages.length === 0" class="pet-chat-empty">
          <Icon name="beaker" size="lg" />
          <strong>{{ t('pet.emptyPlaygroundTitle') }}</strong>
          <p>{{ pet.preferences.assistant_group_id ? t('pet.emptyPlayground') : t('pet.groupRequired') }}</p>
        </div>
        <article
          v-for="message in pet.messages"
          :key="message.id"
          class="pet-message"
          :class="`pet-message--${message.role}`"
        >
          <div v-if="message.role === 'user'" class="pet-message-text">{{ message.content }}</div>
          <template v-else>
            <details v-if="message.reasoning" class="pet-reasoning">
              <summary>{{ t('pet.reasoning') }}</summary>
              <p>{{ message.reasoning }}</p>
            </details>
            <div v-if="message.content" class="pet-markdown" v-html="renderMarkdown(message.content)" />
            <div v-else-if="message.status === 'streaming'" class="pet-thinking" role="status" :aria-label="t('pet.generating')">
              <span /><span /><span />
            </div>
            <p v-if="message.status === 'error'" class="pet-message-error">{{ message.error }}</p>
            <button
              v-if="message.content"
              type="button"
              class="pet-message-copy"
              :title="t('common.copy')"
              @click="copyMessage(message.content)"
            >
              <Icon name="copy" size="xs" />
            </button>
          </template>
        </article>
      </div>

      <details class="pet-parameters">
        <summary>
          <span><Icon name="beaker" size="sm" />{{ t('pet.requestParameters') }}</span>
          <Icon name="chevronDown" size="xs" />
        </summary>
        <div class="pet-parameters__body">
          <label class="pet-system-prompt">
            <span>{{ t('pet.systemPrompt') }}</span>
            <textarea v-model="systemPrompt" rows="2" :disabled="pet.asking" :placeholder="t('pet.systemPromptPlaceholder')" />
          </label>
          <label class="pet-parameter-row">
            <input v-model="temperatureEnabled" type="checkbox" :disabled="pet.asking" />
            <span>{{ t('pet.temperature') }}</span>
            <input v-model.number="temperature" type="number" min="0" max="2" step="0.1" :disabled="pet.asking || !temperatureEnabled" />
          </label>
          <label class="pet-parameter-row">
            <input v-model="topPEnabled" type="checkbox" :disabled="pet.asking" />
            <span>Top P</span>
            <input v-model.number="topP" type="number" min="0" max="1" step="0.05" :disabled="pet.asking || !topPEnabled" />
          </label>
          <label class="pet-parameter-row">
            <input v-model="maxTokensEnabled" type="checkbox" :disabled="pet.asking" />
            <span>{{ t('pet.maxTokens') }}</span>
            <input v-model.number="maxTokens" type="number" min="1" max="32768" step="1" :disabled="pet.asking || !maxTokensEnabled" />
          </label>
        </div>
      </details>

      <form class="pet-composer" @submit.prevent="submit">
        <textarea
          v-model="question"
          :placeholder="t('pet.playgroundPlaceholder')"
          rows="2"
          maxlength="12000"
          :disabled="!canCompose"
          @keydown.enter.exact.prevent="submit"
        />
        <button
          v-if="pet.asking"
          type="button"
          class="pet-send-button pet-send-button--stop"
          :title="t('pet.stop')"
          @click="pet.stopGeneration"
        >
          <span class="pet-stop-icon" />
        </button>
        <button v-else type="submit" class="pet-send-button" :disabled="!canSend" :title="t('pet.send')">
          <Icon name="arrowUp" size="sm" />
        </button>
      </form>
    </section>

    <Transition name="pet-bubble">
      <button
        v-if="pet.statusBubble && !pet.panelOpen"
        :key="pet.statusBubble.id"
        type="button"
        class="pet-status-bubble"
        :class="`pet-status-bubble--${pet.statusBubble.tone}`"
        @click="pet.panelOpen = true"
      >
        <span class="pet-status-dot" />
        {{ statusBubbleLabel }}
      </button>
    </Transition>

    <button
      ref="launcher"
      type="button"
      class="pet-launcher"
      :class="{ 'pet-launcher--fallback': !pet.spriteURL || !pet.selectedAsset }"
      :title="pet.panelOpen ? t('common.close') : t('pet.dragAssistant')"
      @pointerdown="startDrag"
      @pointermove="moveDrag"
      @pointerup="finishDrag"
      @pointercancel="finishDrag"
      @click="togglePanel"
    >
      <PetSprite
        v-if="pet.spriteURL && pet.selectedAsset"
        :asset="pet.selectedAsset"
        :src="pet.spriteURL"
        :state="pet.animation"
        :size="pet.preferences.size"
        :reduced-motion="pet.preferences.reduced_motion"
      />
      <Icon v-else name="chat" size="lg" />
    </button>
  </aside>
</template>

<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import type { Group } from '@/types'
import userGroupsAPI from '@/api/groups'
import { listPlaygroundModels } from '@/api/playground'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { usePetStore } from '@/stores/pet'
import { extractApiErrorMessage } from '@/utils/apiError'
import PetSprite from './PetSprite.vue'

const props = defineProps<{
  avoidRightDock?: boolean
  sidebarCollapsed?: boolean
}>()
const { t } = useI18n()
const pet = usePetStore()
const app = useAppStore()
const question = ref('')
const systemPrompt = ref('')
const temperatureEnabled = ref(false)
const temperature = ref(0.7)
const topPEnabled = ref(false)
const topP = ref(1)
const maxTokensEnabled = ref(false)
const maxTokens = ref(2048)
const groups = ref<Group[]>([])
const models = ref<string[]>([])
const groupsLoading = ref(false)
const modelsLoading = ref(false)
const messageList = ref<HTMLElement | null>(null)
const launcher = ref<HTMLElement | null>(null)
const dragging = ref(false)
const dragPosition = ref<{ x: number; y: number } | null>(null)
let suppressClick = false
let modelLoadGeneration = 0

interface DragState {
  pointerId: number
  startX: number
  startY: number
  originCenterX: number
  originBottom: number
  width: number
  height: number
  moved: boolean
}

let dragState: DragState | null = null

const selectedGroup = computed(() => groups.value.find((group) => group.id === pet.preferences.assistant_group_id))
const currentContextLabel = computed(() => {
  if (!selectedGroup.value) return t('pet.groupRequiredShort')
  return pet.preferences.assistant_model
    ? `${selectedGroup.value.name} · ${pet.preferences.assistant_model}`
    : selectedGroup.value.name
})
const canCompose = computed(() => Boolean(pet.preferences.assistant_group_id && pet.preferences.assistant_model))
const canSend = computed(() => canCompose.value && !pet.asking && Boolean(question.value.trim()))
const statusBubbleLabel = computed(() => {
  if (!pet.statusBubble) return ''
  return t(pet.statusBubble.labelKey, { count: pet.statusBubble.count || 1 })
})
const savedPosition = computed(() => {
  const x = pet.preferences.position_x
  const y = pet.preferences.position_y
  return x == null || y == null ? null : { x, y }
})
const widgetStyle = computed(() => {
  const position = dragPosition.value || savedPosition.value
  if (!position) return undefined
  return {
    left: `${position.x * 100}vw`,
    right: 'auto',
    top: 'auto',
    bottom: `${position.y * 100}vh`,
    transform: 'translateX(-50%)',
  }
})

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(Math.max(value, minimum), Math.max(minimum, maximum))
}

function startDrag(event: PointerEvent) {
  if (event.button !== 0 || !launcher.value) return
  const rect = launcher.value.getBoundingClientRect()
  dragState = {
    pointerId: event.pointerId,
    startX: event.clientX,
    startY: event.clientY,
    originCenterX: rect.left + rect.width / 2,
    originBottom: window.innerHeight - rect.bottom,
    width: rect.width,
    height: rect.height,
    moved: false,
  }
  launcher.value.setPointerCapture(event.pointerId)
}

function moveDrag(event: PointerEvent) {
  if (!dragState || event.pointerId !== dragState.pointerId) return
  const deltaX = event.clientX - dragState.startX
  const deltaY = event.clientY - dragState.startY
  if (!dragState.moved && Math.hypot(deltaX, deltaY) < 5) return
  dragState.moved = true
  dragging.value = true
  const padding = 8
  const centerX = clamp(
    dragState.originCenterX + deltaX,
    dragState.width / 2 + padding,
    window.innerWidth - dragState.width / 2 - padding,
  )
  const bottom = clamp(
    dragState.originBottom - deltaY,
    padding,
    window.innerHeight - dragState.height - padding,
  )
  dragPosition.value = { x: centerX / window.innerWidth, y: bottom / window.innerHeight }
}

function finishDrag(event: PointerEvent) {
  if (!dragState || event.pointerId !== dragState.pointerId) return
  if (launcher.value?.hasPointerCapture(event.pointerId)) launcher.value.releasePointerCapture(event.pointerId)
  const moved = dragState.moved
  dragState = null
  dragging.value = false
  if (!moved || !dragPosition.value) return
  suppressClick = true
  window.setTimeout(() => { suppressClick = false }, 0)
  const position = { ...dragPosition.value }
  const anchor = position.x < 0.5 ? 'bottom-left' : 'bottom-right'
  void pet.savePreferences({ ...pet.preferences, anchor, position_x: position.x, position_y: position.y })
    .catch((error) => {
      dragPosition.value = null
      app.showError(extractApiErrorMessage(error, t('pet.saveFailed')))
    })
    .finally(() => { dragPosition.value = null })
}

function togglePanel() {
  if (suppressClick) {
    suppressClick = false
    return
  }
  pet.panelOpen = !pet.panelOpen
}

async function loadGroups() {
  if (groupsLoading.value || groups.value.length) return
  groupsLoading.value = true
  try {
    groups.value = (await userGroupsAPI.getAvailable()).filter((group) => !group.claude_code_only)
  } catch (error) {
    app.showError(extractApiErrorMessage(error, t('pet.groupsLoadFailed')))
  } finally {
    groupsLoading.value = false
  }
}

async function loadModels(groupID: number) {
  const generation = ++modelLoadGeneration
  models.value = []
  if (!groupID) return
  modelsLoading.value = true
  try {
    const result = await listPlaygroundModels(groupID)
    if (generation !== modelLoadGeneration) return
    models.value = result.models
    const current = pet.preferences.assistant_model
    const next = result.models.includes(current) ? current : result.default_model
    if (next && next !== current) {
      await pet.savePreferences({ ...pet.preferences, assistant_model: next })
    }
  } catch (error) {
    if (generation === modelLoadGeneration) app.showError(extractApiErrorMessage(error, t('pet.modelsLoadFailed')))
  } finally {
    if (generation === modelLoadGeneration) modelsLoading.value = false
  }
}

async function ensurePlaygroundOptions() {
  await loadGroups()
  const groupID = pet.preferences.assistant_group_id || 0
  if (groupID && models.value.length === 0) await loadModels(groupID)
}

async function selectGroup(event: Event) {
  const groupID = Number((event.target as HTMLSelectElement).value)
  try {
    await pet.savePreferences({
      ...pet.preferences,
      assistant_group_id: groupID > 0 ? groupID : null,
      assistant_model: '',
    })
    await loadModels(groupID)
  } catch (error) {
    app.showError(extractApiErrorMessage(error, t('pet.saveFailed')))
  }
}

async function selectModel(event: Event) {
  const model = (event.target as HTMLSelectElement).value
  try {
    await pet.savePreferences({ ...pet.preferences, assistant_model: model })
  } catch (error) {
    app.showError(extractApiErrorMessage(error, t('pet.saveFailed')))
  }
}

async function submit() {
  const value = question.value.trim()
  if (!value || !canSend.value) return
  question.value = ''
  try {
    await pet.ask(value, {
      model: pet.preferences.assistant_model,
      systemPrompt: systemPrompt.value,
      temperature: temperatureEnabled.value ? clamp(temperature.value, 0, 2) : undefined,
      topP: topPEnabled.value ? clamp(topP.value, 0, 1) : undefined,
      maxTokens: maxTokensEnabled.value ? Math.round(clamp(maxTokens.value, 1, 32768)) : undefined,
    })
  } catch (error) {
    app.showError(extractApiErrorMessage(error, t('pet.askFailed')))
  }
}

function renderMarkdown(content: string): string {
  const html = marked.parse(content, { breaks: true, gfm: true }) as string
  return DOMPurify.sanitize(html)
}

async function copyMessage(content: string) {
  try { await navigator.clipboard.writeText(content) } catch { /* Clipboard permissions are optional. */ }
}

watch(() => pet.panelOpen, (open) => { if (open) void ensurePlaygroundOptions() })
watch(() => [pet.messages.length, pet.asking, pet.messages.at(-1)?.content], async () => {
  await nextTick()
  if (messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight
})

onMounted(() => {
  if (pet.panelOpen) void ensurePlaygroundOptions()
})
</script>

<style scoped>
.pet-widget { position: fixed; z-index: 45; bottom: 1rem; width: max-content; display: flex; flex-direction: column; align-items: flex-end; gap: 0.5rem; pointer-events: none; }
.pet-widget--bottom-left { left: calc(260px + 1rem); align-items: flex-start; }
.pet-widget--bottom-left.pet-widget--sidebar-collapsed { left: calc(64px + 1rem); }
.pet-widget--bottom-right { right: 1rem; }
.pet-widget--avoid-right-dock { bottom: 5.75rem; }
.pet-launcher, .pet-chat-panel, .pet-status-bubble { pointer-events: auto; }
.pet-launcher { display: grid; min-width: 4.75rem; min-height: 4.75rem; place-items: end center; padding: 0; border: 0; background: transparent; color: var(--app-text); cursor: grab; touch-action: none; user-select: none; }
.pet-launcher:active, .pet-widget--dragging .pet-launcher { cursor: grabbing; }
.pet-launcher--fallback { width: 3.25rem; min-width: 3.25rem; height: 3.25rem; min-height: 3.25rem; place-items: center; border: 1px solid var(--app-border); border-radius: 50%; background: var(--app-surface); box-shadow: 0 12px 26px rgba(15, 23, 42, 0.16); }
.pet-chat-panel { position: absolute; right: 0; bottom: calc(100% + 0.5rem); width: min(26rem, calc(100vw - 2rem)); height: min(36rem, calc(100vh - 7rem)); display: grid; grid-template-rows: auto auto minmax(0, 1fr) auto auto; overflow: hidden; border: 1px solid var(--app-border); border-radius: 8px; background: var(--app-surface); color: var(--app-text); box-shadow: 0 18px 48px rgba(15, 23, 42, 0.2); }
.pet-widget--bottom-left .pet-chat-panel { right: auto; left: 0; }
.pet-chat-header { min-height: 3.75rem; display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; padding: 0.7rem 0.75rem 0.65rem 0.875rem; border-bottom: 1px solid var(--app-border); }
.pet-chat-header h2 { font-size: 0.875rem; font-weight: 700; }
.pet-chat-header p { max-width: 15rem; margin-top: 0.125rem; overflow: hidden; color: var(--app-muted); font-size: 0.6875rem; text-overflow: ellipsis; white-space: nowrap; }
.pet-header-actions { display: flex; flex: none; align-items: center; gap: 0.125rem; }
.pet-icon-button { display: inline-flex; width: 1.875rem; height: 1.875rem; align-items: center; justify-content: center; border-radius: 5px; color: var(--app-muted-strong); }
.pet-icon-button:hover { background: var(--app-surface-muted); color: var(--app-text); }
.pet-icon-button:disabled { cursor: not-allowed; opacity: 0.4; }
.pet-model-bar { display: grid; grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr); gap: 0.5rem; padding: 0.625rem 0.75rem; border-bottom: 1px solid var(--app-border); background: var(--app-bg); }
.pet-model-bar label { min-width: 0; }
.pet-model-bar label > span { display: block; margin-bottom: 0.2rem; color: var(--app-muted); font-size: 0.625rem; font-weight: 600; }
.pet-model-bar select { width: 100%; height: 2rem; overflow: hidden; border: 1px solid var(--app-border); border-radius: 5px; background: var(--app-surface); padding: 0 1.5rem 0 0.5rem; color: var(--app-text); font-size: 0.6875rem; text-overflow: ellipsis; }
.pet-message-list { min-height: 0; overflow-y: auto; padding: 0.875rem; }
.pet-chat-empty { min-height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 0.45rem; padding: 2rem; color: var(--app-muted); text-align: center; }
.pet-chat-empty strong { color: var(--app-text); font-size: 0.8125rem; }
.pet-chat-empty p { max-width: 16rem; font-size: 0.75rem; line-height: 1.55; }
.pet-message { position: relative; max-width: 90%; margin-bottom: 0.875rem; font-size: 0.8125rem; line-height: 1.6; overflow-wrap: anywhere; }
.pet-message--user { margin-left: auto; padding: 0.55rem 0.7rem; border-radius: 8px 8px 2px 8px; background: #08775c; color: #fff; white-space: pre-wrap; }
.pet-message--assistant { margin-right: auto; padding-right: 1.5rem; color: var(--app-text); }
.pet-markdown :deep(p) { margin: 0 0 0.55rem; }
.pet-markdown :deep(p:last-child) { margin-bottom: 0; }
.pet-markdown :deep(pre) { max-width: 100%; overflow-x: auto; border: 1px solid var(--app-border); border-radius: 5px; background: var(--app-bg); padding: 0.625rem; font-size: 0.6875rem; }
.pet-markdown :deep(code) { font-size: 0.75em; }
.pet-markdown :deep(ul), .pet-markdown :deep(ol) { margin: 0.4rem 0; padding-left: 1.15rem; }
.pet-reasoning { margin-bottom: 0.5rem; color: var(--app-muted); font-size: 0.6875rem; }
.pet-reasoning summary { cursor: pointer; font-weight: 600; }
.pet-reasoning p { margin-top: 0.35rem; white-space: pre-wrap; }
.pet-message-copy { position: absolute; top: 0; right: 0; display: grid; width: 1.25rem; height: 1.25rem; place-items: center; border-radius: 4px; color: var(--app-muted); opacity: 0; }
.pet-message--assistant:hover .pet-message-copy, .pet-message-copy:focus-visible { opacity: 1; }
.pet-message-copy:hover { background: var(--app-surface-muted); color: var(--app-text); }
.pet-message-error { margin-top: 0.35rem; color: #c2413b; font-size: 0.6875rem; }
.pet-thinking { display: flex; gap: 0.25rem; padding: 0.5rem 0; }
.pet-thinking span { width: 0.35rem; height: 0.35rem; border-radius: 50%; background: var(--app-muted); animation: pet-pulse 1.1s ease-in-out infinite; }
.pet-thinking span:nth-child(2) { animation-delay: 140ms; }
.pet-thinking span:nth-child(3) { animation-delay: 280ms; }
.pet-parameters { border-top: 1px solid var(--app-border); background: var(--app-bg); }
.pet-parameters > summary { display: flex; min-height: 2rem; align-items: center; justify-content: space-between; padding: 0 0.75rem; color: var(--app-muted-strong); cursor: pointer; font-size: 0.6875rem; font-weight: 600; list-style: none; }
.pet-parameters > summary::-webkit-details-marker { display: none; }
.pet-parameters > summary span { display: inline-flex; align-items: center; gap: 0.35rem; }
.pet-parameters[open] > summary > svg { transform: rotate(180deg); }
.pet-parameters__body { max-height: 12rem; overflow-y: auto; display: grid; gap: 0.5rem; padding: 0.25rem 0.75rem 0.7rem; }
.pet-system-prompt > span { display: block; margin-bottom: 0.25rem; color: var(--app-muted); font-size: 0.625rem; font-weight: 600; }
.pet-system-prompt textarea { width: 100%; resize: vertical; border: 1px solid var(--app-border); border-radius: 5px; background: var(--app-surface); padding: 0.45rem 0.55rem; color: var(--app-text); font-size: 0.6875rem; outline: none; }
.pet-parameter-row { display: grid; grid-template-columns: 1rem minmax(0, 1fr) 5rem; align-items: center; gap: 0.45rem; color: var(--app-muted-strong); font-size: 0.6875rem; }
.pet-parameter-row input[type='checkbox'] { accent-color: #08775c; }
.pet-parameter-row input[type='number'] { width: 100%; height: 1.75rem; border: 1px solid var(--app-border); border-radius: 5px; background: var(--app-surface); padding: 0 0.4rem; color: var(--app-text); font-variant-numeric: tabular-nums; }
.pet-composer { display: grid; grid-template-columns: minmax(0, 1fr) 2.25rem; gap: 0.5rem; align-items: end; padding: 0.625rem; border-top: 1px solid var(--app-border); }
.pet-composer textarea { width: 100%; min-height: 2.5rem; max-height: 7rem; resize: none; border: 1px solid var(--app-border); border-radius: 6px; background: var(--app-bg); padding: 0.55rem 0.65rem; color: var(--app-text); font-size: 0.8125rem; outline: none; }
.pet-composer textarea:focus, .pet-system-prompt textarea:focus, .pet-model-bar select:focus { border-color: #08775c; box-shadow: 0 0 0 3px rgba(8, 119, 92, 0.14); }
.pet-composer textarea:disabled { cursor: not-allowed; opacity: 0.55; }
.pet-send-button { display: inline-flex; width: 2.25rem; height: 2.25rem; align-items: center; justify-content: center; border-radius: 6px; background: #08775c; color: #fff; }
.pet-send-button:disabled { cursor: not-allowed; opacity: 0.42; }
.pet-send-button--stop { background: color-mix(in srgb, #c2413b 12%, var(--app-surface)); color: #c2413b; }
.pet-stop-icon { width: 0.65rem; height: 0.65rem; border-radius: 2px; background: currentColor; }
.pet-status-bubble { position: relative; display: inline-flex; max-width: 16rem; min-height: 2rem; align-items: center; gap: 0.45rem; border: 1px solid var(--app-border); border-radius: 7px; background: var(--app-surface); padding: 0.4rem 0.65rem; color: var(--app-text); box-shadow: 0 10px 26px rgba(15, 23, 42, 0.15); font-size: 0.6875rem; font-weight: 600; text-align: left; }
.pet-status-bubble::after { position: absolute; right: 1.5rem; bottom: -0.3rem; width: 0.55rem; height: 0.55rem; border-right: 1px solid var(--app-border); border-bottom: 1px solid var(--app-border); background: var(--app-surface); content: ''; transform: rotate(45deg); }
.pet-widget--bottom-left .pet-status-bubble::after { right: auto; left: 1.5rem; }
.pet-status-dot { width: 0.45rem; height: 0.45rem; flex: none; border-radius: 50%; background: #08775c; }
.pet-status-bubble--success .pet-status-dot { background: #16835f; }
.pet-status-bubble--error .pet-status-dot { background: #c2413b; }
.pet-status-bubble--working .pet-status-dot { animation: pet-status-pulse 1.2s ease-in-out infinite; }
.pet-bubble-enter-active, .pet-bubble-leave-active { transition: opacity 180ms ease, transform 180ms ease; }
.pet-bubble-enter-from, .pet-bubble-leave-to { opacity: 0; transform: translateY(4px); }
@keyframes pet-pulse { 0%, 100% { opacity: 0.3; transform: translateY(0); } 50% { opacity: 1; transform: translateY(-2px); } }
@keyframes pet-status-pulse { 0%, 100% { opacity: 0.35; transform: scale(0.85); } 50% { opacity: 1; transform: scale(1); } }
@media (max-width: 640px) {
  .pet-widget { bottom: 0.75rem; z-index: 35; }
  .pet-widget--bottom-left { left: 0.75rem; }
  .pet-widget--bottom-right { right: 0.75rem; }
  .pet-widget--avoid-right-dock { bottom: 5.5rem; }
  .pet-chat-panel { width: calc(100vw - 1.5rem); height: min(34rem, calc(100dvh - 6rem)); }
  .pet-chat-header p { max-width: 10rem; }
  .pet-model-bar { grid-template-columns: 1fr 1fr; }
}
@media (prefers-reduced-motion: reduce) {
  .pet-thinking span, .pet-status-bubble--working .pet-status-dot { animation: none; }
  .pet-bubble-enter-active, .pet-bubble-leave-active { transition: none; }
}
</style>
