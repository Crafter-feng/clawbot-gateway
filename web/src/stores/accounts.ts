import { create } from 'zustand'
import { api } from '../api/client'

interface Account {
  account_id: string
  user_id: string
  login_at: number
  status: string
}

interface AccountsState {
  items: Account[]
  loading: boolean
  error: string | null
  fetch(): Promise<void>
  disconnect(id: string): Promise<void>
}

export const useAccountsStore = create<AccountsState>((set, get) => ({
  items: [],
  loading: false,
  error: null,
  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ accounts: Account[] }>('/api/v1/accounts')
      set({ items: data.accounts || [], loading: false })
    } catch (e) {
      const msg = e instanceof Error ? e.message : '获取账号列表失败'
      set({ items: [], loading: false, error: msg })
    }
  },

  async disconnect(id: string) {
    try {
      await api.del(`/api/v1/accounts/${id}`)
      await get().fetch()
      set({ error: null })
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'operation failed' })
      throw e
    }
  },
}))
