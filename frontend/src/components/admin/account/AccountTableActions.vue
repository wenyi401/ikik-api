<template>
  <div class="flex flex-wrap items-center justify-end gap-2">
    <slot name="before"></slot>
    <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="$emit('refresh')">
      <Icon name="refresh" size="md" :class="[loading ? 'animate-spin' : '']" />
    </UiIconButton>
    <slot name="after"></slot>
    <UiMenu :label="t('common.more')">
      <button type="button" @click="$emit('sync')">{{ t('admin.accounts.syncFromCrs') }}</button>
      <slot name="beforeCreate"></slot>
    </UiMenu>
    <button @click="$emit('create')" class="btn btn-primary">{{ t('admin.accounts.createAccount') }}</button>
    <slot name="afterCreate"></slot>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { UiIconButton, UiMenu } from '@/ui'

defineProps(['loading'])
defineEmits(['refresh', 'sync', 'create'])

const { t } = useI18n()
</script>
