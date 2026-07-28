import { createBrowserRouter, Navigate, Outlet } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import App from './App'
import { ToastProvider } from './components/Toast'

const LoginPage = lazy(() => import('./pages/LoginPage'))
const DashboardPage = lazy(() => import('./pages/DashboardPage'))
const ChannelsPage = lazy(() => import('./pages/ChannelsPage'))
const ManagePage = lazy(() => import('./pages/ManagePage'))
const SettingsPage = lazy(() => import('./pages/SettingsPage'))
const NotificationPage = lazy(() => import('./pages/NotificationPage'))
const LogPage = lazy(() => import('./pages/LogPage'))

function LoadingFallback() {
  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
      <span className="spinner" />
    </div>
  )
}

function RootLayout() {
  return (
    <ToastProvider>
      <Suspense fallback={<LoadingFallback />}>
        <Outlet />
      </Suspense>
    </ToastProvider>
  )
}

export const router = createBrowserRouter([
  {
    element: <RootLayout />,
    children: [
      { path: '/login', element: <LoginPage /> },
      {
        element: <App />,
        children: [
          { index: true, element: <Navigate to="/dashboard" replace /> },
          { path: '/dashboard', element: <DashboardPage /> },
          { path: '/channels', element: <ChannelsPage /> },
          { path: '/manage', element: <ManagePage /> },
          { path: '/notify', element: <NotificationPage /> },
          { path: '/logs', element: <LogPage /> },
          { path: '/settings', element: <SettingsPage /> },
        ],
      },
    ],
  },
])
