<template>
  <Teleport to="body">
    <div v-if="show && position">
      <div class="fixed inset-0 z-[9998]" @click="emit('close')"></div>
      <div
        class="fixed z-[9999] w-52 overflow-hidden rounded-xl bg-white shadow-lg ring-1 ring-black/5 dark:bg-dark-800"
        :style="{ top: position.top + 'px', left: position.left + 'px' }"
        @click.stop
      >
        <div v-if="account" class="py-1">
          <button class="menu-item" @click="emitAction('test')">
            <Icon name="play" size="sm" class="text-green-500" :stroke-width="2" />
            {{ t('admin.accounts.testConnection') }}
          </button>
          <button class="menu-item" @click="emitAction('stats')">
            <Icon name="chart" size="sm" class="text-indigo-500" />
            {{ t('admin.accounts.viewStats') }}
          </button>
          <template v-if="account.type === 'oauth' || account.type === 'setup-token'">
            <button class="menu-item text-blue-600" @click="emitAction('reauth')">
              <Icon name="link" size="sm" />
              {{ t('admin.accounts.reAuthorize') }}
            </button>
            <button class="menu-item text-purple-600" @click="emitAction('refresh-token')">
              <Icon name="refresh" size="sm" />
              {{ t('admin.accounts.refreshToken') }}
            </button>
          </template>
          <button v-if="supportsPrivacy" class="menu-item text-emerald-600" @click="emitAction('set-privacy')">
            <Icon name="shield" size="sm" />
            {{ t('admin.accounts.setPrivacy') }}
          </button>
          <div v-if="hasRecoverableState" class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
          <button v-if="hasRecoverableState" class="menu-item text-emerald-600" @click="emitAction('recover-state')">
            <Icon name="sync" size="sm" />
            {{ t('admin.accounts.recoverState') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { Account } from '@/types'

const props = defineProps<{
  show: boolean
  account: Account | null
  position: { top: number; left: number } | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'test', account: Account): void
  (e: 'stats', account: Account): void
  (e: 'reauth', account: Account): void
  (e: 'refresh-token', account: Account): void
  (e: 'set-privacy', account: Account): void
  (e: 'recover-state', account: Account): void
}>()

const { t } = useI18n()

const supportsPrivacy = computed(() => {
  return (
    props.account?.type === 'oauth' &&
    (props.account.platform === 'openai' || props.account.platform === 'antigravity')
  )
})

const isRateLimited = computed(() => {
  if (props.account?.rate_limit_reset_at && new Date(props.account.rate_limit_reset_at) > new Date()) {
    return true
  }
  const modelLimits = (props.account?.extra as Record<string, unknown> | undefined)?.model_rate_limits as
    | Record<string, { rate_limit_reset_at: string }>
    | undefined
  if (!modelLimits) return false
  const now = new Date()
  return Object.values(modelLimits).some((info) => new Date(info.rate_limit_reset_at) > now)
})

const isOverloaded = computed(() =>
  Boolean(props.account?.overload_until && new Date(props.account.overload_until) > new Date())
)

const isTempUnschedulable = computed(() =>
  Boolean(
    props.account?.temp_unschedulable_until &&
      new Date(props.account.temp_unschedulable_until) > new Date()
  )
)

const hasRecoverableState = computed(() =>
  props.account?.status === 'error' ||
  isRateLimited.value ||
  isOverloaded.value ||
  isTempUnschedulable.value
)

function emitAction(event: 'test' | 'stats' | 'reauth' | 'refresh-token' | 'set-privacy' | 'recover-state'): void {
  if (!props.account) return
  switch (event) {
    case 'test':
      emit('test', props.account)
      break
    case 'stats':
      emit('stats', props.account)
      break
    case 'reauth':
      emit('reauth', props.account)
      break
    case 'refresh-token':
      emit('refresh-token', props.account)
      break
    case 'set-privacy':
      emit('set-privacy', props.account)
      break
    case 'recover-state':
      emit('recover-state', props.account)
      break
  }
  emit('close')
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') emit('close')
}

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      window.addEventListener('keydown', handleKeydown)
    } else {
      window.removeEventListener('keydown', handleKeydown)
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<style scoped>
.menu-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  text-align: left;
  font-size: 0.875rem;
  transition: background-color 0.15s ease;
}

.menu-item:hover {
  background: rgb(243 244 246);
}

:global(.dark) .menu-item:hover {
  background: rgb(55 65 81);
}
</style>
