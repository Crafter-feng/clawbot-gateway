import { useEffect } from 'react'
import { useStatsStore } from '../stores/stats'
import { useBackendsStore } from '../stores/backends'
import { useRoutesStore } from '../stores/routes'
import { useAccountsStore } from '../stores/accounts'
import MetricCard from '../components/MetricCard'

export default function DashboardPage() {
  const stats = useStatsStore()
  const backends = useBackendsStore()
  const routes = useRoutesStore()
  const accounts = useAccountsStore()

  useEffect(() => {
    stats.fetch()
    backends.fetch()
    routes.fetch()
    accounts.fetch()
  }, [])

  const onlineBackends = backends.items.filter((b) => b.healthy).length

  return (
    <div>
      <div className="page-header">
        <h1>仪表盘</h1>
        <p>系统运行状态概览</p>
      </div>

      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
        gap: '16px',
        marginBottom: '24px',
      }}>
        <MetricCard label="微信账号" value={accounts.items.length} />
        <MetricCard label="后端数量" value={backends.items.length} />
        <MetricCard label="在线后端" value={onlineBackends} />
        <MetricCard label="路由规则" value={routes.items.length} />
        <MetricCard label="活跃会话" value={stats.sessions} />
        <MetricCard label="消息处理" value={stats.messagesProcessed} />
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
        {/* 微信账号 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>
            微信账号
          </div>
          {accounts.items.length === 0 ? (
            <div className="empty-state">暂未绑定微信账号</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {accounts.items.map((a) => (
                <div key={a.account_id} style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '12px 16px',
                  background: 'var(--bg-primary)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border)',
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                    <span style={{
                      width: '8px',
                      height: '8px',
                      borderRadius: '50%',
                      background: 'var(--success)',
                      flexShrink: 0,
                    }} />
                    <div>
                      <div style={{ fontWeight: 600, fontSize: '14px' }}>{a.user_id}</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        {a.account_id} · {new Date(a.login_at * 1000).toLocaleString('zh-CN')}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 后端状态 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>
            后端服务状态
          </div>
          {backends.items.length === 0 ? (
            <div className="empty-state">暂无后端服务</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {backends.items.map((b) => (
                <div key={b.id} style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '12px 16px',
                  background: 'var(--bg-primary)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border)',
                }}>
                  <div>
                    <div style={{ fontWeight: 600, fontSize: '14px' }}>{b.name}</div>
                    <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>{b.id} · {b.type}</div>
                  </div>
                  <span className={`tag ${b.healthy ? 'tag-success' : 'tag-danger'}`}>
                    {b.healthy ? '健康' : '异常'}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 路由规则 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>
            路由规则
          </div>
          {routes.items.length === 0 ? (
            <div className="empty-state">暂无路由规则</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {routes.items.map((r, i) => (
                <div key={i} style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '12px 16px',
                  background: 'var(--bg-primary)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border)',
                }}>
                  <div>
                    <div style={{ fontWeight: 600, fontSize: '14px' }}>
                      {r.keyword}
                      {r.is_regexp && <span className="tag tag-warning" style={{ marginLeft: '8px' }}>正则</span>}
                    </div>
                    <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                      转发至: {r.backend_id}
                    </div>
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