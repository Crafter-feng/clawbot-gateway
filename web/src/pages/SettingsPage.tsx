import { useEffect, useState, useCallback } from 'react'
import { api } from '../api/client'
import { useToast } from '../components/Toast'

interface Settings {
  [key: string]: string
}

export default function SettingsPage() {
  const { toast } = useToast()
  const [settings, setSettings] = useState<Settings>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    fetchSettings()
  }, [])

  const fetchSettings = async () => {
    setLoading(true)
    try {
      const data = await api.get<{ settings: Settings }>('/api/v1/config')
      setSettings(data.settings || {})
    } catch {
      toast('加载配置失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  const handleSave = useCallback(async () => {
    setSaving(true)
    try {
      await api.put('/api/v1/config', settings)
      toast('配置保存成功', 'success')
    } catch {
      toast('保存配置失败', 'error')
    } finally {
      setSaving(false)
    }
  }, [settings, toast])

  const updateSetting = (key: string, value: string) => {
    setSettings(prev => ({ ...prev, [key]: value }))
  }

  if (loading) {
    return <div className="empty-state">加载中...</div>
  }

  return (
    <div>
      <div className="page-header">
        <h1>设置</h1>
        <p>系统配置管理</p>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>

        {/* 服务器设置 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>服务器设置</div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>监听地址</label>
              <input className="input" value={settings['server.host'] || ''} onChange={(e) => updateSetting('server.host', e.target.value)} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>监听端口</label>
              <input className="input" type="number" value={settings['server.port'] || '8080'} onChange={(e) => updateSetting('server.port', e.target.value)} />
            </div>
          </div>
        </div>

        {/* iLink 设置 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>iLink 设置</div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
            <div style={{ gridColumn: 'span 2' }}>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>基础 URL</label>
              <input className="input" value={settings['clawbot.base_url'] || ''} onChange={(e) => updateSetting('clawbot.base_url', e.target.value)} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>轮询超时（秒）</label>
              <input className="input" type="number" value={settings['clawbot.poll_timeout'] || '35'} onChange={(e) => updateSetting('clawbot.poll_timeout', e.target.value)} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>Bot 类型</label>
              <input className="input" type="number" value={settings['clawbot.bot_type'] || '3'} onChange={(e) => updateSetting('clawbot.bot_type', e.target.value)} />
            </div>
          </div>
        </div>

        {/* API 设置 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>API 设置</div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>JWT 有效期（小时）</label>
              <input className="input" type="number" value={settings['api.jwt_expiry_hours'] || '24'} onChange={(e) => updateSetting('api.jwt_expiry_hours', e.target.value)} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>允许来源</label>
              <input className="input" value={settings['api.allowed_origins'] || '*'} onChange={(e) => updateSetting('api.allowed_origins', e.target.value)} />
            </div>
          </div>
        </div>

        {/* 会话设置 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>会话设置</div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '16px' }}>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>最大历史条数</label>
              <input className="input" type="number" value={settings['context.max_history'] || '20'} onChange={(e) => updateSetting('context.max_history', e.target.value)} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>切换策略</label>
              <select className="select" value={settings['context.switch_strategy'] || 'keep'} onChange={(e) => updateSetting('context.switch_strategy', e.target.value)}>
                <option value="keep">保留</option>
                <option value="clear">清空</option>
                <option value="isolated">独立</option>
              </select>
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>会话超时（秒）</label>
              <input className="input" type="number" value={settings['context.ttl'] || '3600'} onChange={(e) => updateSetting('context.ttl', e.target.value)} />
            </div>
          </div>
        </div>

        {/* 路由设置 */}
        <div className="card">
          <div style={{ fontSize: '15px', fontWeight: 600, marginBottom: '16px' }}>路由设置</div>
          <div>
            <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '4px' }}>默认后端</label>
            <input className="input" value={settings['route.default_backend'] || 'echo'} onChange={(e) => updateSetting('route.default_backend', e.target.value)} />
          </div>
        </div>

        <button className="btn btn-primary" onClick={handleSave} disabled={saving}>
          {saving ? <span className="spinner spinner-sm" /> : null}
          保存设置
        </button>
      </div>
    </div>
  )
}
