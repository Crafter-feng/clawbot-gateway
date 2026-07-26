import { useEffect, useState, useCallback, useRef } from 'react'
import { api } from '../api/client'
import { useToast } from '../components/Toast'
import Button from '../components/ui/Button'
import Input from '../components/ui/Input'
import Select from '../components/ui/Select'
import ConfirmDialog from '../components/ui/ConfirmDialog'
import EmptyState from '../components/ui/EmptyState'
import { ListItemSkeleton } from '../components/ui/Skeleton'

interface Token {
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
  const [tokens, setTokens] = useState<Token[]>([])
  const [accounts, setAccounts] = useState<Account[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [newAccountId, setNewAccountId] = useState('')
  const [creating, setCreating] = useState(false)
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [showToken, setShowToken] = useState<{ id: string; token: string } | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const copyTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)


  useEffect(() => {
    fetchData()
  }, [])

  const fetchData = async () => {
    setLoading(true)
    try {
      const [tokensRes, accountsRes] = await Promise.all([
        api.get<{ tokens: Token[] }>('/api/v1/notify/tokens'),
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
    try {
      await api.del(`/api/v1/notify/tokens/${id}`)
      toast('Token 已删除', 'success')
      fetchData()
    } catch {
      toast('删除失败', 'error')
    } finally {
      setDeleteTarget(null)
    }
  }, [toast])

  const handleCopy = useCallback(async (text: string, id: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedId(id)
      clearTimeout(copyTimerRef.current)
      copyTimerRef.current = setTimeout(() => setCopiedId(null), 2000)
    } catch {
      toast('复制失败', 'error')
    }
  }, [toast])
  useEffect(() => {
    return () => clearTimeout(copyTimerRef.current)
  }, [])

  const getAccountName = (accountId: string) => {
    if (!accountId) return '全部账号'
    const account = accounts.find(a => a.account_id === accountId)
    return account ? (account.account_name || account.user_id) : accountId
  }

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1>通知</h1>
          <p>配置 Token，供外部系统调用推送消息到微信</p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          创建 Token
        </Button>
      </div>

      <div className="dashboard-content">
        {/* Token 列表 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">Token 列表</h2>
          </div>

          {loading ? (
            <div className="list-section">
              <ListItemSkeleton />
              <ListItemSkeleton />
            </div>
          ) : tokens.length === 0 ? (
            <EmptyState
              icon={
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
                </svg>
              }
              title="暂无 Token"
              description="创建 Token 以允许外部系统发送消息"
              action={
                <Button onClick={() => setShowCreate(true)}>
                  创建 Token
                </Button>
              }
            />
          ) : (
            <div className="list-section">
              {tokens.map((t) => (
                <div key={t.id} className="list-item" style={{ flexDirection: 'column', alignItems: 'stretch', gap: 'var(--space-3)' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <div className="list-item-info">
                      <div className="list-item-title">{t.name}</div>
                      <div className="list-item-subtitle">
                        绑定: {getAccountName(t.account_id)}
                      </div>
                      <div className="list-item-subtitle">
                        创建: {new Date(t.created_at).toLocaleString('zh-CN')}
                      </div>
                    </div>
                    <div className="list-item-actions">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleCopy(t.token, t.id)}
                      >
                        {copiedId === t.id ? '已复制' : '复制 Token'}
                      </Button>
                      <Button
                        variant="ghost-danger"
                        size="sm"
                        onClick={() => setDeleteTarget(t.id)}
                      >
                        删除
                      </Button>
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
            <div className="card-header">
              <h2 className="card-title">创建 Token</h2>
            </div>
            <div className="manage-form">
              <Input
                label="名称"
                placeholder="例如：Hermes 通知"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
              />
              <Select
                label="绑定账号"
                value={newAccountId}
                onChange={(e) => setNewAccountId(e.target.value)}
                hint="选择要绑定的微信账号，留空则可发送到任意账号"
              >
                <option value="">全部账号</option>
                {accounts.map((a) => (
                  <option key={a.account_id} value={a.account_id}>
                    {a.account_name || a.user_id} ({a.account_id})
                  </option>
                ))}
              </Select>
              <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                <Button onClick={handleCreate} loading={creating} disabled={!newName.trim()}>
                  创建
                </Button>
                <Button variant="ghost" onClick={() => setShowCreate(false)}>
                  取消
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* 新创建的 Token 显示 */}
        {showToken && (
          <div className="card" style={{ border: '1px solid var(--accent)' }}>
            <div className="card-header">
              <h2 className="card-title" style={{ color: 'var(--accent)' }}>Token 创建成功</h2>
            </div>
            <p className="card-description">
              请复制并保存 Token，关闭后将无法再次查看
            </p>
            <div style={{ display: 'flex', gap: 'var(--space-2)', alignItems: 'center', marginTop: 'var(--space-4)' }}>
              <code className="code-block" style={{ flex: 1 }}>
                {showToken.token}
              </code>
              <Button
                size="sm"
                onClick={() => handleCopy(showToken.token, 'new')}
              >
                {copiedId === 'new' ? '已复制' : '复制'}
              </Button>
            </div>
            <Button
              variant="ghost"
              size="sm"
              style={{ marginTop: 'var(--space-3)' }}
              onClick={() => setShowToken(null)}
            >
              关闭
            </Button>
          </div>
        )}

        {/* 使用说明 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">使用说明</h2>
          </div>
          <div className="notification-guide">
            <p>1. 创建 Token，选择要绑定的微信账号</p>
            <p>2. 复制 Token 并保存</p>
            <p>3. 外部系统使用 Token 调用 API 发送消息</p>
            <p>4. Token 仅在创建时显示，请妥善保管</p>
          </div>

          <div className="manage-config-preview" style={{ marginTop: 'var(--space-4)' }}>
            <div className="manage-config-title">调用示例</div>
            <pre className="code-block">{`curl -X POST http://localhost:8080/api/v1/notify/send \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer <token>" \\
  -d '{
    "to_user": "wxid_xxx",
    "content": "Hello!"
  }'`}</pre>
          </div>
        </div>
      </div>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => deleteTarget && handleDelete(deleteTarget)}
        title="删除 Token"
        description="确定要删除此 Token 吗？删除后外部系统将无法使用此 Token 发送消息。"
        confirmLabel="删除"
      />
    </div>
  )
}
