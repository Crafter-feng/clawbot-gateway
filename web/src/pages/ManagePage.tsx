import { useEffect, useState } from 'react'
import { useBackendsStore } from '../stores/backends'
import { useRoutesStore } from '../stores/routes'
import { useToast } from '../components/Toast'
interface BackendForm {
  id: string
  name: string
  type: string
  config: {
    api_key: string
    base_url: string
    model: string
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

  /* Route form */
  const [rKeyword, setRKeyword] = useState('')
  const [rBackend, setRBackend] = useState('')
  const [rIsRegexp, setRIsRegexp] = useState(false)
  const [rLoading, setRLoading] = useState(false)

  /* Default backend */
  const [defBackend, setDefBackend] = useState('')
  const [defSaving, setDefSaving] = useState(false)

  useEffect(() => {
    backends.fetch()
    routes.fetch()
  }, [])

  useEffect(() => {
    setDefBackend(backends.defaultBackend)
  }, [backends.defaultBackend])

  const handleAddBackend = async () => {
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
  }

  const handleRemoveBackend = async (id: string) => {
    try {
      await backends.remove(id)
      toast('后端已删除', 'success')
    } catch {
      toast('删除失败', 'error')
    }
  }

  const handleAddRoute = async () => {
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
  }

  const handleRemoveRoute = async (index: number) => {
    try {
      await routes.remove(index)
      toast('路由规则已删除', 'success')
    } catch {
      toast('删除失败', 'error')
    }
  }

  const handleSaveDefault = async () => {
    if (!defBackend) return
    setDefSaving(true)
    try {
      await backends.setDefault(defBackend)
      toast('默认后端已更新', 'success')
    } catch {
      toast('设置失败', 'error')
    } finally {
      setDefSaving(false)
    }
  }

  return (
    <div>
      <div className="page-header">
        <h1>管理</h1>
        <p>后端服务与路由规则配置</p>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
        {/* 后端管理 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>后端管理</div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '16px' }}>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>ID</label>
              <input className="input" placeholder="唯一标识" value={bId} onChange={(e) => setBId(e.target.value)} />
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
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      <span className={`tag ${b.healthy ? 'tag-success' : 'tag-danger'}`}>
                        {b.healthy ? '健康' : '异常'}
                      </span>
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
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>路由规则</div>
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
                        转发至: {r.backend}
                      </div>
                    </div>
                    <button className="btn btn-danger btn-sm" onClick={() => handleRemoveRoute(i)}>删除</button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* 默认后端 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>默认后端</div>
          <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-end' }}>
            <div style={{ flex: 1 }}>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>选择默认后端</label>
              <select
                className="select"
                value={defBackend}
                onChange={(e) => setDefBackend(e.target.value)}
              >
                <option value="">未设置</option>
                {backends.items.map((b) => (
                  <option key={b.id} value={b.id}>{b.name} ({b.id})</option>
                ))}
              </select>
            </div>
            <button
              className="btn btn-primary btn-sm"
              onClick={handleSaveDefault}
              disabled={defSaving || !defBackend}
              style={{ marginBottom: '1px' }}
            >
              {defSaving ? <span className="spinner spinner-sm" /> : null}
              保存
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}