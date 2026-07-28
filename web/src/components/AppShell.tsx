import { type ReactNode } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import { useToast } from './Toast'
import ThemeToggle from './ui/ThemeToggle'

const navItems = [
  { path: '/dashboard', label: '仪表盘', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6' },
  { path: '/channels', label: '通道', icon: 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z' },
  { path: '/manage', label: '管理', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z' },
  { path: '/notify', label: '通知', icon: 'M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z' },
  { path: '/logs', label: '日志', icon: 'M4 6h16M4 10h16M4 14h16M4 18h16' },
  { path: '/settings', label: '设置', icon: 'M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z M15 12a3 3 0 11-6 0 3 3 0 016 0z' },
]

export default function AppShell({ children }: { children: ReactNode }) {
  const location = useLocation()
  const navigate = useNavigate()
  const { logout } = useAuthStore()
  const { toast } = useToast()

  const activeIdx = navItems.findIndex((item) => location.pathname.startsWith(item.path))

  const handleLogout = () => {
    logout()
    toast('已退出登录', 'info')
    navigate('/login')
  }

  return (
    <>
      {/* Desktop sidebar */}
      <div className="app-shell-desktop">
        <aside className="app-sidebar" role="navigation" aria-label="主导航">
          {/* Logo */}
          <div className="app-sidebar-logo">
            <div className="app-sidebar-logo-icon">
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M12 2L2 7l10 5 10-5-10-5z" />
                <path d="M2 17l10 5 10-5" />
                <path d="M2 12l10 5 10-5" />
              </svg>
            </div>
            <div>
              <div className="app-sidebar-logo-name">ClawBot</div>
              <div className="app-sidebar-logo-subtitle">消息管理平台</div>
            </div>
          </div>

          {/* Nav items */}
          <nav className="app-nav">
            {navItems.map((item, i) => {
              const isActive = i === activeIdx
              return (
                <button
                  key={item.path}
                  onClick={() => navigate(item.path)}
                  className={`app-nav-item ${isActive ? 'app-nav-item--active' : ''}`}
                  aria-current={isActive ? 'page' : undefined}
                  aria-label={item.label}
                >
                  {isActive && <div className="app-nav-indicator" />}
                  <svg
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={isActive ? 2 : 1.5}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="app-nav-icon"
                    aria-hidden="true"
                  >
                    <path d={item.icon} />
                  </svg>
                  <span>{item.label}</span>
                </button>
              )
            })}
          </nav>

          {/* Bottom */}
          <div className="app-sidebar-footer">
            <ThemeToggle />
            <button
              onClick={handleLogout}
              className="app-nav-item app-nav-item--danger"
              aria-label="退出登录"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4" />
                <polyline points="16 17 21 12 16 7" />
                <line x1="21" y1="12" x2="9" y2="12" />
              </svg>
              退出登录
            </button>
          </div>
        </aside>

        <main className="app-main">
          {children}
        </main>
      </div>

      {/* Mobile bottom nav */}
      <div className="app-shell-mobile">
        <main className="app-main-mobile">
          {children}
        </main>

        <nav className="app-mobile-nav" aria-label="移动端导航">
          {navItems.map((item, i) => {
            const isActive = i === activeIdx
            return (
              <button
                key={item.path}
                onClick={() => navigate(item.path)}
                className={`app-mobile-nav-item ${isActive ? 'app-mobile-nav-item--active' : ''}`}
                aria-current={isActive ? 'page' : undefined}
                aria-label={item.label}
              >
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill={isActive ? 'var(--accent)' : 'none'}
                  stroke="currentColor"
                  strokeWidth={isActive ? 2 : 1.5}
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <path d={item.icon} />
                </svg>
                <span>{item.label}</span>
              </button>
            )
          })}
        </nav>
      </div>
    </>
  )
}
