import { useEffect, useState, useCallback } from 'react'
import { useBackendsStore } from '../stores/backends'
import { useRoutesStore, type RouteRule } from '../stores/routes'
import { useToast } from '../components/Toast'
import Button from '../components/ui/Button'
import Input from '../components/ui/Input'
import Select from '../components/ui/Select'
import Tag from '../components/ui/Tag'
import Modal from '../components/ui/Modal'
import ConfirmDialog from '../components/ui/ConfirmDialog'
import EmptyState from '../components/ui/EmptyState'
import { ListItemSkeleton } from '../components/ui/Skeleton'
import RouteRuleForm from '../components/ui/RouteRuleForm'
import RouteRuleList from '../components/ui/RouteRuleList'

interface BackendForm {
  id: string
  name: string
  type: string
  config: {
    api_key?: string
    base_url?: string
    model?: string
  }
}

export default function ManagePage() {
  const backends = useBackendsStore()
  const routes = useRoutesStore()
  const { toast } = useToast()

  /* Backend form */
  const [bId, setBId] = useState('')
  const [bName, setBName] = useState('')
  const [bType, setBType] = useState('echo')
  const [bApiKey, setBApiKey] = useState('')
  const [bBaseUrl, setBBaseUrl] = useState('')
  const [bModel, setBModel] = useState('')
  const [bLoading, setBLoading] = useState(false)

  /* Edit modal */
  const [editModal, setEditModal] = useState<{ open: boolean; backend: BackendForm | null }>({ open: false, backend: null })
  const [editName, setEditName] = useState('')
  const [editType, setEditType] = useState('')
  const [editApiKey, setEditApiKey] = useState('')
  const [editBaseUrl, setEditBaseUrl] = useState('')
  const [editModel, setEditModel] = useState('')
  const [editLoading, setEditLoading] = useState(false)

  /* Info modal */
  const [infoModal, setInfoModal] = useState<{ open: boolean; backend: { id: string; name: string; type: string; healthy: boolean } | null }>({ open: false, backend: null })
  const [testResult, setTestResult] = useState<{ loading: boolean; result?: { healthy: boolean; reply?: string; error?: string } }>({ loading: false })

  /* Delete confirm */
  const [deleteTarget, setDeleteTarget] = useState<{ type: 'backend' | 'route'; id: string | number } | null>(null)

  /* Route rule form */
  const [showRouteForm, setShowRouteForm] = useState(false)
  const [editingRule, setEditingRule] = useState<RouteRule | null>(null)

  useEffect(() => {
    backends.fetch()
    routes.fetch()
  }, [])

  const handleAddBackend = useCallback(async () => {
    if (!bId.trim() || !bName.trim()) return
    setBLoading(true)
    try {
      const form: BackendForm = {
        id: bId.trim(),
        name: bName.trim(),
        type: bType,
        config: { api_key: bApiKey, base_url: bBaseUrl, model: bModel },
      }
      await backends.register(form)
      setBId('')
      setBName('')
      setBType('echo')
      setBApiKey('')
      setBBaseUrl('')
      setBModel('')
      toast('后端添加成功', 'success')
    } catch {
      toast('添加后端失败', 'error')
    } finally {
      setBLoading(false)
    }
  }, [bId, bName, bType, bApiKey, bBaseUrl, bModel, backends, toast])

  const handleRemoveBackend = useCallback(async (id: string) => {
    try {
      await backends.remove(id)
      toast('后端已删除', 'success')
    } catch {
      toast('删除失败', 'error')
    } finally {
      setDeleteTarget(null)
    }
  }, [backends, toast])

  const handleOpenEdit = useCallback(async (b: { id: string; name: string; type: string }) => {
    const detail = await backends.get(b.id)
    setEditModal({ open: true, backend: { id: b.id, name: b.name, type: b.type, config: {} } })
    setEditName(b.name)
    setEditType(b.type)
    if (detail?.config) {
      try {
        const config = JSON.parse(detail.config)
        setEditApiKey(config.api_key || '')
        setEditBaseUrl(config.base_url || '')
        setEditModel(config.model || '')
      } catch {
        setEditApiKey('')
        setEditBaseUrl('')
        setEditModel('')
      }
    } else {
      setEditApiKey('')
      setEditBaseUrl('')
      setEditModel('')
    }
  }, [backends])

  const handleSaveEdit = useCallback(async () => {
    if (!editModal.backend) return
    setEditLoading(true)
    try {
      const config: Record<string, string> = {}
      if (editType === 'openai_compatible') {
        if (editApiKey) config.api_key = editApiKey
        if (editBaseUrl) config.base_url = editBaseUrl
        if (editModel) config.model = editModel
      }
      await backends.update(editModal.backend.id, {
        name: editName || editModal.backend.name,
        type: editType,
        config,
      })
      setEditModal({ open: false, backend: null })
      toast('后端更新成功', 'success')
    } catch {
      toast('更新失败', 'error')
    } finally {
      setEditLoading(false)
    }
  }, [editModal, editName, editType, editApiKey, editBaseUrl, editModel, backends, toast])

  const handleOpenInfo = useCallback((b: { id: string; name: string; type: string; healthy: boolean }) => {
    setInfoModal({ open: true, backend: b })
    setTestResult({ loading: false })
  }, [])

  const handleTestBackend = useCallback(async (id: string) => {
    setTestResult({ loading: true })
    try {
      const result = await backends.test(id)
      setTestResult({ loading: false, result })
    } catch {
      setTestResult({ loading: false, result: { healthy: false, error: '测试失败' } })
    }
  }, [backends])

  const handleRemoveRoute = useCallback(async (id: number) => {
    try {
      await routes.remove(id)
      toast('路由规则已删除', 'success')
    } catch {
      toast('删除失败', 'error')
    } finally {
      setDeleteTarget(null)
    }
  }, [routes, toast])

  return (
    <div>
      <div className="page-header">
        <h1>管理</h1>
        <p>后端服务配置与路由规则</p>
      </div>

      <div className="dashboard-content">
        {/* 后端管理 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">后端管理</h2>
          </div>

          <div className="manage-form">
            <div className="form-grid form-grid-2">
              <Input
                label="ID"
                placeholder="唯一标识（如 hermes, openclaw）"
                value={bId}
                onChange={(e) => setBId(e.target.value)}
              />
              <Input
                label="名称"
                placeholder="显示名称"
                value={bName}
                onChange={(e) => setBName(e.target.value)}
              />
              <Select
                label="类型"
                value={bType}
                onChange={(e) => setBType(e.target.value)}
              >
                <option value="echo">Echo 调试</option>
                <option value="openai_compatible">OpenAI 兼容</option>
                <option value="ilink_proxy">iLink 代理</option>
              </Select>
            </div>

            {bType === 'openai_compatible' && (
              <div className="form-grid form-grid-2">
                <Input
                  label="API Key"
                  type="password"
                  placeholder="sk-..."
                  value={bApiKey}
                  onChange={(e) => setBApiKey(e.target.value)}
                />
                <Input
                  label="Base URL"
                  placeholder="https://api.openai.com/v1"
                  value={bBaseUrl}
                  onChange={(e) => setBBaseUrl(e.target.value)}
                />
                <Input
                  label="模型"
                  placeholder="gpt-4o"
                  value={bModel}
                  onChange={(e) => setBModel(e.target.value)}
                />
              </div>
            )}

            {bType === 'ilink_proxy' && (
              <div className="manage-config-preview">
                <div className="manage-config-title">iLink 代理配置预览</div>
                <p className="manage-config-hint">外部服务需要配置以下环境变量连接到此 Gateway</p>
                <pre className="code-block">{`# ${bName || '后端名称'} 配置
# 外部服务需要配置以下环境变量

WEIXIN_BASE_URL=http://localhost:8080
WEIXIN_TOKEN=gw_${bId || '<id>'}
WEIXIN_ACCOUNT_ID=gw_${bId || '<id>'}`}</pre>
              </div>
            )}

            <Button
              onClick={handleAddBackend}
              loading={bLoading}
              disabled={!bId.trim() || !bName.trim()}
            >
              添加后端
            </Button>
          </div>

          <div className="divider" />

          <div className="card-header">
            <h3 className="card-title" style={{ fontSize: 'var(--text-sm)' }}>已注册后端</h3>
          </div>

          {backends.loading ? (
            <div className="list-section">
              <ListItemSkeleton />
              <ListItemSkeleton />
            </div>
          ) : backends.items.length === 0 ? (
            <EmptyState
              icon={
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="2" y="2" width="20" height="8" rx="2" ry="2" />
                  <rect x="2" y="14" width="20" height="8" rx="2" ry="2" />
                </svg>
              }
              title="暂无后端"
              description="添加后端服务以处理消息"
            />
          ) : (
            <div className="list-section">
              {backends.items.map((b) => (
                <div key={b.id} className="list-item">
                  <div className="list-item-content">
                    <div className="status-dot" style={{ background: b.healthy ? 'var(--success)' : 'var(--danger)' }} />
                    <div className="list-item-info">
                      <div className="list-item-title">{b.name}</div>
                      <div className="list-item-subtitle">
                        {b.id} · <Tag variant="neutral">{b.type}</Tag>
                      </div>
                    </div>
                  </div>
                  <div className="list-item-actions">
                    <Tag variant={b.healthy ? 'success' : 'danger'}>
                      {b.healthy ? '健康' : '异常'}
                    </Tag>
                    <Button variant="ghost" size="sm" onClick={() => handleOpenInfo(b)}>详情</Button>
                    <Button variant="ghost" size="sm" onClick={() => handleOpenEdit(b)}>编辑</Button>
                    <Button variant="ghost-danger" size="sm" onClick={() => setDeleteTarget({ type: 'backend', id: b.id })}>删除</Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 路由规则 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">路由规则</h2>
            <Button size="sm" onClick={() => { setEditingRule(null); setShowRouteForm(true) }}>
              添加规则
            </Button>
          </div>
          <p className="card-description">根据消息内容自动路由到指定后端，支持且/或/非逻辑组合</p>

          {routes.loading ? (
            <div className="list-section">
              <ListItemSkeleton />
            </div>
          ) : (
            <RouteRuleList onEdit={(rule) => { setEditingRule(rule); setShowRouteForm(true) }} onDelete={(id) => setDeleteTarget({ type: 'route', id })} />
          )}
        </div>
      </div>

      {/* 编辑模态框 */}
      <Modal
        open={editModal.open}
        onClose={() => setEditModal({ open: false, backend: null })}
        title={`编辑后端: ${editModal.backend?.id}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditModal({ open: false, backend: null })}>
              取消
            </Button>
            <Button onClick={handleSaveEdit} loading={editLoading}>
              保存
            </Button>
          </>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
          <Input
            label="名称"
            value={editName}
            onChange={(e) => setEditName(e.target.value)}
          />
          <Select
            label="类型"
            value={editType}
            onChange={(e) => setEditType(e.target.value)}
          >
            <option value="echo">Echo 调试</option>
            <option value="openai_compatible">OpenAI 兼容</option>
            <option value="ilink_proxy">iLink 代理</option>
          </Select>

          {editType === 'openai_compatible' && (
            <>
              <Input
                label="API Key"
                type="password"
                placeholder="留空则不修改"
                value={editApiKey}
                onChange={(e) => setEditApiKey(e.target.value)}
              />
              <Input
                label="Base URL"
                placeholder="留空则不修改"
                value={editBaseUrl}
                onChange={(e) => setEditBaseUrl(e.target.value)}
              />
              <Input
                label="模型"
                placeholder="留空则不修改"
                value={editModel}
                onChange={(e) => setEditModel(e.target.value)}
              />
            </>
          )}
        </div>
      </Modal>

      {/* 详情模态框 */}
      <Modal
        open={infoModal.open}
        onClose={() => setInfoModal({ open: false, backend: null })}
        title="后端详情"
        footer={
          <Button variant="secondary" onClick={() => setInfoModal({ open: false, backend: null })}>
            关闭
          </Button>
        }
      >
        {infoModal.backend && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
            <div className="manage-info-row">
              <span className="text-muted">ID</span>
              <span className="font-mono">{infoModal.backend.id}</span>
            </div>
            <div className="manage-info-row">
              <span className="text-muted">名称</span>
              <span>{infoModal.backend.name}</span>
            </div>
            <div className="manage-info-row">
              <span className="text-muted">类型</span>
              <span>{infoModal.backend.type}</span>
            </div>
            <div className="manage-info-row">
              <span className="text-muted">状态</span>
              <Tag variant={infoModal.backend.healthy ? 'success' : 'danger'}>
                {infoModal.backend.healthy ? '健康' : '异常'}
              </Tag>
            </div>

            {infoModal.backend.type === 'ilink_proxy' && (
              <>
                <div className="divider" />
                <div className="input-label">连接配置</div>
                <pre className="code-block">{`WEIXIN_BASE_URL=http://localhost:8080
WEIXIN_TOKEN=gw_${infoModal.backend.id}
WEIXIN_ACCOUNT_ID=gw_${infoModal.backend.id}`}</pre>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    const config = `WEIXIN_BASE_URL=http://localhost:8080\nWEIXIN_TOKEN=gw_${infoModal.backend!.id}\nWEIXIN_ACCOUNT_ID=gw_${infoModal.backend!.id}`
                    navigator.clipboard.writeText(config)
                    toast('配置已复制', 'success')
                  }}
                >
                  复制配置
                </Button>
              </>
            )}

            <div className="divider" />
            <div className="input-label">测试连接</div>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => handleTestBackend(infoModal.backend!.id)}
              loading={testResult.loading}
            >
              发送测试消息
            </Button>
            {testResult.result && (
              <div
                className="code-block"
                style={{
                  background: testResult.result.healthy ? 'var(--success-dim)' : 'var(--danger-dim)',
                }}
              >
                {testResult.result.healthy ? (
                  `测试成功: ${testResult.result.reply || 'OK'}`
                ) : (
                  `测试失败: ${testResult.result.error || '未知错误'}`
                )}
              </div>
            )}
          </div>
        )}
      </Modal>

      {/* 删除确认 */}
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => {
          if (!deleteTarget) return
          if (deleteTarget.type === 'backend') {
            handleRemoveBackend(deleteTarget.id as string)
          } else {
            handleRemoveRoute(deleteTarget.id as number)
          }
        }}
        title={deleteTarget?.type === 'backend' ? '删除后端' : '删除路由规则'}
        description={deleteTarget?.type === 'backend'
          ? '确定要删除此后端吗？此操作不可撤销，相关路由规则将失效。'
          : '确定要删除此路由规则吗？此操作不可撤销。'
        }
      />

      {/* 路由规则表单模态框 */}
      <Modal
        open={showRouteForm}
        onClose={() => { setShowRouteForm(false); setEditingRule(null) }}
        title={editingRule ? '编辑路由规则' : '添加路由规则'}
        maxWidth="680px"
      >
        <RouteRuleForm
          rule={editingRule}
          onClose={() => { setShowRouteForm(false); setEditingRule(null) }}
          onSave={() => { setShowRouteForm(false); setEditingRule(null); routes.fetch() }}
        />
      </Modal>
    </div>
  )
}
