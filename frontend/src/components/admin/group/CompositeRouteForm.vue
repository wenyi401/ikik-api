<template>
  <form class="route-form" @submit.prevent="submit">
    <div class="form-heading">
      <h4>{{ editing ? t('admin.groups.compositeRoutes.editRoute') : t('admin.groups.compositeRoutes.addRoute') }}</h4>
      <button v-if="editing" type="button" class="text-button" @click="emit('cancel')">
        {{ t('common.cancel') }}
      </button>
    </div>

    <div class="form-grid">
      <label class="field field-wide">
        <span>{{ t('admin.groups.compositeRoutes.publicModel') }}</span>
        <input v-model.trim="form.public_model" required class="input" autocomplete="off" />
      </label>

      <label class="field">
        <span>{{ t('admin.groups.compositeRoutes.matchType') }}</span>
        <select v-model="form.match_type" class="input">
          <option value="exact">{{ t('admin.groups.compositeRoutes.match.exact') }}</option>
          <option value="prefix">{{ t('admin.groups.compositeRoutes.match.prefix') }}</option>
        </select>
      </label>

      <label class="field">
        <span>{{ t('admin.groups.compositeRoutes.endpoint') }}</span>
        <select v-model="form.endpoint" class="input">
          <option v-for="option in endpointOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </label>

      <label class="field">
        <span>{{ t('admin.groups.compositeRoutes.targetPlatform') }}</span>
        <select v-model="form.target_platform" class="input">
          <option v-for="platform in targetPlatforms" :key="platform" :value="platform">
            {{ platformLabel(platform) }}
          </option>
        </select>
      </label>

      <label class="field">
        <span>{{ t('admin.groups.compositeRoutes.priority') }}</span>
        <input v-model.number="form.priority" type="number" min="1" class="input" />
      </label>

      <label class="field field-wide">
        <span>{{ t('admin.groups.compositeRoutes.upstreamModel') }}</span>
        <input v-model.trim="form.upstream_model" class="input" autocomplete="off" />
      </label>

      <label class="field field-wide">
        <span>{{ t('admin.groups.compositeRoutes.notes') }}</span>
        <input v-model.trim="form.notes" class="input" autocomplete="off" />
      </label>
    </div>

    <div class="form-actions">
      <label class="enabled-toggle">
        <input v-model="form.enabled" type="checkbox" />
        <span>{{ t('admin.groups.compositeRoutes.enabled') }}</span>
      </label>
      <button type="submit" class="primary-button" :disabled="saving || !form.public_model.trim()">
        <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
        <span>{{ editing ? t('common.update') : t('common.create') }}</span>
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { COMPOSITE_ROUTE_TARGET_OPTIONS, CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import type {
  CompositeModelRoute,
  CompositeModelRouteInput,
  CompositeRouteEndpoint,
  CompositeTargetPlatform
} from '@/types'

const props = defineProps<{
  route?: CompositeModelRoute | null
  saving?: boolean
}>()

const emit = defineEmits<{
  save: [input: CompositeModelRouteInput]
  cancel: []
}>()

const { t } = useI18n()
// 与后端 isConcreteRequestPlatform 对齐：复合分组可路由到所有具体平台
// （kiro / custom 除外），新增供应商会随平台目录自动出现。
const targetPlatforms = COMPOSITE_ROUTE_TARGET_OPTIONS.map((option) => option.value as CompositeTargetPlatform)
const endpointValues: CompositeRouteEndpoint[] = [
  'any',
  'messages',
  'count_tokens',
  'responses',
  'chat_completions',
  'embeddings',
  'images',
  'gemini'
]
const endpointOptions = endpointValues.map((value) => ({
  value,
  label: t(`admin.groups.compositeRoutes.endpoints.${value === 'count_tokens' ? 'countTokens' : value === 'chat_completions' ? 'chatCompletions' : value}`)
}))

const emptyForm = (): CompositeModelRouteInput => ({
  public_model: '',
  match_type: 'exact',
  target_platform: 'openai',
  upstream_model: '',
  endpoint: 'any',
  priority: 100,
  enabled: true,
  notes: ''
})

const form = reactive<CompositeModelRouteInput>(emptyForm())
const editing = computed(() => Boolean(props.route))

const reset = () => Object.assign(form, emptyForm())

watch(
  () => props.route,
  (route) => {
    if (!route) {
      reset()
      return
    }
    Object.assign(form, {
      public_model: route.public_model,
      match_type: route.match_type,
      target_platform: route.target_platform,
      upstream_model: route.upstream_model,
      endpoint: route.endpoint,
      priority: route.priority || 100,
      enabled: route.enabled,
      notes: route.notes || ''
    })
  },
  { immediate: true }
)

const platformLabel = (platform: string) =>
  CONCRETE_PLATFORM_OPTIONS.find((option) => option.value === platform)?.label ??
  platform.charAt(0).toUpperCase() + platform.slice(1)

const submit = () => {
  emit('save', {
    ...form,
    public_model: form.public_model.trim(),
    upstream_model: form.upstream_model?.trim(),
    notes: form.notes?.trim(),
    priority: Number(form.priority) || 100
  })
}
</script>

<style scoped>
.route-form { border: 1px solid var(--app-border); border-radius: 12px; padding: 18px; background: var(--app-surface); }
.form-heading, .form-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.form-heading { margin-bottom: 16px; }
.form-heading h4 { color: var(--app-text); font-size: 14px; font-weight: 600; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.field { min-width: 0; }
.field-wide { grid-column: 1 / -1; }
.field > span { display: block; margin-bottom: 6px; color: var(--app-muted); font-size: 12px; }
.input { width: 100%; min-height: 42px; border: 1px solid var(--app-border); border-radius: 10px; background: var(--app-bg); color: var(--app-text); padding: 9px 11px; outline: none; }
.input:focus { border-color: var(--app-text); box-shadow: 0 0 0 1px var(--app-text); }
.form-actions { margin-top: 16px; }
.enabled-toggle { display: inline-flex; align-items: center; gap: 8px; color: var(--app-text); font-size: 13px; }
.primary-button, .text-button { display: inline-flex; align-items: center; justify-content: center; gap: 7px; border-radius: 10px; font-size: 13px; font-weight: 600; }
.primary-button { min-height: 40px; padding: 0 16px; background: var(--app-text); color: var(--app-bg); }
.primary-button:disabled { cursor: not-allowed; opacity: .45; }
.text-button { color: var(--app-muted); }
@media (max-width: 640px) { .route-form { padding: 14px; } .form-grid { grid-template-columns: 1fr; } .field-wide { grid-column: auto; } }
</style>
