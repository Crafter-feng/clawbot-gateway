import { create } from 'zustand'
import { api } from '../api/client'

const STORAGE_KEY = 'clawbot_token'

interface AuthState {
  token: string | null
  authenticated: boolean
  loginError: string
  loginLoading: boolean
  error: string | null
  // API Token 管理
  apiToken: string
  apiTokenLoading: boolean
  // 方法
  login(password: string): Promise<boolean>
  logout(): void
  checkAuth(): void
  fetchApiToken(): Promise<void>
  regenerateApiToken(): Promise<void>
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: null,
  authenticated: false,
  loginError: '',
  loginLoading: false,
  error: null,
  apiToken: '',
  apiTokenLoading: false,

  async login(password: string) {
    set({ loginLoading: true, loginError: '' })
    try {
      const res = await api.post<{ token: string }>('/auth/login', { password })
      const t = res.token
      api.setToken(t)
      localStorage.setItem(STORAGE_KEY, t)
      set({ token: t, authenticated: true, loginLoading: false, loginError: '' })
      // 登录成功后自动获取 API Token
      get().fetchApiToken()
      return true
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Login failed'
      set({ loginLoading: false, loginError: msg })
      return false
    }
  },

  logout() {
    api.setToken(null)
    localStorage.removeItem(STORAGE_KEY)
    set({ token: null, authenticated: false, loginError: '', loginLoading: false, apiToken: '' })
  },

  checkAuth() {
    const t = localStorage.getItem(STORAGE_KEY)
    if (t) {
      // 非 JWT 格式（token 不是三段式），视为过期并清除
      const parts = t.split('.')
      if (parts.length !== 3) {
        localStorage.removeItem(STORAGE_KEY)
        set({ authenticated: false, apiToken: '' })
        return
      }
      // 检查 JWT 是否过期（简单解析 exp 字段）
      try {
        const payload = JSON.parse(atob(parts[1]))
        if (payload.exp && payload.exp * 1000 < Date.now()) {
          // JWT 已过期
          get().logout()
          return
        }
      } catch {
        // JWT 解析失败，视为过期
        get().logout()
        return
      }
      // JWT 有效，设置认证状态
      api.setToken(t)
      set({ token: t, authenticated: true })
      get().fetchApiToken()
    }
  },

  async fetchApiToken() {
    try {
      const res = await api.get<{ token: string }>('/api/v1/auth/token')
      set({ apiToken: res.token })
    } catch (e) {
      const msg = e instanceof Error ? e.message : '获取 API Token 失败'
      set({ error: msg })
      // 静默失败（可能无权限或网络错误）
    }
  },

  async regenerateApiToken() {
    set({ apiTokenLoading: true })
    try {
      const res = await api.post<{ token: string }>('/api/v1/auth/token')
      set({ apiToken: res.token, apiTokenLoading: false, error: null })
    } catch (e) {
      set({ apiTokenLoading: false, error: e instanceof Error ? e.message : 'regenerate failed' })
      throw e
    }
  },
}))
