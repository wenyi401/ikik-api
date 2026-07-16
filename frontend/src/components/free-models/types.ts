import type { Account } from '@/types'

export interface FreeModelProvider {
  code: string
  name: string
  initials: string
  baseUrl: string
  baseUrlEditable?: boolean
  models: string[]
  note: string
  keyUrl: string
  docsUrl: string
}

export interface FreeModelTestState {
  status: 'success' | 'warning' | 'error'
  message: string
  latency?: number
}

export interface FreeModelAccountLimit {
  model: string
  resetAt: string
}

export type FreeModelAccount = Account
