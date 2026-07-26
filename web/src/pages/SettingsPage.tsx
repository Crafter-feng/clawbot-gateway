import { useEffect, useState, useCallback } from 'react'
import { api } from '../api/client'
import { useToast } from '../components/Toast'
import { useBackendsStore } from '../stores/backends'
import Button from '../components/ui/Button'
import Input from '../components/ui/Input'
import Select from '../components/ui/Select'
import Skeleton from '../components/ui/Skeleton'

interface Settings {
  [key: string]: string
}

export default function SettingsPage() {
  const { toast } = useToast()
  const [settings, setSettings] = useState<Settings>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const backends = useBackendsStore()

  useEffect(() => {
    fetchSettings()
    backends.fetch()
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
    return (
      <div>
        <div className="page-header">
          <h1>设置</h1>
          <p>系统配置管理</p>
        </div>
        <div className="dashboard-content">
          <div className="card">
            <Skeleton variant="title" width="30%" />
            <div style={{ marginTop: 'var(--space-4)' }}>
              <Skeleton variant="text" height={40} />
            </div>
          </div>
          <div className="card">
            <Skeleton variant="title" width="30%" />
            <div style={{ marginTop: 'var(--space-4)' }}>
              <Skeleton variant="text" height={40} />
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <h1>设置</h1>
        <p>系统配置管理</p>
      </div>

      <div className="dashboard-content">
        {/* API 设置 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">API 设置</h2>
          </div>
          <div className="form-grid form-grid-2">
            <Input
              label="JWT 有效期（小时）"
              type="number"
              value={settings['api.jwt_expiry_hours'] || '24'}
              onChange={(e) => updateSetting('api.jwt_expiry_hours', e.target.value)}
            />
            <Input
              label="允许来源"
              value={settings['api.allowed_origins'] || '*'}
              onChange={(e) => updateSetting('api.allowed_origins', e.target.value)}
              hint="多个来源用逗号分隔"
            />
          </div>
        </div>

        {/* 会话设置 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">会话设置</h2>
          </div>
          <div className="form-grid form-grid-3">
            <Input
              label="最大历史条数"
              type="number"
              value={settings['context.max_history'] || '20'}
              onChange={(e) => updateSetting('context.max_history', e.target.value)}
            />
            <Select
              label="切换策略"
              value={settings['context.switch_strategy'] || 'keep'}
              onChange={(e) => updateSetting('context.switch_strategy', e.target.value)}
            >
              <option value="keep">保留</option>
              <option value="clear">清空</option>
              <option value="isolated">独立</option>
            </Select>
            <Input
              label="会话超时（秒）"
              type="number"
              value={settings['context.ttl'] || '3600'}
              onChange={(e) => updateSetting('context.ttl', e.target.value)}
            />
          </div>
        </div>

        {/* 路由设置 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">路由设置</h2>
          </div>
          <Select
            label="默认后端"
            value={settings['route.default_backend'] || ''}
            onChange={(e) => updateSetting('route.default_backend', e.target.value)}
            hint="未匹配路由规则时使用的后端"
          >
            <option value="">无（不自动路由）</option>
            {backends.items.map((b) => (
              <option key={b.id} value={b.id}>{b.name} ({b.id})</option>
            ))}
          </Select>
        </div>

        <Button onClick={handleSave} loading={saving}>
          保存设置
        </Button>
      </div>
    </div>
  )
}
