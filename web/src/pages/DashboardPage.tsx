import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useStatsStore } from '../stores/stats'
import { useBackendsStore } from '../stores/backends'
import { useRoutesStore } from '../stores/routes'
import { useAccountsStore } from '../stores/accounts'
import MetricCard from '../components/MetricCard'
import EmptyState from '../components/ui/EmptyState'
import Tag from '../components/ui/Tag'
import { MetricCardSkeleton } from '../components/ui/Skeleton'
import Button from '../components/ui/Button'

export default function DashboardPage() {
  const stats = useStatsStore()
  const backends = useBackendsStore()
  const routes = useRoutesStore()
  const accounts = useAccountsStore()
  const navigate = useNavigate()

  useEffect(() => {
    stats.fetch()
    backends.fetch()
    routes.fetch()
    accounts.fetch()
  }, [])

  const onlineBackends = backends.items.filter((b) => b.healthy).length
  const isLoading = stats.loading || backends.loading

  return (
    <div>
      <div className="page-header">
        <h1>仪表盘</h1>
        <p>系统运行状态概览</p>
      </div>

      {/* Metric Cards */}
      <div className="dashboard-metrics">
        {isLoading ? (
          <>
            <MetricCardSkeleton />
            <MetricCardSkeleton />
            <MetricCardSkeleton />
            <MetricCardSkeleton />
          </>
        ) : (
          <>
            <MetricCard
              label="微信账号"
              value={accounts.items.length}
              icon={
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
                  <circle cx="9" cy="7" r="4" />
                  <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
                  <path d="M16 3.13a4 4 0 0 1 0 7.75" />
                </svg>
              }
            />
            <MetricCard
              label="后端服务"
              value={`${onlineBackends}/${backends.items.length}`}
              icon={
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="2" y="2" width="20" height="8" rx="2" ry="2" />
                  <rect x="2" y="14" width="20" height="8" rx="2" ry="2" />
                  <line x1="6" y1="6" x2="6.01" y2="6" />
                  <line x1="6" y1="18" x2="6.01" y2="18" />
                </svg>
              }
              trend={backends.items.length > 0 ? { value: onlineBackends, label: '健康' } : undefined}
            />
            <MetricCard
              label="活跃会话"
              value={stats.sessions}
              icon={
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
                </svg>
              }
            />
            <MetricCard
              label="消息处理"
              value={stats.messagesProcessed}
              icon={
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <line x1="22" y1="2" x2="11" y2="13" />
                  <polygon points="22 2 15 22 11 13 2 9 22 2" />
                </svg>
              }
            />
          </>
        )}
      </div>

      {/* Content Sections */}
      <div className="dashboard-content">
        {/* 微信账号 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">微信账号</h2>
            <Button variant="ghost" size="sm" onClick={() => navigate('/channels')}>
              查看全部
            </Button>
          </div>
          {accounts.items.length === 0 ? (
            <EmptyState
              icon={
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
                  <circle cx="9" cy="7" r="4" />
                </svg>
              }
              title="暂无绑定账号"
              description="绑定微信账号以开始使用"
              action={
                <Button size="sm" onClick={() => navigate('/channels')}>
                  绑定账号
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
                        {a.account_id} · {new Date(a.login_at * 1000).toLocaleString('zh-CN')}
                      </div>
                    </div>
                  </div>
                  <Tag variant={a.status === 'online' ? 'success' : 'danger'}>
                    {a.status === 'online' ? '在线' : '离线'}
                  </Tag>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 后端状态 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">后端服务</h2>
            <Button variant="ghost" size="sm" onClick={() => navigate('/manage')}>
              管理
            </Button>
          </div>
          {backends.items.length === 0 ? (
            <EmptyState
              icon={
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="2" y="2" width="20" height="8" rx="2" ry="2" />
                  <rect x="2" y="14" width="20" height="8" rx="2" ry="2" />
                </svg>
              }
              title="暂无后端服务"
              description="添加后端服务以处理消息"
              action={
                <Button size="sm" onClick={() => navigate('/manage')}>
                  添加后端
                </Button>
              }
            />
          ) : (
            <div className="list-section">
              {backends.items.map((b) => (
                <div key={b.id} className="list-item">
                  <div className="list-item-content">
                    <div className={`status-dot ${b.healthy ? 'status-dot-online' : 'status-dot-offline'}`} />
                    <div className="list-item-info">
                      <div className="list-item-title">{b.name}</div>
                      <div className="list-item-subtitle">
                        {b.id} · <Tag variant="neutral">{b.type}</Tag>
                      </div>
                    </div>
                  </div>
                  <Tag variant={b.healthy ? 'success' : 'danger'}>
                    {b.healthy ? '健康' : '异常'}
                  </Tag>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 路由规则 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">路由规则</h2>
            <Button variant="ghost" size="sm" onClick={() => navigate('/manage')}>
              管理
            </Button>
          </div>
          {routes.items.length === 0 ? (
            <EmptyState
              icon={
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="16 3 21 3 21 8" />
                  <line x1="4" y1="20" x2="21" y2="3" />
                  <polyline points="21 16 21 21 16 21" />
                  <line x1="15" y1="15" x2="21" y2="21" />
                  <line x1="4" y1="4" x2="9" y2="9" />
                </svg>
              }
              title="暂无路由规则"
              description="创建路由规则以自动分发消息"
              action={
                <Button size="sm" onClick={() => navigate('/manage')}>
                  创建规则
                </Button>
              }
            />
          ) : (
            <div className="list-section">
              {routes.items.map((r, i) => (
                <div key={i} className="list-item">
                  <div className="list-item-content">
                    <div className="list-item-info">
                      <div className="list-item-title font-mono">
                        {r.keyword}
                        {r.is_regexp && <Tag variant="warning" style={{ marginLeft: '8px' }}>正则</Tag>}
                      </div>
                      <div className="list-item-subtitle">
                        转发至: {r.backend_id}
                      </div>
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
