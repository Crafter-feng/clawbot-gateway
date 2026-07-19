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
  fetch(): Promise<void>
  disconnect(id: string): Promise<void>
}

export const useAccountsStore = create<AccountsState>((set, get) => ({
  items: [],
  loading: false,

  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ accounts: Account[] }>('/api/v1/accounts')
      set({ items: data.accounts || [], loading: false })
    } catch {
      set({ items: [], loading: false })
    }
  },

  async disconnect(id: string) {
    await api.del(`/api/v1/accounts/${id}`)
    await get().fetch()
  },
}))
