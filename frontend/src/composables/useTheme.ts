import { readonly, ref } from 'vue'

export type Theme = 'light' | 'dark'

const storageKey = 'conduit_theme'
const theme = ref<Theme>('light')

function preferredTheme(): Theme {
  const saved = localStorage.getItem(storageKey)
  if (saved === 'light' || saved === 'dark') return saved
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(value: Theme) {
  theme.value = value
  document.documentElement.classList.toggle('dark', value === 'dark')
  document.documentElement.style.colorScheme = value
  localStorage.setItem(storageKey, value)
}

export function initializeTheme() {
  applyTheme(preferredTheme())
}

export function useTheme() {
  function toggleTheme() {
    applyTheme(theme.value === 'light' ? 'dark' : 'light')
  }

  return { theme: readonly(theme), toggleTheme }
}
