import { ref } from 'vue'

export type Theme = 'dark' | 'light'

const STORAGE_KEY = 'reqost:theme'

function initial(): Theme {
  const saved = localStorage.getItem(STORAGE_KEY)
  return saved === 'light' ? 'light' : 'dark'
}

// Module-level so every caller shares the one theme.
const theme = ref<Theme>(initial())

function apply(t: Theme) {
  document.documentElement.dataset.theme = t
}
apply(theme.value)

export function useTheme() {
  function toggle() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    apply(theme.value)
    localStorage.setItem(STORAGE_KEY, theme.value)
  }
  return { theme, toggle }
}
