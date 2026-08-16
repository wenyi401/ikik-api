import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  openPetActivityStream,
  petAPI,
  type PetActivityEvent,
  type PetAsset,
  type PetPreferences,
} from '@/api/pet'
import {
  streamPlaygroundChat,
  type PlaygroundChatRequest,
  type PlaygroundMessageInput,
} from '@/api/playground'
import { normalizePetSpritesheet } from '@/features/pet/spriteNormalizer'

export type PetAnimationState = 'idle' | 'running' | 'waving' | 'failed' | 'waiting' | 'review'
export type PetMessageStatus = 'streaming' | 'complete' | 'error'
export type PetBubbleTone = 'working' | 'success' | 'error'

export interface PetPlaygroundMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  reasoning: string
  status: PetMessageStatus
  error?: string
}

export interface PetPlaygroundOptions {
  model: string
  systemPrompt?: string
  temperature?: number
  topP?: number
  maxTokens?: number
}

export interface PetStatusBubble {
  id: number
  labelKey: string
  tone: PetBubbleTone
  count?: number
}

const defaultPreferences: PetPreferences = {
  selected_asset_id: 'baf63c4a-f555-5eaf-acb7-47eaabdd1382',
  assistant_group_id: null,
  assistant_model: '',
  enabled: false,
  size: 'medium',
  anchor: 'bottom-right',
  reduced_motion: false,
  activity_reactions: true,
  position_x: null,
  position_y: null,
}

const priority: Record<PetAnimationState, number> = {
  idle: 0,
  waving: 1,
  running: 2,
  waiting: 3,
  review: 4,
  failed: 5,
}

export const usePetStore = defineStore('pet', () => {
  const assets = ref<PetAsset[]>([])
  const preferences = ref<PetPreferences>({ ...defaultPreferences })
  const initialized = ref(false)
  const loading = ref(false)
  const panelOpen = ref(false)
  const animation = ref<PetAnimationState>('idle')
  const spriteURL = ref('')
  const messages = ref<PetPlaygroundMessage[]>([])
  const asking = ref(false)
  const activeRequests = ref(0)
  const statusBubble = ref<PetStatusBubble | null>(null)
  let animationTimer: ReturnType<typeof setTimeout> | undefined
  let bubbleTimer: ReturnType<typeof setTimeout> | undefined
  let streamController: AbortController | undefined
  let streamGeneration = 0
  let chatController: AbortController | undefined
  let spriteGeneration = 0
  let bubbleID = 0

  const selectedAsset = computed(() => (
    assets.value.find((asset) => asset.id === preferences.value.selected_asset_id) || null
  ))

  function setAnimation(next: PetAnimationState, holdMs = 0, force = false) {
    if (!force && priority[next] < priority[animation.value] && animationTimer) return
    if (animationTimer) clearTimeout(animationTimer)
    animationTimer = undefined
    animation.value = next
    if (holdMs > 0) {
      animationTimer = setTimeout(() => {
        animationTimer = undefined
        animation.value = activeRequests.value > 0 ? 'running' : 'idle'
      }, holdMs)
    }
  }

  function showStatusBubble(labelKey: string, tone: PetBubbleTone, holdMs = 0, count?: number) {
    if (bubbleTimer) clearTimeout(bubbleTimer)
    bubbleTimer = undefined
    statusBubble.value = { id: ++bubbleID, labelKey, tone, count }
    if (holdMs > 0) {
      bubbleTimer = setTimeout(() => {
        bubbleTimer = undefined
        statusBubble.value = null
      }, holdMs)
    }
  }

  function react(event: PetActivityEvent) {
    if (!preferences.value.activity_reactions && event.type === 'api.request') return
    if (event.type === 'api.request') {
      if (event.status === 'started') {
        activeRequests.value += 1
        setAnimation('running', 30_000, true)
        showStatusBubble('pet.bubbles.processing', 'working', 0, activeRequests.value)
        return
      }
      activeRequests.value = Math.max(0, activeRequests.value - 1)
      if (event.status === 'failed') {
        setAnimation('failed', 3200, true)
        showStatusBubble('pet.bubbles.failed', 'error', 3600)
      } else if (activeRequests.value > 0) {
        setAnimation('running', 30_000, true)
        showStatusBubble('pet.bubbles.processing', 'working', 0, activeRequests.value)
      } else {
        setAnimation('waving', 1800, true)
        showStatusBubble('pet.bubbles.completed', 'success', 2600)
      }
      return
    }
    if (event.type === 'assistant.thinking') {
      setAnimation('review', 60_000, true)
      showStatusBubble('pet.bubbles.thinking', 'working')
    } else if (event.type === 'assistant.error') {
      setAnimation('failed', 3200, true)
      showStatusBubble('pet.bubbles.failed', 'error', 3600)
    } else if (event.type === 'assistant.done') {
      setAnimation('waving', 1800, true)
      showStatusBubble('pet.bubbles.completed', 'success', 2600)
    }
  }

  async function loadSprite() {
    const generation = ++spriteGeneration
    if (spriteURL.value) URL.revokeObjectURL(spriteURL.value)
    spriteURL.value = ''
    if (!selectedAsset.value) return
    const blob = await petAPI.fetchSpritesheet(selectedAsset.value)
    let displayBlob = blob
    try {
      displayBlob = await normalizePetSpritesheet(blob, selectedAsset.value)
    } catch {
      displayBlob = blob
    }
    if (generation !== spriteGeneration) return
    spriteURL.value = URL.createObjectURL(displayBlob)
  }

  async function initialize() {
    if (loading.value) return
    loading.value = true
    try {
      const [nextAssets, nextPreferences] = await Promise.all([
        petAPI.listAssets(),
        petAPI.getPreferences(),
      ])
      assets.value = nextAssets
      preferences.value = { ...defaultPreferences, ...nextPreferences }
      if (!preferences.value.selected_asset_id && assets.value.length > 0) {
        preferences.value.selected_asset_id = assets.value[0].id
      }
      initialized.value = true
      try {
        await loadSprite()
      } catch {
        // Keep the assistant available with its fallback launcher when a
        // remote catalog image is temporarily unavailable.
        spriteURL.value = ''
      }
      startActivityStream()
    } finally {
      loading.value = false
    }
  }

  async function savePreferences(next: PetPreferences) {
    const previousAsset = preferences.value.selected_asset_id
    preferences.value = await petAPI.savePreferences(next)
    if (preferences.value.selected_asset_id !== previousAsset) await loadSprite()
  }

  async function importAsset(file: File) {
    const asset = await petAPI.importAsset(file)
    assets.value = [asset, ...assets.value.filter((item) => item.id !== asset.id)]
    await savePreferences({ ...preferences.value, selected_asset_id: asset.id, enabled: true })
    return asset
  }

  async function removeAsset(id: string) {
    await petAPI.deleteAsset(id)
    assets.value = assets.value.filter((asset) => asset.id !== id)
    if (preferences.value.selected_asset_id === id) {
      await savePreferences({ ...preferences.value, selected_asset_id: assets.value[0]?.id || null })
    }
  }

  async function ask(question: string, options: PetPlaygroundOptions) {
    const content = question.trim()
    const groupID = preferences.value.assistant_group_id
    const model = options.model.trim()
    if (!content || asking.value || !groupID || !model) return
    const history: PlaygroundMessageInput[] = messages.value
      .filter((message) => message.status === 'complete' && message.content.trim())
      .map((message) => ({ role: message.role, content: message.content }))
    const userMessage: PetPlaygroundMessage = {
      id: crypto.randomUUID(), role: 'user', content, reasoning: '', status: 'complete',
    }
    const assistantMessage: PetPlaygroundMessage = {
      id: crypto.randomUUID(), role: 'assistant', content: '', reasoning: '', status: 'streaming',
    }
    const requestMessages: PlaygroundMessageInput[] = []
    if (options.systemPrompt?.trim()) {
      requestMessages.push({ role: 'system', content: options.systemPrompt.trim() })
    }
    requestMessages.push(...history, { role: 'user', content })
    const request: PlaygroundChatRequest = {
      group_id: groupID,
      model,
      messages: requestMessages,
      ...(options.temperature == null ? {} : { temperature: options.temperature }),
      ...(options.topP == null ? {} : { top_p: options.topP }),
      ...(options.maxTokens == null ? {} : { max_tokens: options.maxTokens }),
    }
    asking.value = true
    setAnimation('review', 60_000, true)
    showStatusBubble('pet.bubbles.thinking', 'working')
    messages.value.push(userMessage, assistantMessage)
    const responseMessage = messages.value[messages.value.length - 1]
    chatController = new AbortController()
    try {
      await streamPlaygroundChat(request, (type, chunk) => {
        if (type === 'reasoning') responseMessage.reasoning += chunk
        else responseMessage.content += chunk
      }, chatController.signal)
      if (!responseMessage.content.trim() && responseMessage.reasoning.trim()) {
        responseMessage.content = responseMessage.reasoning
        responseMessage.reasoning = ''
      }
      if (!responseMessage.content.trim()) throw new Error('Playground returned an empty response')
      responseMessage.status = 'complete'
      setAnimation('waving', 1800, true)
      showStatusBubble('pet.bubbles.completed', 'success', 2600)
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        if (!responseMessage.content.trim()) {
          messages.value = messages.value.filter((message) => message.id !== responseMessage.id)
        } else {
          responseMessage.status = 'complete'
        }
        setAnimation('idle', 0, true)
        statusBubble.value = null
        return
      }
      responseMessage.status = 'error'
      responseMessage.error = error instanceof Error ? error.message : String(error)
      setAnimation('failed', 3200, true)
      showStatusBubble('pet.bubbles.failed', 'error', 3600)
      throw error
    } finally {
      chatController = undefined
      asking.value = false
    }
  }

  function stopGeneration() {
    chatController?.abort()
  }

  function clearMessages() {
    if (asking.value) return
    messages.value = []
  }

  function startActivityStream() {
    stopActivityStream()
    const generation = ++streamGeneration
    streamController = new AbortController()
    void consumeActivityStream(generation, streamController.signal)
  }

  async function consumeActivityStream(generation: number, signal: AbortSignal) {
    let retryMs = 1000
    while (!signal.aborted && generation === streamGeneration) {
      try {
        const response = await openPetActivityStream(signal)
        if (!response.ok || !response.body) throw new Error(`Activity stream returned ${response.status}`)
        retryMs = 1000
        const reader = response.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''
        while (!signal.aborted) {
          const { value, done } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')
          let boundary = buffer.indexOf('\n\n')
          while (boundary >= 0) {
            const block = buffer.slice(0, boundary)
            buffer = buffer.slice(boundary + 2)
            const data = block.split('\n').filter((line) => line.startsWith('data:')).map((line) => line.slice(5).trim()).join('\n')
            if (data) {
              try { react(JSON.parse(data) as PetActivityEvent) } catch { /* ignore malformed event */ }
            }
            boundary = buffer.indexOf('\n\n')
          }
        }
      } catch (error) {
        if (signal.aborted) return
      }
      await new Promise((resolve) => setTimeout(resolve, retryMs))
      retryMs = Math.min(retryMs * 2, 15_000)
    }
  }

  function stopActivityStream() {
    streamGeneration += 1
    streamController?.abort()
    streamController = undefined
  }

  function dispose() {
    stopActivityStream()
    chatController?.abort()
    if (animationTimer) clearTimeout(animationTimer)
    if (bubbleTimer) clearTimeout(bubbleTimer)
    if (spriteURL.value) URL.revokeObjectURL(spriteURL.value)
    spriteURL.value = ''
    statusBubble.value = null
    initialized.value = false
  }

  return {
    assets, preferences, selectedAsset, initialized, loading, panelOpen, animation,
    spriteURL, messages, asking, statusBubble, initialize, savePreferences,
    importAsset, removeAsset, ask, stopGeneration, clearMessages,
    startActivityStream, stopActivityStream, dispose,
  }
})
