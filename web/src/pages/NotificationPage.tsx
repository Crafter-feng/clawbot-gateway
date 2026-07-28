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
  to_user: string
  name: string
  token: string
  enabled: boolean
  created_at: string
}


export default function NotificationPage() {
  const { toast } = useToast()
  const [tokens, setTokens] = useState<Token[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [newToUser, setNewToUser] = useState('')
  const [creating, setCreating] = useState(false)
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [showToken, setShowToken] = useState<{ id: string; token: string } | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const copyTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)
  const [testToken, setTestToken] = useState('')
  const [testContent, setTestContent] = useState('')
  const [testSending, setTestSending] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; msg: string } | null>(null)
  const [testTokenSelect, setTestTokenSelect] = useState('')
  const handleTestSend = useCallback(async () => {
    const token = testTokenSelect || testToken
    if (!token || !testContent.trim()) return
    setTestSending(true)
    setTestResult(null)
    try {
      const res = await fetch('/api/v1/notify/send', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ content: testContent.trim() }),
      })
      const data = await res.json()
      if (res.ok) {
        setTestResult({ success: true, msg: '推送成功' })
        toast('推送成功', 'success')
      } else {
        setTestResult({ success: false, msg: data.error || '推送失败' })
        toast(data.error || '推送失败', 'error')
      }
    } catch {
      setTestResult({ success: false, msg: '网络错误' })
      toast('网络错误', 'error')
    } finally {
      setTestSending(false)
    }
  }, [testToken, testTokenSelect, testContent, toast])


  useEffect(() => {
    fetchData()
  }, [])

  const fetchData = async () => {
    setLoading(true)
    try {
      const res = await api.get<{ tokens: Token[] }>('/api/v1/notify/tokens')
      setTokens(res.tokens || [])
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
        to_user: newToUser,
      })
      toast('Token 创建成功', 'success')
      setShowCreate(false)
      setNewName('')
      setNewToUser('')
      setShowToken({ id: res.id, token: res.token })
      fetchData()
    } catch {
      toast('创建失败', 'error')
    } finally {
      setCreating(false)
    }
  }, [newName, newToUser, toast])

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
      if (navigator.clipboard) {
        await navigator.clipboard.writeText(text)
      } else {
        const textarea = document.createElement('textarea')
        textarea.value = text
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
      }
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

  const getToUserLabel = (toUser: string) => {
    if (!toUser) return '全部客户'
    return toUser
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
                        推送目标: {getToUserLabel(t.to_user)}
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
              <Input
                label="推送目标"
                placeholder="wxid_xxx，留空则推送全部客户"
                value={newToUser}
                onChange={(e) => setNewToUser(e.target.value)}
                hint="指定消息接收者的微信 ID，留空则推送给所有联系人"
              />
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
            <p>1. 创建 Token，设置推送目标（客户微信 ID）</p>
            <p>2. 复制 Token 并保存</p>
            <p>3. 外部系统使用 Token 调用 API 推送消息到绑定的客户</p>
            <p>4. Token 仅在创建时显示，请妥善保管</p>
          </div>

          <div className="manage-config-preview" style={{ marginTop: 'var(--space-4)' }}>
            <div className="manage-config-title">调用示例</div>
            <pre className="code-block">{`curl -X POST ${window.location.origin}/api/v1/notify/send \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer <token>" \\
  -d '{
    "content": "Hello!"
  }'`}</pre>
          </div>
        </div>

        {/* 测试发送 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">测试推送</h2>
          </div>
          <div className="manage-form">
            {tokens.length > 0 && (
              <Select
                label="选择 Token"
                value={testTokenSelect}
                onChange={(e) => { setTestTokenSelect(e.target.value); setTestToken('') }}
                hint="选择已有 Token，或手动输入下方 Token"
              >
                <option value="">手动输入</option>
                {tokens.map((t) => (
                  <option key={t.id} value={t.token}>
                    {t.name} ({getToUserLabel(t.to_user)})
                  </option>
                ))}
              </Select>
            )}
            {!testTokenSelect && (
              <Input
                label="Token"
                placeholder="粘贴 Token 进行测试"
                value={testToken}
                onChange={(e) => setTestToken(e.target.value)}
              />
            )}
            <Input
              label="推送内容"
              placeholder="输入要推送的消息内容"
              value={testContent}
              onChange={(e) => setTestContent(e.target.value)}
            />
            <div style={{ display: 'flex', gap: 'var(--space-2)', alignItems: 'center' }}>
              <Button onClick={handleTestSend} loading={testSending} disabled={!(testToken || testTokenSelect) || !testContent.trim()}>
                发送测试
              </Button>
              {testResult && (
                <span style={{ fontSize: '13px', color: testResult.success ? 'var(--success)' : '#ef4444' }}>
                  {testResult.msg}
                </span>
              )}
            </div>
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
