import { useEffect, useState } from 'react'
import { useAccountsStore } from '../stores/accounts'
import { api } from '../api/client'
import { useToast } from '../components/Toast'
import QrModal from '../components/QrModal'
import Button from '../components/ui/Button'
import Input from '../components/ui/Input'
import Tag from '../components/ui/Tag'
import Textarea from '../components/ui/Textarea'
import Select from '../components/ui/Select'
import EmptyState from '../components/ui/EmptyState'
import ConfirmDialog from '../components/ui/ConfirmDialog'
import { ListItemSkeleton } from '../components/ui/Skeleton'

export default function ChannelsPage() {
  const accounts = useAccountsStore()
  const { toast } = useToast()
  const [qrVisible, setQrVisible] = useState(false)
  const [disconnectTarget, setDisconnectTarget] = useState<string | null>(null)

  const [sendMode, setSendMode] = useState<'broadcast' | 'to-one'>('broadcast')
  const [toUser, setToUser] = useState('')
  const [content, setContent] = useState('')
  const [sending, setSending] = useState(false)
  const [sendResult, setSendResult] = useState('')

  const [testMsg, setTestMsg] = useState('')
  const [testToUser, setTestToUser] = useState('')
  const [testLoading, setTestLoading] = useState(false)
  const [testReply, setTestReply] = useState('')

  useEffect(() => {
    accounts.fetch()
  }, [])

  const handleDisconnect = async (id: string) => {
    try {
      await accounts.disconnect(id)
      toast('账号已断开', 'success')
    } catch {
      toast('断开失败', 'error')
    } finally {
      setDisconnectTarget(null)
    }
  }

  const handleSend = async () => {
    if (!content.trim()) return
    setSending(true)
    setSendResult('')
    try {
      let res: { success?: boolean; reply?: string }
      if (sendMode === 'broadcast') {
        res = await api.post<{ success?: boolean }>('/api/v1/message/broadcast', {
          content: content.trim(),
          msg_type: 1,
        })
      } else {
        res = await api.post<{ success?: boolean; reply?: string }>('/api/v1/message/send', {
          content: content.trim(),
          msg_type: 1,
          to_user: toUser,
          wait_reply: false,
        })
      }
      if (res.success !== false) {
        toast('消息发送成功', 'success')
        setSendResult(res.reply ?? '发送成功')
      } else {
        toast('发送失败', 'error')
        setSendResult('发送失败')
      }
    } catch {
      toast('发送请求失败', 'error')
      setSendResult('发送请求失败')
    } finally {
      setSending(false)
    }
  }

  const handleTest = async () => {
    if (!testMsg.trim()) return
    setTestLoading(true)
    setTestReply('')
    try {
      const res = await api.post<{ reply?: string }>('/api/v1/message/send', {
        content: testMsg.trim(),
        msg_type: 1,
        to_user: testToUser || undefined,
        wait_reply: true,
      })
      setTestReply(res.reply ?? '（无回复）')
    } catch {
      setTestReply('发送失败')
    } finally {
      setTestLoading(false)
    }
  }

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1>通道</h1>
          <p>微信账号与消息通道管理</p>
        </div>
        <Button onClick={() => setQrVisible(true)}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="14" y="14" width="7" height="7" /><rect x="3" y="14" width="7" height="7" />
          </svg>
          扫码绑定
        </Button>
      </div>

      <div className="dashboard-content">
        {/* 微信账号 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">微信账号</h2>
          </div>
          {accounts.loading ? (
            <div className="list-section">
              <ListItemSkeleton />
              <ListItemSkeleton />
            </div>
          ) : accounts.items.length === 0 ? (
            <EmptyState
              icon={
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="14" y="14" width="7" height="7" /><rect x="3" y="14" width="7" height="7" />
                </svg>
              }
              title="暂无绑定账号"
              description="扫描二维码绑定微信账号"
              action={
                <Button onClick={() => setQrVisible(true)}>
                  扫码绑定
                </Button>
              }
            />
          ) : (
            <div className="list-section">
              {accounts.items.map((a) => (
                <div key={a.account_id} className="list-item">
                  <div className="list-item-content">
                    <div className="avatar avatar-green">
                      {a.user_id.charAt(0).toUpperCase()}
                    </div>
                    <div className="list-item-info">
                      <div className="list-item-title">{a.user_id}</div>
                      <div className="list-item-subtitle">
                        登录于 {new Date(a.login_at * 1000).toLocaleString('zh-CN')}
                      </div>
                    </div>
                  </div>
                  <div className="list-item-actions">
                    <Tag variant={a.status === 'online' ? 'success' : 'danger'}>
                      {a.status === 'online' ? '在线' : '离线'}
                    </Tag>
                    <Button
                      variant="ghost-danger"
                      size="sm"
                      onClick={() => setDisconnectTarget(a.account_id)}
                    >
                      断开
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 消息发送 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">消息发送</h2>
          </div>
          <div className="channels-send-form">
            <div className="form-grid" style={{ gridTemplateColumns: 'auto 1fr' }}>
              <Select
                value={sendMode}
                onChange={(e) => setSendMode(e.target.value as typeof sendMode)}
                label="发送方式"
              >
                <option value="broadcast">广播 (全部账号)</option>
                <option value="to-one">指定账号</option>
              </Select>
              {sendMode === 'to-one' && (
                <Select
                  value={toUser}
                  onChange={(e) => setToUser(e.target.value)}
                  label="接收方"
                >
                  <option value="">选择账号</option>
                  {accounts.items.map((a) => (
                    <option key={a.account_id} value={a.user_id}>{a.user_id}</option>
                  ))}
                </Select>
              )}
            </div>

            <Textarea
              placeholder="输入消息内容..."
              value={content}
              onChange={(e) => setContent(e.target.value)}
              style={{ minHeight: '80px' }}
            />

            <div style={{ display: 'flex', gap: 'var(--space-3)', alignItems: 'flex-start' }}>
              <Button
                onClick={handleSend}
                loading={sending}
                disabled={!content.trim() || (sendMode === 'to-one' && !toUser)}
              >
                发送
              </Button>
              {sendResult && (
                <div className="code-block" style={{ flex: 1 }}>
                  {sendResult}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* 测试消息 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">测试消息</h2>
          </div>
          <div className="channels-send-form">
            <Select
              value={testToUser}
              onChange={(e) => setTestToUser(e.target.value)}
              label="接收方"
              hint="留空则仅测试路由，不实际发送"
            >
              <option value="">仅测试路由（不发送）</option>
              {accounts.items.map((a) => (
                <option key={a.account_id} value={a.user_id}>{a.user_id}</option>
              ))}
            </Select>

            <Input
              id="test-message"
              label="测试消息"
              placeholder="输入测试消息..."
              value={testMsg}
              onChange={(e) => setTestMsg(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleTest() }}
            />

            <div style={{ display: 'flex', gap: 'var(--space-3)', alignItems: 'flex-start' }}>
              <Button
                variant="secondary"
                onClick={handleTest}
                loading={testLoading}
                disabled={!testMsg.trim()}
              >
                发送并等待回复
              </Button>
              {testReply && (
                <div className="code-block" style={{ flex: 1 }}>
                  {testReply}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      <QrModal visible={qrVisible} onClose={() => { setQrVisible(false); accounts.fetch() }} />

      <ConfirmDialog
        open={!!disconnectTarget}
        onClose={() => setDisconnectTarget(null)}
        onConfirm={() => disconnectTarget && handleDisconnect(disconnectTarget)}
        title="断开账号"
        description="确定要断开此微信账号吗？断开后将无法接收消息。"
        confirmLabel="断开"
      />
    </div>
  )
}
