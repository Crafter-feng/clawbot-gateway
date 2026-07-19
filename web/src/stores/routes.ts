import { create } from 'zustand'
import { api } from '../api/client'

interface Route {
  keyword: string
  backend: string
  is_regexp: boolean
}

interface RoutesState {
  items: Route[]
  loading: boolean
  fetch(): Promise<void>
  add(keyword: string, backend: string, isRegexp: boolean): Promise<void>
  remove(index: number): Promise<void>
}

export const useRoutesStore = create<RoutesState>((set, get) => ({
  items: [],
  loading: false,

  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ routes: Route[] }>('/api/v1/routes')
      set({ items: data.routes, loading: false })
    } catch {
      set({ loading: false })
    }
  },

  async add(keyword: string, backend: string, isRegexp: boolean) {
    await api.post('/api/v1/routes', { keyword, backend, is_regexp: isRegexp })
    await get().fetch()
  },

  async remove(index: number) {
    await api.del(`/api/v1/routes/${index}`)
    await get().fetch()
  },
}))