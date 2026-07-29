import { useEffect, useState, useCallback } from 'react'
import { api } from '../api/client'
import { useToast } from '../components/Toast'
import { useBackendsStore } from '../stores/backends'
import { useThemeStore } from '../stores/theme'
import Button from '../components/ui/Button'
import Input from '../components/ui/Input'
import Select from '../components/ui/Select'
import Skeleton from '../components/ui/Skeleton'

interface Settings {
  default_backend?: string
  notify_token?: string
  [key: string]: string | undefined
}

export default function SettingsPage() {
  const { toast } = useToast()
  const [settings, setSettings] = useState<Settings>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const backends = useBackendsStore()
  const theme = useThemeStore(s => s.theme)

  /* Password change */
  const [pwdOld, setPwdOld] = useState('')
  const [pwdNew, setPwdNew] = useState('')
  const [pwdConfirm, setPwdConfirm] = useState('')
  const [pwdLoading, setPwdLoading] = useState(false)

  useEffect(() => {
    fetchSettings()
    backends.fetch()
  }, [])

  const fetchSettings = async () => {
    setLoading(true)
    try {
      const res = await api.get<{ settings: Settings }>('/api/v1/config')
      setSettings(res.settings || {})
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
      toast('设置已保存', 'success')
    } catch {
      toast('保存失败', 'error')
    } finally {
      setSaving(false)
    }
  }, [settings, toast])

  const updateSetting = (key: string, value: string) => {
    setSettings(prev => ({ ...prev, [key]: value }))
  }

  const handleChangePassword = useCallback(async () => {
    if (!pwdOld) { toast('请输入旧密码', 'error'); return }
    if (!pwdNew) { toast('请输入新密码', 'error'); return }
    if (pwdNew !== pwdConfirm) { toast('两次输入的新密码不一致', 'error'); return }
    if (pwdNew.length < 6) { toast('新密码至少 6 位', 'error'); return }
    setPwdLoading(true)
    try {
      await api.put('/api/v1/auth/password', { old_password: pwdOld, new_password: pwdNew })
      toast('密码修改成功', 'success')
      setPwdOld('')
      setPwdNew('')
      setPwdConfirm('')
    } catch (e) {
      toast(e instanceof Error ? e.message : '修改密码失败', 'error')
    } finally {
      setPwdLoading(false)
    }
  }, [pwdOld, pwdNew, pwdConfirm, toast])

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
            <Input
              label="会话超时（秒）"
              type="number"
              value={settings['context.ttl'] || '3600'}
              onChange={(e) => updateSetting('context.ttl', e.target.value)}
            />
          </div>
        </div>

        {/* 密码设置 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">密码设置</h2>
          </div>
          <div className="form-grid form-grid-2">
            <Input
              label="旧密码"
              type="password"
              value={pwdOld}
              onChange={(e) => setPwdOld(e.target.value)}
            />
            <div />
            <Input
              label="新密码"
              type="password"
              value={pwdNew}
              onChange={(e) => setPwdNew(e.target.value)}
            />
            <Input
              label="确认新密码"
              type="password"
              value={pwdConfirm}
              onChange={(e) => setPwdConfirm(e.target.value)}
            />
          </div>
          <div style={{ marginTop: 'var(--space-3)' }}>
            <Button onClick={handleChangePassword} loading={pwdLoading} variant="secondary">
              修改密码
            </Button>
          </div>
        </div>

        {/* 主题设置 */}
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">主题设置</h2>
          </div>
          <div className="manage-form">
            <div style={{ display: 'flex', gap: 'var(--space-3)', alignItems: 'center' }}>
              <Button
                onClick={() => useThemeStore.getState().setTheme('light')}
                variant={theme === 'light' ? 'primary' : 'secondary'}
                style={{ flex: 1 }}
              >
                <span style={{ display: 'flex', alignItems: 'center', gap: 6, justifyContent: 'center' }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="12" cy="12" r="5" />
                    <line x1="12" y1="1" x2="12" y2="3" />
                    <line x1="12" y1="21" x2="12" y2="23" />
                    <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
                    <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
                    <line x1="1" y1="12" x2="3" y2="12" />
                    <line x1="21" y1="12" x2="23" y2="12" />
                    <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
                    <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
                  </svg>
                  浅色
                </span>
              </Button>
              <Button
                onClick={() => useThemeStore.getState().setTheme('dark')}
                variant={theme === 'dark' ? 'primary' : 'secondary'}
                style={{ flex: 1 }}
              >
                <span style={{ display: 'flex', alignItems: 'center', gap: 6, justifyContent: 'center' }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M21 12.79A9 9 0 1111.21 3 7 7 0 0021 12.79z" />
                  </svg>
                  深色
                </span>
              </Button>
            </div>
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