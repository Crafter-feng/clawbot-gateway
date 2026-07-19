import { useEffect, useState } from 'react'
import { useAccountsStore } from '../stores/accounts'
import { api } from '../api/client'
import { useToast } from '../components/Toast'
import QrModal from '../components/QrModal'

export default function ChannelsPage() {
  const accounts = useAccountsStore()
  const { toast } = useToast()
  const [qrVisible, setQrVisible] = useState(false)

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
        <button className="btn btn-primary" onClick={() => setQrVisible(true)}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="14" y="14" width="7" height="7" /><rect x="3" y="14" width="7" height="7" />
          </svg>
          扫码绑定
        </button>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
        {/* 微信账号 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>
            微信账号
          </div>
          {accounts.items.length === 0 ? (
            <div className="empty-state">
              暂无已绑定的微信账号
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {accounts.items.map((a) => (
                <div key={a.account_id} style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '14px 16px',
                  background: 'var(--bg-primary)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border)',
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                    <div style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: '50%',
                      background: 'linear-gradient(135deg, #34d399, #22c55e)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      fontSize: '14px',
                      color: 'white',
                      fontWeight: 700,
                    }}>
                      {a.user_id.charAt(0).toUpperCase()}
                    </div>
                    <div>
                      <div style={{ fontWeight: 600, fontSize: '14px' }}>{a.user_id}</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        登录于 {new Date(a.login_at * 1000).toLocaleString('zh-CN')}
                      </div>
                    </div>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                    <span className={`tag ${a.status === 'online' ? 'tag-success' : 'tag-danger'}`}>
                      {a.status === 'online' ? '在线' : '离线'}
                    </span>
                    <button
                      className="btn btn-danger btn-sm"
                      onClick={() => handleDisconnect(a.account_id)}
                    >
                      断开
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 消息发送 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>
            消息发送
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
              <label style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-secondary)' }}>发送方式</label>
              <select
                className="select"
                style={{ width: 'auto', minWidth: '140px' }}
                value={sendMode}
                onChange={(e) => setSendMode(e.target.value as typeof sendMode)}
              >
                <option value="broadcast">广播 (全部账号)</option>
                <option value="to-one">指定账号</option>
              </select>
              {sendMode === 'to-one' && (
                <select
                  className="select"
                  style={{ width: 'auto', minWidth: '160px' }}
                  value={toUser}
                  onChange={(e) => setToUser(e.target.value)}
                >
                  <option value="">选择账号</option>
                  {accounts.items.map((a) => (
                    <option key={a.account_id} value={a.user_id}>{a.user_id}</option>
                  ))}
                </select>
              )}
            </div>

            <textarea
              className="input"
              placeholder="输入消息内容..."
              value={content}
              onChange={(e) => setContent(e.target.value)}
              style={{ minHeight: '80px' }}
            />

            <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-start' }}>
              <button
                className="btn btn-primary"
                onClick={handleSend}
                disabled={sending || !content.trim() || (sendMode === 'to-one' && !toUser)}
              >
                {sending ? <span className="spinner spinner-sm" /> : null}
                {sending ? '发送中...' : '发送'}
              </button>
              {sendResult && (
                <div style={{
                  padding: '8px 14px',
                  borderRadius: 'var(--radius-md)',
                  background: 'var(--bg-primary)',
                  border: '1px solid var(--border)',
                  fontSize: '13px',
                  color: 'var(--text-secondary)',
                  flex: 1,
                }}>
                  {sendResult}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* 测试消息 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>
            测试消息
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
              <label style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text-secondary)' }}>接收方</label>
              <select
                className="select"
                style={{ width: 'auto', minWidth: '160px' }}
                value={testToUser}
                onChange={(e) => setTestToUser(e.target.value)}
              >
                <option value="">仅测试路由（不发送）</option>
                {accounts.items.map((a) => (
                  <option key={a.account_id} value={a.user_id}>{a.user_id}</option>
                ))}
              </select>
            </div>
            <input
              className="input"
              placeholder="输入测试消息..."
              value={testMsg}
              onChange={(e) => setTestMsg(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleTest() }}
            />
            <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-start' }}>
              <button
                className="btn btn-secondary"
                onClick={handleTest}
                disabled={testLoading || !testMsg.trim()}
              >
                {testLoading ? <span className="spinner spinner-sm" /> : null}
                {testLoading ? '发送中...' : '发送并等待回复'}
              </button>
              {testReply && (
                <div style={{
                  padding: '10px 14px',
                  borderRadius: 'var(--radius-md)',
                  background: 'var(--bg-primary)',
                  border: '1px solid var(--border)',
                  fontSize: '13px',
                  color: 'var(--text-secondary)',
                  flex: 1,
                  lineHeight: 1.5,
                  whiteSpace: 'pre-wrap',
                }}>
                  {testReply}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      <QrModal visible={qrVisible} onClose={() => { setQrVisible(false); accounts.fetch() }} />
    </div>
  )
}