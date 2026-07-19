import { create } from 'zustand'
import { api } from '../api/client'

interface StatsState {
  sessions: number
  messagesProcessed: number
  loading: boolean
  fetch(): Promise<void>
}

export const useStatsStore = create<StatsState>((set) => ({
  sessions: 0,
  messagesProcessed: 0,
  loading: false,

  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ sessions: number; messages_processed: number }>('/api/v1/stats')
      set({ sessions: data.sessions, messagesProcessed: data.messages_processed, loading: false })
    } catch {
      set({ loading: false })
    }
  },
}))