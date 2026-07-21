<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <ProfileWithdrawalCard />
    </UiPage>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import ProfileWithdrawalCard from '@/components/user/profile/ProfileWithdrawalCard.vue'
import { UiPage } from '@/ui'
</script>
