<template>
  <BaseDialog :show="show" :title="operation === 'add' ? t('admin.users.addPoints') : t('admin.users.deductPoints')" width="narrow" @close="$emit('close')">
    <form v-if="user" id="points-form" @submit.prevent="handlePointsSubmit" class="space-y-5">
      <div class="flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-primary-100">
          <span class="text-lg font-medium text-primary-700">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="flex-1">
          <p class="font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
          <p class="text-sm text-gray-500 dark:text-gray-300">{{ t('admin.users.currentPoints') }}: {{ formatPoints(user.points_balance || 0) }}</p>
        </div>
      </div>

      <div>
        <label class="input-label">{{ operation === 'add' ? t('admin.users.addPointsAmount') : t('admin.users.deductPointsAmount') }}</label>
        <div class="relative flex gap-2">
          <input v-model.number="form.amount" type="number" step="0.01" min="0.01" required class="input flex-1" />
          <button v-if="operation === 'subtract'" type="button" @click="fillAllPoints" class="btn btn-secondary whitespace-nowrap">{{ t('admin.users.deductAllPoints') }}</button>
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('admin.users.notes') }}</label>
        <textarea v-model="form.notes" rows="3" class="input"></textarea>
      </div>

      <div v-if="form.amount > 0" class="rounded-xl border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-950">
        <div class="flex items-center justify-between text-sm">
          <span class="text-gray-700 dark:text-gray-300">{{ t('admin.users.newPoints') }}:</span>
          <span class="font-bold text-gray-900 dark:text-gray-100">{{ formatPoints(calculateNewPoints()) }}</span>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button type="submit" form="points-form" :disabled="submitting || !form.amount" class="btn" :class="operation === 'add' ? 'bg-emerald-600 text-white' : 'btn-danger'">
          {{ submitting ? t('common.saving') : t('common.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{ show: boolean, user: AdminUser | null, operation: 'add' | 'subtract' }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const submitting = ref(false)
const form = reactive({ amount: 0, notes: '' })

watch(() => props.show, (visible) => {
  if (visible) {
    form.amount = 0
    form.notes = ''
  }
})

function formatPoints(value: number) {
  if (value === 0) return '0.00'
  const formatted = value.toFixed(10).replace(/\.?0+$/, '')
  const parts = formatted.split('.')
  if (parts.length === 1) return `${formatted}.00`
  if (parts[1].length === 1) return `${formatted}0`
  return formatted
}

function fillAllPoints() {
  if (props.user) {
    form.amount = props.user.points_balance || 0
  }
}

function calculateNewPoints() {
  if (!props.user) return 0
  const current = props.user.points_balance || 0
  const result = props.operation === 'add' ? current + form.amount : current - form.amount
  return Math.abs(result) < 1e-10 ? 0 : result
}

async function handlePointsSubmit() {
  if (!props.user) return
  if (!form.amount || form.amount <= 0) {
    appStore.showError(t('admin.users.amountRequired'))
    return
  }
  if (props.operation === 'subtract' && form.amount > (props.user.points_balance || 0)) {
    appStore.showError(t('admin.users.insufficientPoints'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.users.updatePoints(props.user.id, form.amount, props.operation, form.notes)
    appStore.showSuccess(t('common.success'))
    emit('success')
    emit('close')
  } catch (e: any) {
    console.error('Failed to update points:', e)
    appStore.showError(e.response?.data?.detail || t('common.error'))
  } finally {
    submitting.value = false
  }
}
</script>
