<template>
  <AppLayout>
    <UiPage width="wide">
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-[var(--ui-border-strong)] border-t-[var(--ui-text)]"></div>
      </div>

      <EmptyState
        v-else-if="subscriptions.length === 0"
        :title="t('userSubscriptions.noActiveSubscriptions')"
        :description="t('userSubscriptions.noActiveSubscriptionsDesc')"
        :action-text="t('nav.buySubscription')"
        action-to="/purchase"
      />

      <div v-else class="subscription-list">
        <SubscriptionUsagePanel
          v-for="subscription in subscriptions"
          :key="subscription.id"
          :subscription="subscription"
          @renew="openRenewal(subscription)"
        />
      </div>
    </UiPage>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import SubscriptionUsagePanel from '@/components/user/subscriptions/SubscriptionUsagePanel.vue'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import { useAppStore } from '@/stores/app'
import { UiPage } from '@/ui'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const subscriptions = ref<UserSubscription[]>([])
const loading = ref(true)

async function loadSubscriptions(): Promise<void> {
  loading.value = true
  try {
    subscriptions.value = await subscriptionsAPI.getMySubscriptions()
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function openRenewal(subscription: UserSubscription): void {
  void router.push({
    path: '/purchase',
    query: { tab: 'subscription', group: String(subscription.group_id) }
  })
}

onMounted(() => {
  void loadSubscriptions()
})
</script>

<style scoped>
.subscription-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
</style>
