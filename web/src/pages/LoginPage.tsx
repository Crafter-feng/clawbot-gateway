import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import LoginForm from '../components/LoginForm'

export default function LoginPage() {
  const { authenticated } = useAuthStore()
  const navigate = useNavigate()

  useEffect(() => {
    if (authenticated) {
      navigate('/dashboard', { replace: true })
    }
  }, [authenticated, navigate])

  if (authenticated) return null

  return <LoginForm />
}