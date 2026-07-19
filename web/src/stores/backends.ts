import { create } from 'zustand'
import { api } from '../api/client'

interface Backend {
  id: string
  name: string
  type: string
  healthy: boolean
}

interface BackendForm {
  id: string
  name: string
  type: string
  config: {
    api_key: string
    base_url: string
    model: string
  }
}

interface BackendsState {
  items: Backend[]
  defaultBackend: string
  loading: boolean
  fetch(): Promise<void>
  register(data: BackendForm): Promise<void>
  remove(id: string): Promise<void>
  setDefault(backendId: string): Promise<void>
}

export const useBackendsStore = create<BackendsState>((set, get) => ({
  items: [],
  defaultBackend: '',
  loading: false,

  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ backends: Backend[]; default_backend: string }>('/api/v1/backends')
      set({ items: data.backends, defaultBackend: data.default_backend ?? '', loading: false })
    } catch {
      set({ loading: false })
    }
  },

  async register(data: BackendForm) {
    await api.post('/api/v1/backends', data)
    await get().fetch()
  },

  async remove(id: string) {
    await api.del(`/api/v1/backends/${id}`)
    await get().fetch()
  },

  async setDefault(backendId: string) {
    await api.post('/api/v1/backends/default', { backend_id: backendId })
    set({ defaultBackend: backendId })
  },
}))