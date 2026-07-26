import { create } from 'zustand'

type Theme = 'dark' | 'light'

const STORAGE_KEY = 'clawbot_theme'

function getInitialTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === 'light' || stored === 'dark') return stored
  // 跟随系统偏好
  if (window.matchMedia?.('(prefers-color-scheme: light)').matches) return 'light'
  return 'dark'
}

function applyTheme(theme: Theme) {
  document.documentElement.setAttribute('data-theme', theme)
  localStorage.setItem(STORAGE_KEY, theme)
}

interface ThemeState {
  theme: Theme
  toggle(): void
  setTheme(theme: Theme): void
}

export const useThemeStore = create<ThemeState>((set, get) => {
  const initial = getInitialTheme()
  applyTheme(initial)

  return {
    theme: initial,

    toggle() {
      const next = get().theme === 'dark' ? 'light' : 'dark'
      applyTheme(next)
      set({ theme: next })
    },

    setTheme(theme: Theme) {
      applyTheme(theme)
      set({ theme })
    },
  }
})
