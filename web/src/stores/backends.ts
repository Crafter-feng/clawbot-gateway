import { create } from 'zustand'
import { api } from '../api/client'

interface Backend {
  id: string
  name: string
  type: string
  healthy: boolean
  config?: Record<string, unknown>
}

interface BackendForm {
  id: string
  name: string
  type: string
  config: {
    api_key?: string
    base_url?: string
    model?: string
    url?: string
    headers?: Record<string, string>
  }
}

interface BackendsState {
  items: Backend[]
  defaultBackend: string
  loading: boolean
  fetch(): Promise<void>
  get(id: string): Promise<Backend | null>
  register(data: BackendForm): Promise<void>
  update(id: string, data: Partial<BackendForm>): Promise<void>
  remove(id: string): Promise<void>
  setDefault(backendId: string): Promise<void>
  test(id: string): Promise<{ healthy: boolean; reply?: string; error?: string }>
}

export const useBackendsStore = create<BackendsState>((set, get) => ({
  items: [],
  defaultBackend: '',
  loading: false,

  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ backends: Backend[]; default: string }>('/api/v1/backends')
      set({ items: data.backends, defaultBackend: data.default ?? '', loading: false })
    } catch {
      set({ loading: false })
    }
  },

  async get(id: string) {
    try {
      return await api.get<Backend>(`/api/v1/backends/${id}`)
    } catch {
      return null
    }
  },

  async register(data: BackendForm) {
    await api.post('/api/v1/backends', data)
    await get().fetch()
  },

  async update(id: string, data: Partial<BackendForm>) {
    await api.post(`/api/v1/backends/${id}`, data)
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

  async test(id: string) {
    return await api.post<{ healthy: boolean; reply?: string; error?: string }>(`/api/v1/backends/${id}/test`)
  },
}))
