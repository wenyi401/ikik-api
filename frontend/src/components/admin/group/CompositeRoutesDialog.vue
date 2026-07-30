<template>
  <BaseDialog :show="show" :title="title" width="wide" @close="emit('close')">
    <div class="dialog-layout">
      <section class="routes-section">
        <div class="section-heading">
          <h4>{{ t('admin.groups.compositeRoutes.routes') }}</h4>
          <button class="icon-button" :disabled="loading" :title="t('common.refresh')" @click="loadRoutes">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          </button>
        </div>

        <div v-if="loading" class="empty-state"><Icon name="refresh" size="md" class="animate-spin" /></div>
        <div v-else-if="routes.length === 0" class="empty-state">
          {{ t('admin.groups.compositeRoutes.empty') }}
        </div>
        <div v-else class="routes-list">
          <article v-for="route in routes" :key="route.id" class="route-card">
            <div class="route-main">
              <div class="route-title-row">
                <strong>{{ route.public_model }}</strong>
                <span :class="['status-dot', route.enabled ? 'is-active' : '']" />
              </div>
              <div class="route-target">
                <PlatformIcon :platform="route.target_platform" size="xs" />
                <span>{{ route.target_platform }}</span>
                <Icon name="arrowRight" size="xs" />
                <code>{{ route.upstream_model || route.public_model }}</code>
              </div>
              <div class="route-meta">
                <span>{{ route.match_type }}</span><span>{{ endpointLabel(route.endpoint) }}</span><span>P{{ route.priority }}</span>
              </div>
            </div>
            <div class="route-actions">
              <button class="icon-button" :title="t('common.edit')" @click="editingRoute = route">
                <Icon name="edit" size="sm" />
              </button>
              <button class="icon-button danger" :title="t('common.delete')" @click="removeRoute(route)">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </article>
        </div>
      </section>

      <CompositeRouteForm
        :route="editingRoute"
        :saving="saving"
        @save="saveRoute"
        @cancel="editingRoute = null"
      />

      <section class="preview-section">
        <h4>{{ t('admin.groups.compositeRoutes.preview') }}</h4>
        <div class="preview-controls">
          <input v-model.trim="previewModel" class="preview-input" :placeholder="t('admin.groups.compositeRoutes.publicModel')" />
          <select v-model="previewEndpoint" class="preview-select">
            <option v-for="option in endpointOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
          <button class="secondary-button" :disabled="previewing || !previewModel" @click="preview">
            <Icon v-if="previewing" name="refresh" size="sm" class="animate-spin" />
            <span>{{ t('admin.groups.compositeRoutes.preview') }}</span>
          </button>
        </div>
        <div v-if="previewDecision" class="preview-result">
          <span :class="['result-state', previewDecision.matched ? 'matched' : '']">
            {{ previewDecision.matched ? t('admin.groups.compositeRoutes.matched') : t('admin.groups.compositeRoutes.notMatched') }}
          </span>
          <template v-if="previewDecision.matched">
            <span>{{ previewDecision.target_platform }}</span>
            <Icon name="arrowRight" size="xs" />
            <code>{{ previewDecision.upstream_model }}</code>
          </template>
          <span v-else-if="previewDecision.reason" class="reason">{{ previewDecision.reason }}</span>
        </div>
      </section>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import CompositeRouteForm from './CompositeRouteForm.vue'
import type {
  AdminGroup,
  CompositeModelRoute,
  CompositeModelRouteInput,
  CompositeRouteDecision,
  CompositeRouteEndpoint
} from '@/types'

const props = defineProps<{ show: boolean; group: AdminGroup | null }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const appStore = useAppStore()
const routes = ref<CompositeModelRoute[]>([])
const loading = ref(false)
const saving = ref(false)
const editingRoute = ref<CompositeModelRoute | null>(null)
const previewModel = ref('')
const previewEndpoint = ref<CompositeRouteEndpoint>('any')
const previewing = ref(false)
const previewDecision = ref<CompositeRouteDecision | null>(null)
const title = computed(() => props.group
  ? t('admin.groups.compositeRoutes.titleWithGroup', { name: props.group.name })
  : t('admin.groups.compositeRoutes.title'))

const endpointKeys: Array<{ value: CompositeRouteEndpoint; key: string }> = [
  { value: 'any', key: 'any' }, { value: 'messages', key: 'messages' },
  { value: 'count_tokens', key: 'countTokens' }, { value: 'responses', key: 'responses' },
  { value: 'chat_completions', key: 'chatCompletions' }, { value: 'embeddings', key: 'embeddings' },
  { value: 'images', key: 'images' }, { value: 'gemini', key: 'gemini' }
]
const endpointOptions = computed(() => endpointKeys.map((item) => ({
  value: item.value,
  label: t(`admin.groups.compositeRoutes.endpoints.${item.key}`)
})))
const endpointLabel = (value: CompositeRouteEndpoint) => endpointOptions.value.find((item) => item.value === value)?.label || value

const loadRoutes = async () => {
  if (!props.group) return
  loading.value = true
  try {
    routes.value = (await adminAPI.groups.listCompositeRoutes(props.group.id)).sort((a, b) => a.priority - b.priority || a.id - b.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.compositeRoutes.failedToLoad')))
  } finally {
    loading.value = false
  }
}

const saveRoute = async (input: CompositeModelRouteInput) => {
  if (!props.group) return
  saving.value = true
  try {
    if (editingRoute.value) {
      await adminAPI.groups.updateCompositeRoute(props.group.id, editingRoute.value.id, input)
      appStore.showSuccess(t('admin.groups.compositeRoutes.routeUpdated'))
    } else {
      await adminAPI.groups.createCompositeRoute(props.group.id, input)
      appStore.showSuccess(t('admin.groups.compositeRoutes.routeCreated'))
    }
    editingRoute.value = null
    await loadRoutes()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.compositeRoutes.failedToSave')))
  } finally {
    saving.value = false
  }
}

const removeRoute = async (route: CompositeModelRoute) => {
  if (!props.group || !window.confirm(t('admin.groups.compositeRoutes.deleteConfirm'))) return
  try {
    await adminAPI.groups.deleteCompositeRoute(props.group.id, route.id)
    if (editingRoute.value?.id === route.id) editingRoute.value = null
    appStore.showSuccess(t('admin.groups.compositeRoutes.routeDeleted'))
    await loadRoutes()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.compositeRoutes.failedToDelete')))
  }
}

const preview = async () => {
  if (!props.group || !previewModel.value) return
  previewing.value = true
  try {
    previewDecision.value = await adminAPI.groups.previewCompositeRoute(props.group.id, {
      model: previewModel.value,
      endpoint: previewEndpoint.value
    })
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.compositeRoutes.failedToPreview')))
  } finally {
    previewing.value = false
  }
}

watch(() => props.show, (show) => {
  if (show) {
    editingRoute.value = null
    previewDecision.value = null
    void loadRoutes()
  }
})
</script>

<style scoped>
.dialog-layout { display: grid; gap: 16px; }
.routes-section, .preview-section { border: 1px solid var(--app-border); border-radius: 12px; padding: 16px; background: var(--app-surface); }
.section-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.section-heading h4, .preview-section h4 { color: var(--app-text); font-size: 14px; font-weight: 600; }
.routes-list { display: grid; gap: 8px; max-height: 290px; overflow-y: auto; }
.route-card { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 13px 14px; border: 1px solid var(--app-border); border-radius: 10px; background: var(--app-bg); }
.route-main { min-width: 0; }
.route-title-row, .route-target, .route-meta, .route-actions, .preview-result { display: flex; align-items: center; }
.route-title-row { gap: 8px; color: var(--app-text); font-size: 13px; }
.status-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--app-muted); opacity: .45; }
.status-dot.is-active { background: #10a37f; opacity: 1; }
.route-target { min-width: 0; gap: 7px; margin-top: 7px; color: var(--app-muted); font-size: 12px; }
.route-target code, .preview-result code { overflow: hidden; color: var(--app-text); text-overflow: ellipsis; white-space: nowrap; }
.route-meta { gap: 10px; margin-top: 7px; color: var(--app-muted); font-size: 11px; }
.route-actions { flex: none; gap: 4px; }
.icon-button { display: inline-flex; width: 34px; height: 34px; align-items: center; justify-content: center; border-radius: 9px; color: var(--app-muted); }
.icon-button:hover { background: var(--app-hover); color: var(--app-text); }
.icon-button.danger:hover { color: #dc2626; }
.empty-state { display: flex; min-height: 86px; align-items: center; justify-content: center; color: var(--app-muted); font-size: 13px; }
.preview-section h4 { margin-bottom: 12px; }
.preview-controls { display: grid; grid-template-columns: minmax(0, 1fr) 180px auto; gap: 8px; }
.preview-input, .preview-select { min-height: 40px; min-width: 0; border: 1px solid var(--app-border); border-radius: 10px; background: var(--app-bg); color: var(--app-text); padding: 8px 10px; outline: none; }
.secondary-button { display: inline-flex; min-height: 40px; align-items: center; justify-content: center; gap: 7px; border: 1px solid var(--app-border); border-radius: 10px; padding: 0 14px; color: var(--app-text); font-size: 13px; font-weight: 600; }
.secondary-button:disabled { opacity: .45; }
.preview-result { flex-wrap: wrap; gap: 8px; margin-top: 12px; border-radius: 10px; background: var(--app-bg); padding: 11px 12px; color: var(--app-muted); font-size: 12px; }
.result-state { color: #b45309; font-weight: 600; }.result-state.matched { color: #0f8a6b; }.reason { min-width: 0; overflow-wrap: anywhere; }
@media (max-width: 640px) { .routes-section, .preview-section { padding: 13px; } .route-card { align-items: flex-start; } .preview-controls { grid-template-columns: 1fr; } .routes-list { max-height: 240px; } }
</style>
