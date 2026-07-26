import { useState } from 'react'
import { useAuthStore } from '../stores/auth'
import { useToast } from './Toast'

export default function LoginForm() {
  const [password, setPassword] = useState('')
  const { loginLoading, loginError, login } = useAuthStore()
  const { toast } = useToast()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!password.trim()) return
    const ok = await login(password)
    if (ok) {
      toast('登录成功', 'success')
    }
  }

  return (
    <div className="login-screen">
      <div className="login-card">
        <div className="login-header">
          <div className="login-logo">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 2L2 7l10 5 10-5-10-5z" />
              <path d="M2 17l10 5 10-5" />
              <path d="M2 12l10 5 10-5" />
            </svg>
          </div>
          <h1 className="login-title">ClawBot 网关</h1>
          <p className="login-subtitle">微信多后端 AI 代理管理平台</p>
        </div>

        <form onSubmit={handleSubmit} className="login-form">
          <div className="input-group">
            <label className="input-label" htmlFor="login-password">
              管理密码
            </label>
            <input
              id="login-password"
              className="input"
              type="password"
              placeholder="请输入密码"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoFocus
              autoComplete="current-password"
            />
          </div>

          {loginError && (
            <div className="login-error" role="alert">
              {loginError}
            </div>
          )}

          <button
            type="submit"
            className="btn btn-primary btn-lg login-submit"
            disabled={loginLoading || !password.trim()}
          >
            {loginLoading ? (
              <>
                <span className="spinner spinner-sm spinner-white" />
                验证中...
              </>
            ) : (
              '登录管理面板'
            )}
          </button>
        </form>
      </div>
    </div>
  )
}
