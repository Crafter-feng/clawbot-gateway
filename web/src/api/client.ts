// 在 Tauri 生产模式下使用绝对 URL，开发模式使用相对路径（Vite proxy）
const isTauri = typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window
const BASE_URL = isTauri ? 'http://localhost:8080' : ''

let token: string | null = null
let onUnauthorized: (() => void) | null = null

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`${BASE_URL}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401) {
    token = null
    onUnauthorized?.()
    throw new Error('认证已过期，请重新登录')
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? `HTTP ${res.status}`)
  }

  return res.json()
}

async function get<T>(path: string): Promise<T> {
  return request<T>('GET', path)
}

async function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>('POST', path, body)
}

async function del<T>(path: string): Promise<T> {
  return request<T>('DELETE', path)
}

async function put<T>(path: string, body?: unknown): Promise<T> {
  return request<T>('PUT', path, body)
}

function setToken(newToken: string | null): void {
  token = newToken
}

function setOnUnauthorized(handler: (() => void) | null): void {
  onUnauthorized = handler
}

export const api = { get, post, del, put, setToken, setOnUnauthorized }