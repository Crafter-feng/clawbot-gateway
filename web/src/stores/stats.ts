import { create } from 'zustand'
import { api } from '../api/client'

interface StatsState {
  accounts: number
  sessions: number
  messagesProcessed: number
  version: string
  loading: boolean
  error: string | null
  fetch(): Promise<void>
}

export const useStatsStore = create<StatsState>((set) => ({
  accounts: 0,
  sessions: 0,
  messagesProcessed: 0,
  version: '',
  loading: false,
  error: null,
  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ accounts: number; sessions: number; messages_processed: number; version: string }>('/api/v1/stats')
      set({ accounts: data.accounts, sessions: data.sessions, messagesProcessed: data.messages_processed, version: data.version || '', loading: false })
    } catch (e) {
      const msg = e instanceof Error ? e.message : '获取统计数据失败'
      set({ loading: false, error: msg })
    }
  },
}))