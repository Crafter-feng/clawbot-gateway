import { useState, useCallback, useEffect, createContext, useContext, type ReactNode } from 'react'

type ToastType = 'success' | 'error' | 'info'

interface Toast {
  id: number
  message: string
  type: ToastType
}

interface ToastContextValue {
  toast: (message: string, type?: ToastType) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

let nextId = 0

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const timers = useState<Map<number, ReturnType<typeof setTimeout>>>(new Map())[0]

  const removeToast = useCallback((id: number) => {
    clearTimeout(timers.get(id))
    timers.delete(id)
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [timers])

  const addToast = useCallback((message: string, type: ToastType = 'info') => {
    const id = nextId++
    setToasts((prev) => [...prev, { id, message, type }])
    const timer = setTimeout(() => removeToast(id), 3000)
    timers.set(id, timer)
  }, [removeToast, timers])

  useEffect(() => {
    return () => {
      timers.forEach((timer) => clearTimeout(timer))
    }
  }, [timers])

  return (
    <ToastContext.Provider value={{ toast: addToast }}>
      {children}
      <div className="toast-container">
        {toasts.map((t) => (
          <div key={t.id} className={`toast ${t.type}`}>
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}