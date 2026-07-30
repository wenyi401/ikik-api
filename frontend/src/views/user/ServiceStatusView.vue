<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <UiPageHeader :title="t('serviceStatus.title')" :description="t('serviceStatus.description')" />

      <OpenAIStatusPanel
        :snapshot="openAIStatus"
        :loading="statusLoading"
        :error="statusError"
        @refresh="loadOpenAIStatus"
      />

      <ProviderStatusPanel
        :snapshot="providerStatus"
        :loading="providerStatusLoading"
        :error="providerStatusError"
        @refresh="loadProviderStatus"
      />

      <EndpointDiagnosticsPanel
        :targets="endpointTargets"
        :results="results"
        :testing-all="testingAll"
        :result-for="resultFor"
        @test="test"
        @test-all="testAll(endpointTargets)"
      />
    </UiPage>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import OpenAIStatusPanel from '@/components/user/service-status/OpenAIStatusPanel.vue'
import ProviderStatusPanel from '@/components/user/service-status/ProviderStatusPanel.vue'
import EndpointDiagnosticsPanel from '@/components/user/service-status/EndpointDiagnosticsPanel.vue'
import { UiPage, UiPageHeader } from '@/ui'
import {
  getOpenAIServiceStatus,
  getProviderServiceStatus,
  type OpenAIStatusSnapshot,
  type ProviderStatusSnapshot,
} from '@/api/serviceStatus'
import { useEndpointDiagnostics } from '@/composables/useEndpointDiagnostics'
import { buildServiceEndpointTargets } from '@/utils/serviceStatus'

const { t } = useI18n()
const appStore = useAppStore()
const openAIStatus = ref<OpenAIStatusSnapshot | null>(null)
const statusLoading = ref(false)
const statusError = ref(false)
let statusController: AbortController | null = null
const providerStatus = ref<ProviderStatusSnapshot | null>(null)
const providerStatusLoading = ref(false)
const providerStatusError = ref(false)
let providerStatusController: AbortController | null = null

const { results, resultFor, test, testAll, testingAll } = useEndpointDiagnostics()

const endpointTargets = computed(() =>
  buildServiceEndpointTargets(
    appStore.cachedPublicSettings?.api_base_url,
    appStore.cachedPublicSettings?.custom_endpoints,
    window.location.origin,
  ),
)

async function loadOpenAIStatus() {
  statusController?.abort()
  const controller = new AbortController()
  statusController = controller
  statusLoading.value = true
  statusError.value = false
  try {
    openAIStatus.value = await getOpenAIServiceStatus({ signal: controller.signal })
  } catch (error) {
    if (controller.signal.aborted) return
    statusError.value = true
  } finally {
    if (statusController === controller) {
      statusLoading.value = false
      statusController = null
    }
  }
}

async function loadProviderStatus() {
  providerStatusController?.abort()
  const controller = new AbortController()
  providerStatusController = controller
  providerStatusLoading.value = true
  providerStatusError.value = false
  try {
    providerStatus.value = await getProviderServiceStatus({ signal: controller.signal })
  } catch (error) {
    if (controller.signal.aborted) return
    providerStatusError.value = true
  } finally {
    if (providerStatusController === controller) {
      providerStatusLoading.value = false
      providerStatusController = null
    }
  }
}

onMounted(async () => {
  const settingsPromise = appStore.fetchPublicSettings()
  const statusPromise = Promise.all([loadOpenAIStatus(), loadProviderStatus()])
  await settingsPromise
  await Promise.all([statusPromise, testAll(endpointTargets.value)])
})

onBeforeUnmount(() => {
  statusController?.abort()
  providerStatusController?.abort()
})
</script>
