import { create } from 'zustand'
import { api } from '../api/client'

interface Route {
  id: number
  keyword: string
  backend_id: string
  is_regexp: boolean
  priority: number
}

interface RoutesState {
  items: Route[]
  loading: boolean
  fetch(): Promise<void>
  add(keyword: string, backendId: string, isRegexp: boolean): Promise<void>
  remove(id: number): Promise<void>
}

export const useRoutesStore = create<RoutesState>((set, get) => ({
  items: [],
  loading: false,

  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ routes: Route[] }>('/api/v1/routes')
      set({ items: data.routes || [], loading: false })
    } catch {
      set({ items: [], loading: false })
    }
  },

  async add(keyword: string, backendId: string, isRegexp: boolean) {
    await api.post('/api/v1/routes', { keyword, backend_id: backendId, is_regexp: isRegexp })
    await get().fetch()
  },

  async remove(id: number) {
    await api.del(`/api/v1/routes/${id}`)
    await get().fetch()
  },
}))
