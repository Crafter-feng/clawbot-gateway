import { useEffect, useState } from 'react'
import { useAuthStore } from '../stores/auth'
import { useBackendsStore } from '../stores/backends'
import { useRoutesStore } from '../stores/routes'
import { useAccountsStore } from '../stores/accounts'
import { useToast } from '../components/Toast'
import { api } from '../api/client'

interface UserRouteInfo {
  user_id: string
  mode: string
  secondaries: string[]
  backend: string
}

export default function SettingsPage() {
  const auth = useAuthStore()
  const backends = useBackendsStore()
  const routes = useRoutesStore()
  const accounts = useAccountsStore()
  const { toast } = useToast()

  const [routeUsers, setRouteUsers] = useState<UserRouteInfo[]>([])
  const [routeLoading, setRouteLoading] = useState(false)
  const [savingUserId, setSavingUserId] = useState<string | null>(null)

  useEffect(() => {
    backends.fetch()
    routes.fetch()
    accounts.fetch()
  }, [])

  useEffect(() => {
    if (backends.items.length > 0) {
      const controller = new AbortController()
      fetchRouteModes(controller.signal)
      return () => controller.abort()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [backends.items.length])

  const fetchRouteModes = async (signal?: AbortSignal) => {
    setRouteLoading(true)
    try {
      const usersData = await api.get<{ users: { user_id: string; backend: string }[] }>('/api/v1/users')
      if (signal?.aborted) return
      const infos: UserRouteInfo[] = []
      for (const u of usersData.users) {
        try {
          const modeData = await api.get<{ user_id: string; mode: string; secondaries: string[] }>(
            `/api/v1/users/${encodeURIComponent(u.user_id)}/route-mode`
          )
          if (signal?.aborted) return
          infos.push({ ...modeData, backend: u.backend })
        } catch {
          infos.push({ user_id: u.user_id, mode: 'single', secondaries: [], backend: u.backend })
        }
      }
      setRouteUsers(infos)
    } catch (e) {
      if (signal?.aborted) return
      console.error('Failed to fetch route modes', e)
    } finally {
      if (!signal?.aborted) setRouteLoading(false)
    }
  }

  const handleModeChange = async (userId: string, mode: string) => {
    setSavingUserId(userId)
    try {
      await api.post(`/api/v1/users/${encodeURIComponent(userId)}/route-mode`, { mode })
      setRouteUsers(prev =>
        prev.map(u => u.user_id === userId ? { ...u, mode } : u)
      )
      toast('路由模式已更新', 'success')
    } catch (e: any) {
      toast(e.message || '更新失败', 'error')
    } finally {
      setSavingUserId(null)
    }
  }

  const handleSecondaryToggle = async (userId: string, backendId: string, currentSecondaries: string[]) => {
    const newSecondaries = currentSecondaries.includes(backendId)
      ? currentSecondaries.filter(s => s !== backendId)
      : [...currentSecondaries, backendId]

    const user = routeUsers.find(u => u.user_id === userId)
    const mode = user?.mode || 'single'

    setSavingUserId(userId)
    try {
      await api.post(`/api/v1/users/${encodeURIComponent(userId)}/route-mode`, {
        mode,
        secondaries: newSecondaries,
      })
      setRouteUsers(prev =>
        prev.map(u => u.user_id === userId ? { ...u, secondaries: newSecondaries } : u)
      )
    } catch (e: any) {
      toast(e.message || '更新失败', 'error')
    } finally {
      setSavingUserId(null)
    }
  }

  // ── API Token 操作 ──

  const handleCopyApiToken = async () => {
    if (auth.apiToken) {
      try {
        await navigator.clipboard.writeText(auth.apiToken)
        toast('已复制到剪贴板', 'success')
      } catch {
        toast('复制失败', 'error')
      }
    }
  }

  const handleRegenApiToken = async () => {
    if (!window.confirm('确定要重新生成 API Token 吗？当前 Token 将立即失效。')) return
    try {
      await auth.regenerateApiToken()
      toast('API Token 已重新生成', 'success')
    } catch {
      toast('重新生成失败', 'error')
    }
  }

  const modeLabel: Record<string, string> = {
    single: '单后端',
    both: '双后端',
    three: '三后端',
  }

  return (
    <div>
      <div className="page-header">
        <h1>设置</h1>
        <p>系统配置与 API Token 管理</p>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
        {/* 系统信息 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>系统信息</div>
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))',
            gap: '12px',
          }}>
            {[
              { label: '微信账号', value: accounts.items.length },
              { label: '后端数量', value: backends.items.length },
              { label: '路由规则', value: routes.items.length },
              { label: '登录会话', value: auth.authenticated ? '有效' : '未登录' },
            ].map((item, i) => (
              <div key={i} style={{
                padding: '16px',
                background: 'var(--bg-primary)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--border)',
              }}>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '6px' }}>{item.label}</div>
                <div style={{ fontSize: '20px', fontWeight: 700 }}>{String(item.value)}</div>
              </div>
            ))}
          </div>
        </div>

        {/* API Token 管理 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '8px' }}>API Token</div>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginBottom: '16px' }}>
            用于外部 HTTP API 调用的身份验证（Bearer Token）
          </p>

          {auth.apiToken ? (
            <>
              <div style={{
                padding: '14px 16px',
                background: 'var(--bg-primary)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--border)',
                marginBottom: '16px',
                wordBreak: 'break-all',
                fontSize: '14px',
                fontFamily: 'monospace',
                color: 'var(--text-secondary)',
                lineHeight: 1.6,
              }}>
                {auth.apiToken}
              </div>

              <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '12px', fontFamily: 'monospace' }}>
                Authorization: Bearer {auth.apiToken}
              </div>

              <div style={{ display: 'flex', gap: '12px' }}>
                <button className="btn btn-secondary btn-sm" onClick={handleCopyApiToken}>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2" /><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
                  </svg>
                  复制 Token
                </button>
                <button
                  className="btn btn-danger btn-sm"
                  onClick={handleRegenApiToken}
                  disabled={auth.apiTokenLoading}
                >
                  {auth.apiTokenLoading ? <span className="spinner spinner-sm" /> : null}
                  重新生成
                </button>
              </div>
            </>
          ) : (
            <div style={{ color: 'var(--text-muted)', fontSize: '14px' }}>
              {auth.apiTokenLoading ? '加载中...' : '未获取到 API Token'}
            </div>
          )}
        </div>

        {/* 路由模式 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '8px' }}>路由模式</div>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginBottom: '16px' }}>
            为每个用户设置消息路由模式：单后端、双后端或三后端并行回复
          </p>

          {routeLoading && routeUsers.length === 0 ? (
            <div className="loading-overlay">
              <span className="spinner spinner-sm" />
              加载中...
            </div>
          ) : routeUsers.length === 0 ? (
            <div className="empty-state">暂无活跃用户会话</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              {routeUsers.map(user => (
                <div key={user.user_id} style={{
                  padding: '16px',
                  background: 'var(--bg-primary)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border)',
                }}>
                  <div style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    marginBottom: '12px',
                    flexWrap: 'wrap',
                    gap: '8px',
                  }}>
                    <div>
                      <span style={{ fontWeight: 600, fontSize: '14px' }}>{user.user_id}</span>
                      <span style={{
                        marginLeft: '8px',
                        fontSize: '12px',
                        color: 'var(--text-muted)',
                      }}>
                        主后端: {user.backend}
                      </span>
                    </div>
                    <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                      {(['single', 'both', 'three'] as const).map(m => (
                        <button
                          key={m}
                          className={`btn btn-sm ${user.mode === m ? 'btn-primary' : 'btn-ghost'}`}
                          onClick={() => handleModeChange(user.user_id, m)}
                          disabled={savingUserId === user.user_id}
                          style={{
                            padding: '4px 10px',
                            fontSize: '12px',
                            borderRadius: '999px',
                            ...(user.mode === m ? {} : {
                              border: '1px solid var(--border)',
                              background: 'transparent',
                              color: 'var(--text-secondary)',
                            }),
                          }}
                        >
                          {modeLabel[m]}
                        </button>
                      ))}
                    </div>
                  </div>

                  {(user.mode === 'both' || user.mode === 'three') && (
                    <div>
                      <div style={{
                        fontSize: '12px',
                        color: 'var(--text-muted)',
                        marginBottom: '8px',
                      }}>
                        选择副后端（{user.mode === 'both' ? '选 1 个' : '选 1-2 个'}）：
                      </div>
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                        {backends.items
                          .filter(b => b.id !== user.backend)
                          .map(b => {
                            const selected = user.secondaries.includes(b.id)
                            return (
                              <button
                                key={b.id}
                                className={`btn btn-sm ${selected ? 'btn-primary' : 'btn-ghost'}`}
                                onClick={() => handleSecondaryToggle(user.user_id, b.id, user.secondaries)}
                                disabled={savingUserId === user.user_id}
                                style={{
                                  padding: '4px 10px',
                                  fontSize: '12px',
                                  borderRadius: '999px',
                                  ...(selected ? {} : {
                                    border: '1px solid var(--border)',
                                    background: 'transparent',
                                    color: 'var(--text-secondary)',
                                  }),
                                }}
                              >
                                {selected && (
                                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3">
                                    <polyline points="20 6 9 17 4 12" />
                                  </svg>
                                )}
                                {b.name || b.id}
                              </button>
                            )
                          })}
                        {backends.items.filter(b => b.id !== user.backend).length === 0 && (
                          <span style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
                            没有可用的副后端（请先添加更多后端）
                          </span>
                        )}
                      </div>
                    </div>
                  )}

                  <div style={{ marginTop: '8px', fontSize: '12px', color: 'var(--text-muted)' }}>
                    {user.mode === 'single' && '仅使用主后端回复'}
                    {user.mode === 'both' && '主后端 + 1 个副后端并行回复'}
                    {user.mode === 'three' && '主后端 + 2 个副后端并行回复'}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
