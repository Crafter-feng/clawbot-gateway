import { create } from 'zustand'
import { api } from '../api/client'

interface Backend {
  id: string
  name: string
  type: string
  healthy: boolean
  enabled: boolean
  config?: string
}

interface BackendForm {
  id: string
  name: string
  type: string
  config?: Record<string, unknown>
}

interface BackendsState {
  items: Backend[]
  defaultBackend: string
  loading: boolean
  error: string | null
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
  error: null,

  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ backends: Backend[]; default: string }>('/api/v1/backends')
      set({ items: data.backends || [], defaultBackend: data.default ?? '', loading: false })
    } catch (e) {
      const msg = e instanceof Error ? e.message : '获取后端列表失败'
      set({ items: [], defaultBackend: '', loading: false, error: msg })
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
    try {
      await api.post('/api/v1/backends', data)
      await get().fetch()
      set({ error: null })
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'operation failed' })
      throw e
    }
  },

  async update(id: string, data: Partial<BackendForm>) {
    try {
      await api.put(`/api/v1/backends/${id}`, data)
      await get().fetch()
      set({ error: null })
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'operation failed' })
      throw e
    }
  },

  async remove(id: string) {
    try {
      await api.del(`/api/v1/backends/${id}`)
      await get().fetch()
      set({ error: null })
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'operation failed' })
      throw e
    }
  },

  async setDefault(backendId: string) {
    try {
      await api.put('/api/v1/backends/default', { backend_id: backendId })
      set({ defaultBackend: backendId, error: null })
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'operation failed' })
      throw e
    }
  },

  async test(id: string) {
    return await api.post<{ healthy: boolean; reply?: string; error?: string }>(`/api/v1/backends/${id}/test`)
  },
}))
