/**
 * Admin Modules API endpoints (plugin module observability, read-only)
 */

import { apiClient } from '../client'
import type { Module } from '@/types'

export async function list(): Promise<{ modules: Module[] }> {
  const { data } = await apiClient.get<{ modules: Module[] }>('/admin/modules')
  return data
}

const modulesAPI = {
  list
}

export default modulesAPI
