import { useEffect, useState, useCallback } from 'react'
import { useBackendsStore } from '../stores/backends'
import { useRoutesStore } from '../stores/routes'
import { useToast } from '../components/Toast'

interface BackendForm {
  id: string
  name: string
  type: string
  config: {
    api_key?: string
    base_url?: string
    model?: string
    url?: string
    headers?: Record<string, string>
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

  /* Route form */
  const [rKeyword, setRKeyword] = useState('')
  const [rBackend, setRBackend] = useState('')
  const [rIsRegexp, setRIsRegexp] = useState(false)
  const [rLoading, setRLoading] = useState(false)

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
    }
  }, [backends, toast])

  const handleOpenEdit = useCallback(async (b: { id: string; name: string; type: string }) => {
    await backends.get(b.id)
    setEditModal({ open: true, backend: { id: b.id, name: b.name, type: b.type, config: {} } })
    setEditName(b.name)
    setEditType(b.type)
    setEditApiKey('')
    setEditBaseUrl('')
    setEditModel('')
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

  const handleOpenInfo = useCallback(async (b: { id: string; name: string; type: string; healthy: boolean }) => {
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

  const handleAddRoute = useCallback(async () => {
    if (!rKeyword.trim() || !rBackend.trim()) return
    setRLoading(true)
    try {
      await routes.add(rKeyword.trim(), rBackend.trim(), rIsRegexp)
      setRKeyword('')
      setRBackend('')
      setRIsRegexp(false)
      toast('路由规则添加成功', 'success')
    } catch {
      toast('添加路由规则失败', 'error')
    } finally {
      setRLoading(false)
    }
  }, [rKeyword, rBackend, rIsRegexp, routes, toast])

  const handleRemoveRoute = useCallback(async (index: number) => {
    try {
      await routes.remove(index)
      toast('路由规则已删除', 'success')
    } catch {
      toast('删除失败', 'error')
    }
  }, [routes, toast])

  return (
    <div>
      <div className="page-header">
        <h1>管理</h1>
        <p>后端服务配置与路由规则</p>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>

        {/* 后端管理 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '12px' }}>
            后端管理
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '16px' }}>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>ID</label>
              <input className="input" placeholder="唯一标识（如 hermes, openclaw）" value={bId} onChange={(e) => setBId(e.target.value)} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>名称</label>
              <input className="input" placeholder="显示名称" value={bName} onChange={(e) => setBName(e.target.value)} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>类型</label>
              <select className="select" value={bType} onChange={(e) => setBType(e.target.value)}>
                <option value="echo">Echo 调试</option>
                <option value="openai_compatible">OpenAI 兼容</option>
                <option value="ilink_proxy">iLink 代理</option>
              </select>
            </div>
          </div>

          {bType === 'openai_compatible' && (
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '16px' }}>
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>API Key</label>
                <input className="input" type="password" placeholder="sk-..." value={bApiKey} onChange={(e) => setBApiKey(e.target.value)} />
              </div>
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>Base URL</label>
                <input className="input" placeholder="https://api.openai.com/v1" value={bBaseUrl} onChange={(e) => setBBaseUrl(e.target.value)} />
              </div>
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>模型</label>
                <input className="input" placeholder="gpt-4o" value={bModel} onChange={(e) => setBModel(e.target.value)} />
              </div>
            </div>
          )}

          {bType === 'ilink_proxy' && (
            <div style={{
              padding: '16px',
              background: 'var(--bg-primary)',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--border)',
              marginBottom: '16px',
            }}>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '8px' }}>
                iLink 代理配置预览
              </div>
              <p style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '12px' }}>
                添加后将自动生成虚拟 Bot 配置，外部服务可通过此配置连接到 Gateway
              </p>
              <pre style={{
                fontSize: '12px',
                fontFamily: 'monospace',
                padding: '12px',
                background: 'var(--bg-secondary)',
                borderRadius: 'var(--radius-sm)',
                overflow: 'auto',
              }}>{`# ${bName || '后端名称'} 配置
# 添加后自动生成以下配置

ILINK_BASE_URL=http://localhost:8080
ILINK_TOKEN=gw_${bId || '<id>'}`}</pre>
              <div style={{ marginTop: '8px', fontSize: '12px', color: 'var(--text-muted)' }}>
                account_id: gw_{bId || '<id>'}
              </div>
            </div>
          )}

          <button className="btn btn-primary btn-sm" onClick={handleAddBackend} disabled={bLoading || !bId.trim() || !bName.trim()}>
            {bLoading ? <span className="spinner spinner-sm" /> : null}
            添加后端
          </button>

          <div style={{ borderTop: '1px solid var(--border)', marginTop: '16px', paddingTop: '16px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '12px' }}>已注册后端</div>
            {backends.items.length === 0 ? (
              <div className="empty-state">暂无后端</div>
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
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span className={`tag ${b.healthy ? 'tag-success' : 'tag-danger'}`}>
                        {b.healthy ? '健康' : '异常'}
                      </span>
                      <button className="btn btn-secondary btn-sm" onClick={() => handleOpenInfo(b)}>详情</button>
                      <button className="btn btn-secondary btn-sm" onClick={() => handleOpenEdit(b)}>编辑</button>
                      <button className="btn btn-danger btn-sm" onClick={() => handleRemoveBackend(b.id)}>删除</button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* 路由规则 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '12px' }}>
            路由规则
          </div>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginBottom: '16px' }}>
            根据消息内容自动路由到指定后端
          </p>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr auto', gap: '12px', alignItems: 'end', marginBottom: '16px' }}>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>关键词</label>
              <input className="input" placeholder="匹配关键词" value={rKeyword} onChange={(e) => setRKeyword(e.target.value)} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>目标后端</label>
              <select className="select" value={rBackend} onChange={(e) => setRBackend(e.target.value)}>
                <option value="">选择后端</option>
                {backends.items.map((b) => (
                  <option key={b.id} value={b.id}>{b.name}</option>
                ))}
              </select>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', paddingBottom: '4px' }}>
              <input
                type="checkbox"
                id="is-regexp"
                checked={rIsRegexp}
                onChange={(e) => setRIsRegexp(e.target.checked)}
                style={{ accentColor: 'var(--accent)' }}
              />
              <label htmlFor="is-regexp" style={{ fontSize: '13px', color: 'var(--text-secondary)', cursor: 'pointer' }}>正则</label>
            </div>
          </div>
          <button className="btn btn-primary btn-sm" onClick={handleAddRoute} disabled={rLoading || !rKeyword.trim() || !rBackend.trim()}>
            {rLoading ? <span className="spinner spinner-sm" /> : null}
            添加规则
          </button>

          <div style={{ borderTop: '1px solid var(--border)', marginTop: '16px', paddingTop: '16px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '12px' }}>现有规则</div>
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
                    <button className="btn btn-danger btn-sm" onClick={() => handleRemoveRoute(i)}>删除</button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

      </div>

      {/* 编辑模态框 */}
      {editModal.open && editModal.backend && (
        <div style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: 'rgba(0,0,0,0.5)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 1000,
        }} onClick={() => setEditModal({ open: false, backend: null })}>
          <div className="card" style={{ width: '480px', maxHeight: '80vh', overflow: 'auto' }} onClick={(e) => e.stopPropagation()}>
            <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>
              编辑后端: {editModal.backend.id}
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>名称</label>
                <input className="input" value={editName} onChange={(e) => setEditName(e.target.value)} />
              </div>
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>类型</label>
                <select className="select" value={editType} onChange={(e) => setEditType(e.target.value)}>
                  <option value="echo">Echo 调试</option>
                  <option value="openai_compatible">OpenAI 兼容</option>
                  <option value="ilink_proxy">iLink 代理</option>
                </select>
              </div>

              {editType === 'openai_compatible' && (
                <>
                  <div>
                    <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>API Key</label>
                    <input className="input" type="password" placeholder="留空则不修改" value={editApiKey} onChange={(e) => setEditApiKey(e.target.value)} />
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>Base URL</label>
                    <input className="input" placeholder="留空则不修改" value={editBaseUrl} onChange={(e) => setEditBaseUrl(e.target.value)} />
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>模型</label>
                    <input className="input" placeholder="留空则不修改" value={editModel} onChange={(e) => setEditModel(e.target.value)} />
                  </div>
                </>
              )}
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', marginTop: '20px' }}>
              <button className="btn btn-secondary btn-sm" onClick={() => setEditModal({ open: false, backend: null })}>取消</button>
              <button className="btn btn-primary btn-sm" onClick={handleSaveEdit} disabled={editLoading}>
                {editLoading ? <span className="spinner spinner-sm" /> : null}
                保存
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 详情模态框 */}
      {infoModal.open && infoModal.backend && (
        <div style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: 'rgba(0,0,0,0.5)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 1000,
        }} onClick={() => setInfoModal({ open: false, backend: null })}>
          <div className="card" style={{ width: '400px' }} onClick={(e) => e.stopPropagation()}>
            <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>
              后端详情
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', fontSize: '13px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ color: 'var(--text-muted)' }}>ID</span>
                <span style={{ fontFamily: 'monospace' }}>{infoModal.backend.id}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ color: 'var(--text-muted)' }}>名称</span>
                <span>{infoModal.backend.name}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ color: 'var(--text-muted)' }}>类型</span>
                <span>{infoModal.backend.type}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ color: 'var(--text-muted)' }}>状态</span>
                <span className={`tag ${infoModal.backend.healthy ? 'tag-success' : 'tag-danger'}`}>
                  {infoModal.backend.healthy ? '健康' : '异常'}
                </span>
              </div>
            </div>

            <div style={{ borderTop: '1px solid var(--border)', marginTop: '16px', paddingTop: '16px' }}>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '8px' }}>测试连接</div>
              <button
                className="btn btn-secondary btn-sm"
                onClick={() => handleTestBackend(infoModal.backend!.id)}
                disabled={testResult.loading}
              >
                {testResult.loading ? <span className="spinner spinner-sm" /> : null}
                发送测试消息
              </button>
              {testResult.result && (
                <div style={{
                  marginTop: '8px',
                  padding: '8px 12px',
                  background: testResult.result.healthy ? 'var(--success-bg)' : 'var(--danger-bg)',
                  borderRadius: 'var(--radius-sm)',
                  fontSize: '12px',
                }}>
                  {testResult.result.healthy ? (
                    <span>测试成功: {testResult.result.reply || 'OK'}</span>
                  ) : (
                    <span>测试失败: {testResult.result.error || '未知错误'}</span>
                  )}
                </div>
              )}
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '16px' }}>
              <button className="btn btn-secondary btn-sm" onClick={() => setInfoModal({ open: false, backend: null })}>关闭</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
