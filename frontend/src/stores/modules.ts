import { defineStore } from 'pinia'
import { ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { Module } from '@/types'

export const useModulesStore = defineStore('modules', () => {
  // State
  const modules = ref<Module[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Actions
  async function fetchModules() {
    loading.value = true
    error.value = null
    try {
      const res = await adminAPI.modules.list()
      modules.value = res.modules ?? []
    } catch (err: any) {
      error.value = err?.response?.data?.message || err?.message || 'Failed to load modules'
      throw err
    } finally {
      loading.value = false
    }
  }

  return {
    // State
    modules,
    loading,
    error,
    // Actions
    fetchModules,
  }
})
