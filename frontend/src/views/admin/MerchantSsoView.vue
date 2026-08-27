<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-6xl space-y-6 p-4 md:p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold text-[var(--app-text)]">商家登录接入</h1>
          <p class="mt-1 text-sm text-[var(--app-text-muted)]">配置 dynamic_api 的注册登录、已有用户登录和历史用户同步。</p>
        </div>
        <div class="flex gap-2">
          <button class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="md" />刷新</button>
          <button class="btn btn-primary" @click="openCreate"><Icon name="plus" size="md" />新增商家</button>
        </div>
      </div>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ error }}</div>
      <div v-if="loading" class="py-12 text-center text-sm text-[var(--app-text-muted)]">加载中...</div>
      <div v-else-if="integrations.length === 0" class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-10 text-center text-sm text-[var(--app-text-muted)]">暂无商家接入配置</div>
      <div v-else class="grid gap-4 lg:grid-cols-2">
        <article v-for="item in integrations" :key="item.id" class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-5 shadow-sm">
          <div class="flex items-start justify-between gap-3">
            <div><h2 class="font-medium text-[var(--app-text)]">{{ item.merchant_name || item.merchant_code }}</h2><p class="mt-1 font-mono text-xs text-[var(--app-text-muted)]">{{ item.merchant_code }}</p></div>
            <span :class="['badge', item.enabled ? 'badge-success' : 'badge-gray']">{{ item.enabled ? '已启用' : '已停用' }}</span>
          </div>
          <dl class="mt-4 space-y-2 text-xs text-[var(--app-text-muted)]">
            <div><dt class="inline font-medium">注册/登录：</dt><dd class="inline break-all">{{ item.register_login_url }}</dd></div>
            <div><dt class="inline font-medium">已有用户登录：</dt><dd class="inline break-all">{{ item.login_url }}</dd></div>
            <div><dt class="inline font-medium">用户同步：</dt><dd class="inline break-all">{{ item.user_sync_url }}（{{ item.user_sync_auth_type }}）</dd></div>
            <div><dt class="inline font-medium">跳转白名单：</dt><dd class="inline">{{ item.allowed_redirect_hosts.join(', ') }}</dd></div>
          </dl>
          <div class="mt-5 flex flex-wrap justify-end gap-2">
            <button v-if="item.user_sync_auth_type === 'hmac'" class="btn btn-secondary" @click="generate(item)"><Icon name="key" size="sm" />生成/轮换密钥</button>
            <button class="btn btn-secondary" :disabled="syncing === item.id" @click="sync(item)"><Icon name="refresh" size="sm" :class="syncing === item.id ? 'animate-spin' : ''" />{{ syncing === item.id ? '同步中...' : '同步历史用户' }}</button>
            <button class="btn btn-secondary" @click="openEdit(item)"><Icon name="edit" size="sm" />编辑</button>
          </div>
        </article>
      </div>

      <div v-if="editing" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="editing = null">
        <form class="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-lg bg-[var(--app-surface)] p-6 shadow-xl" @submit.prevent="save">
          <div class="flex items-center justify-between"><h2 class="text-lg font-semibold">{{ editing.id ? '编辑商家接入' : '新增商家接入' }}</h2><button type="button" class="btn btn-ghost" @click="editing = null">关闭</button></div>
          <div class="mt-5 grid gap-4 sm:grid-cols-2">
            <label class="field"><span>商家代码</span><input v-model="editing.merchant_code" required class="input" /></label>
            <label class="field"><span>商家名称</span><input v-model="editing.merchant_name" class="input" /></label>
            <label class="field sm:col-span-2"><span>register_login 地址（HTTPS）</span><input v-model="editing.register_login_url" required class="input" /></label>
            <label class="field sm:col-span-2"><span>login 地址（HTTPS）</span><input v-model="editing.login_url" required class="input" /></label>
            <label class="field sm:col-span-2"><span>user_sync 地址（HTTPS）</span><input v-model="editing.user_sync_url" required class="input" /></label>
            <label class="field"><span>同步认证</span><select v-model="editing.user_sync_auth_type" class="input"><option value="none">none</option><option value="hmac">HMAC</option></select></label>
            <label class="field"><span>启用</span><input v-model="editing.enabled" type="checkbox" class="mt-2 h-4 w-4" /></label>
            <label class="field sm:col-span-2"><span>跳转允许域名（逗号分隔）</span><input v-model="editing.allowed_redirect_hosts_text" required class="input" placeholder="merchant.example.com" /></label>
          </div>
          <div class="mt-6 flex justify-end gap-2"><button type="button" class="btn btn-secondary" @click="editing = null">取消</button><button class="btn btn-primary" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button></div>
        </form>
      </div>
      <div v-if="generatedSecret" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4" @click.self="generatedSecret = ''">
        <div class="w-full max-w-xl rounded-lg bg-[var(--app-surface)] p-6 shadow-xl">
          <h2 class="text-lg font-semibold">HMAC 密钥仅显示一次</h2>
          <p class="mt-2 text-sm text-[var(--app-text-muted)]">请立即复制到商家服务端的密钥存储。关闭后平台不会再次显示明文。</p>
          <code class="mt-4 block break-all rounded bg-black/5 p-3 text-sm dark:bg-white/5">{{ generatedSecret }}</code>
          <div class="mt-5 flex justify-end gap-2"><button class="btn btn-secondary" @click="copySecret">复制</button><button class="btn btn-primary" @click="generatedSecret = ''">我已保存</button></div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import merchantSsoAPI from '@/api/admin/merchantSso'
import type { MerchantSsoIntegration, MerchantSsoIntegrationInput } from '@/api/admin/merchantSso'

interface FormState extends MerchantSsoIntegrationInput { id?: number; allowed_redirect_hosts_text: string }
const integrations = ref<MerchantSsoIntegration[]>([])
const loading = ref(false)
const saving = ref(false)
const syncing = ref<number | null>(null)
const error = ref('')
const editing = ref<FormState | null>(null)
const generatedSecret = ref('')

async function load() { loading.value = true; error.value = ''; try { integrations.value = (await merchantSsoAPI.list()).integrations } catch (err: any) { error.value = err?.message || '加载失败' } finally { loading.value = false } }
function openCreate() { editing.value = { merchant_code: '', merchant_name: '', enabled: false, register_login_url: '', login_url: '', user_sync_url: '', user_sync_auth_type: 'none', allowed_redirect_hosts: [], allowed_redirect_hosts_text: '' } }
function openEdit(item: MerchantSsoIntegration) { editing.value = { ...item, allowed_redirect_hosts_text: item.allowed_redirect_hosts.join(', ') } }
async function save() {
  if (!editing.value) return
  saving.value = true
  try {
    const form = editing.value
    const payload: MerchantSsoIntegrationInput = { merchant_code: form.merchant_code.trim(), merchant_name: form.merchant_name?.trim(), enabled: form.enabled, register_login_url: form.register_login_url.trim(), login_url: form.login_url.trim(), user_sync_url: form.user_sync_url.trim(), user_sync_auth_type: form.user_sync_auth_type, allowed_redirect_hosts: form.allowed_redirect_hosts_text.split(',').map((value) => value.trim()).filter(Boolean) }
    if (form.id) await merchantSsoAPI.update(form.id, payload)
    else await merchantSsoAPI.create(payload)
    editing.value = null
    await load()
  } catch (err: any) { error.value = err?.message || '保存失败' } finally { saving.value = false }
}
async function sync(item: MerchantSsoIntegration) { syncing.value = item.id; try { const result = await merchantSsoAPI.syncUsers(item.id); error.value = `同步完成：匹配 ${result.matched}，新增绑定 ${result.created}，更新 ${result.updated}，跳过 ${result.skipped}` } catch (err: any) { error.value = err?.message || '同步失败' } finally { syncing.value = null } }
async function generate(item: MerchantSsoIntegration) { try { generatedSecret.value = (await merchantSsoAPI.generateHmacSecret(item.id)).hmac_secret; await load() } catch (err: any) { error.value = err?.message || '生成密钥失败' } }
async function copySecret() { if (!generatedSecret.value) return; await navigator.clipboard?.writeText(generatedSecret.value) }
onMounted(load)
</script>

<style scoped>
.field { display: flex; flex-direction: column; gap: 0.4rem; font-size: 0.82rem; color: var(--app-text-muted); }
.field input, .field select { color: var(--app-text); }
</style>
