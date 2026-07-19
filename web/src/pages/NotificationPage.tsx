import { useEffect, useState, useCallback } from 'react'
import { api } from '../api/client'
import { useToast } from '../components/Toast'

interface 通知Token {
  id: string
  account_id: string
  name: string
  token: string
  enabled: boolean
  created_at: string
}

interface Account {
  account_id: string
  user_id: string
  account_name: string
}

export default function NotificationPage() {
  const { toast } = useToast()
  const [tokens, setTokens] = useState<通知Token[]>([])
  const [accounts, setAccounts] = useState<Account[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [newAccountId, setNewAccountId] = useState('')
  const [creating, setCreating] = useState(false)
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [showToken, setShowToken] = useState<{ id: string; token: string } | null>(null)

  useEffect(() => {
    fetchData()
  }, [])

  const fetchData = async () => {
    setLoading(true)
    try {
      const [tokensRes, accountsRes] = await Promise.all([
        api.get<{ tokens: 通知Token[] }>('/api/v1/notify/tokens'),
        api.get<{ accounts: Account[] }>('/api/v1/accounts'),
      ])
      setTokens(tokensRes.tokens || [])
      setAccounts(accountsRes.accounts || [])
    } catch {
      toast('加载失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  const handleCreate = useCallback(async () => {
    if (!newName.trim()) return
    setCreating(true)
    try {
      const res = await api.post<{ id: string; token: string }>('/api/v1/notify/tokens', {
        name: newName.trim(),
        account_id: newAccountId,
      })
      toast('Token 创建成功', 'success')
      setShowCreate(false)
      setNewName('')
      setNewAccountId('')
      setShowToken({ id: res.id, token: res.token })
      fetchData()
    } catch {
      toast('创建失败', 'error')
    } finally {
      setCreating(false)
    }
  }, [newName, newAccountId, toast])

  const handleDelete = useCallback(async (id: string) => {
    if (!confirm('确定删除此 Token？')) return
    try {
      await api.del(`/api/v1/notify/tokens/${id}`)
      toast('Token 已删除', 'success')
      fetchData()
    } catch {
      toast('删除失败', 'error')
    }
  }, [toast])

  const handleCopy = useCallback(async (text: string, id: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedId(id)
      setTimeout(() => setCopiedId(null), 2000)
    } catch {
      toast('复制失败', 'error')
    }
  }, [toast])

  const getAccountName = (accountId: string) => {
    if (!accountId) return '全部账号'
    const account = accounts.find(a => a.account_id === accountId)
    return account ? (account.account_name || account.user_id) : accountId
  }

  return (
    <div>
      <div className="page-header">
        <h1>通知</h1>
        <p>配置 Token，供外部系统调用推送消息到微信</p>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>

        {/* Token 列表 */}
        <div className="card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <div style={{ fontSize: '15px', fontWeight: 600 }}>Token 列表</div>
            <button className="btn btn-primary btn-sm" onClick={() => setShowCreate(true)}>
              创建 Token
            </button>
          </div>

          {loading ? (
            <div className="empty-state">加载中...</div>
          ) : tokens.length === 0 ? (
            <div className="empty-state">暂无 Token，点击"创建 Token"开始</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              {tokens.map((t) => (
                <div key={t.id} style={{
                  padding: '16px',
                  background: 'var(--bg-primary)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border)',
                }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <div>
                      <div style={{ fontWeight: 600, fontSize: '14px' }}>{t.name}</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
                        绑定: {getAccountName(t.account_id)}
                      </div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                        创建: {new Date(t.created_at).toLocaleString('zh-CN')}
                      </div>
                    </div>
                    <div style={{ display: 'flex', gap: '8px' }}>
                      <button className="btn btn-secondary btn-sm" onClick={() => handleCopy(t.token, t.id)}>
                        {copiedId === t.id ? '已复制' : '复制 Token'}
                      </button>
                      <button className="btn btn-danger btn-sm" onClick={() => handleDelete(t.id)}>删除</button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 创建 Token 表单 */}
        {showCreate && (
          <div className="card">
            <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>创建 Token</div>
            
            <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>名称</label>
                <input className="input" placeholder="例如：Hermes 通知" value={newName} onChange={(e) => setNewName(e.target.value)} />
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>绑定账号</label>
                <select className="select" value={newAccountId} onChange={(e) => setNewAccountId(e.target.value)}>
                  <option value="">全部账号</option>
                  {accounts.map((a) => (
                    <option key={a.account_id} value={a.account_id}>
                      {a.account_name || a.user_id} ({a.account_id})
                    </option>
                  ))}
                </select>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
                  选择要绑定的微信账号，留空则可发送到任意账号
                </div>
              </div>

              <div style={{ display: 'flex', gap: '8px' }}>
                <button className="btn btn-primary" onClick={handleCreate} disabled={creating || !newName.trim()}>
                  {creating ? <span className="spinner spinner-sm" /> : null}
                  创建
                </button>
                <button className="btn btn-secondary" onClick={() => setShowCreate(false)}>取消</button>
              </div>
            </div>
          </div>
        )}

        {/* 新创建的 Token 显示 */}
        {showToken && (
          <div className="card" style={{ border: '1px solid var(--accent)' }}>
            <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '12px', color: 'var(--accent)' }}>
              Token 创建成功
            </div>
            <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginBottom: '12px' }}>
              请复制并保存 Token，关闭后将无法再次查看
            </p>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center', marginBottom: '12px' }}>
              <code style={{
                flex: 1,
                padding: '12px',
                background: 'var(--bg-secondary)',
                borderRadius: 'var(--radius-sm)',
                fontSize: '13px',
                fontFamily: 'monospace',
                wordBreak: 'break-all',
              }}>
                {showToken.token}
              </code>
              <button className="btn btn-primary btn-sm" onClick={() => handleCopy(showToken.token, 'new')}>
                {copiedId === 'new' ? '已复制' : '复制'}
              </button>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <button className="btn btn-secondary btn-sm" onClick={() => setShowToken(null)}>关闭</button>
            </div>
          </div>
        )}

        {/* 使用说明 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '12px' }}>使用说明</div>
          <div style={{ fontSize: '13px', color: 'var(--text-secondary)', lineHeight: 1.6 }}>
            <p style={{ margin: '0 0 8px 0' }}>1. 创建 Token，选择要绑定的微信账号</p>
            <p style={{ margin: '0 0 8px 0' }}>2. 复制 Token 并保存</p>
            <p style={{ margin: '0 0 8px 0' }}>3. 外部系统使用 Token 调用 API 发送消息</p>
            <p style={{ margin: 0 }}>4. Token 仅在创建时显示，请妥善保管</p>
          </div>

          <div style={{
            marginTop: '16px',
            padding: '12px',
            background: 'var(--bg-primary)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border)',
          }}>
            <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '8px' }}>调用示例</div>
            <pre style={{
              fontSize: '11px',
              fontFamily: 'monospace',
              padding: '8px',
              background: 'var(--bg-secondary)',
              borderRadius: 'var(--radius-sm)',
              overflow: 'auto',
            }}>{`curl -X POST http://localhost:8080/api/v1/notify/send \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer <token>" \\
  -d '{
    "to_user": "wxid_xxx",
    "content": "Hello!"
  }'`}</pre>
          </div>
        </div>
      </div>
    </div>
  )
}
