import { useEffect } from 'react'
import { useNavigate, Outlet } from 'react-router-dom'
import { useAuthStore } from './stores/auth'
import { api } from './api/client'
import AppShell from './components/AppShell'

export default function App() {
  const { authenticated, checkAuth, logout } = useAuthStore()
  const navigate = useNavigate()

  useEffect(() => {
    checkAuth()
  }, [])

  useEffect(() => {
    api.setOnUnauthorized(() => {
      logout()
      navigate('/login', { replace: true })
    })
    return () => api.setOnUnauthorized(null)
  }, [logout, navigate])

  if (!authenticated) return null

  return (
    <AppShell>
      <Outlet />
    </AppShell>
  )
}