<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-4xl space-y-6 p-4 md:p-6">
      <div><h1 class="text-xl font-semibold text-[var(--app-text)]">商家服务</h1><p class="mt-1 text-sm text-[var(--app-text-muted)]">使用当前 IKIK 账号进入已启用的商家服务。</p></div>
      <div v-if="loading" class="py-12 text-center text-sm text-[var(--app-text-muted)]">加载中...</div>
      <div v-else-if="items.length === 0" class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-10 text-center text-sm text-[var(--app-text-muted)]">暂无可用商家服务</div>
      <div v-else class="grid gap-4 sm:grid-cols-2"><article v-for="item in items" :key="item.id" class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-5"><h2 class="font-medium">{{ item.merchant_name || item.merchant_code }}</h2><p class="mt-1 font-mono text-xs text-[var(--app-text-muted)]">{{ item.merchant_code }}</p><button class="btn btn-primary mt-5 w-full" :disabled="launching === item.merchant_code" @click="launch(item.merchant_code)">{{ launching === item.merchant_code ? '连接中...' : '进入商家' }}</button></article></div>
      <p v-if="message" class="text-sm text-[var(--app-text-muted)]">{{ message }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import merchantSsoAPI from '@/api/merchantSso'
import type { MerchantSsoIntegration } from '@/api/admin/merchantSso'
const items = ref<MerchantSsoIntegration[]>([])
const loading = ref(false)
const launching = ref('')
const message = ref('')
async function load() { loading.value = true; try { items.value = (await merchantSsoAPI.listIntegrations()).integrations } catch (err: any) { message.value = err?.message || '加载失败' } finally { loading.value = false } }
async function launch(code: string) { launching.value = code; message.value = ''; try { const result = await merchantSsoAPI.login(code); window.location.assign(result.redirect_url) } catch (err: any) { message.value = err?.message || '连接失败' } finally { launching.value = '' } }
onMounted(load)
</script>
