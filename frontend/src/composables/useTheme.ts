import { ref, watch } from 'vue'

export type Theme = 'dark' | 'light'
export type ThemePref = 'dark' | 'light' | 'system'

const STORAGE_KEY      = 'reqost:theme'         // legacy: explicit dark/light
const STORAGE_PREF_KEY = 'reqost:theme:pref'    // new: 'system' too
const STORAGE_FONT_KEY = 'reqost:fontSize'

function loadPref(): ThemePref {
  const saved = localStorage.getItem(STORAGE_PREF_KEY)
  if (saved === 'system' || saved === 'light' || saved === 'dark') return saved
  // Fall back to the legacy single-key explicit value, so existing users keep
  // their setting on upgrade.
  const legacy = localStorage.getItem(STORAGE_KEY)
  return legacy === 'light' ? 'light' : 'dark'
}

const systemMql = window.matchMedia?.('(prefers-color-scheme: dark)')

// Module-level so every caller shares the same reactive state.
const pref  = ref<ThemePref>(loadPref())
const theme = ref<Theme>(resolveTheme(pref.value))
const fontSize = ref<number>(Number(localStorage.getItem(STORAGE_FONT_KEY)) || 13)

function resolveTheme(p: ThemePref): Theme {
  if (p === 'system') return systemMql?.matches ? 'dark' : 'light'
  return p
}

function apply(t: Theme) {
  document.documentElement.dataset.theme = t
}
function applyFontSize(px: number) {
  document.documentElement.style.setProperty('--app-font-size', `${px}px`)
}

apply(theme.value)
applyFontSize(fontSize.value)

// Listen to OS-level changes while pref === 'system'.
systemMql?.addEventListener?.('change', () => {
  if (pref.value !== 'system') return
  theme.value = resolveTheme('system')
  apply(theme.value)
})

watch(pref, p => {
  localStorage.setItem(STORAGE_PREF_KEY, p)
  // Keep legacy key in sync so the toggle button keeps working in older code.
  if (p !== 'system') localStorage.setItem(STORAGE_KEY, p)
  theme.value = resolveTheme(p)
  apply(theme.value)
})

watch(fontSize, n => {
  localStorage.setItem(STORAGE_FONT_KEY, String(n))
  applyFontSize(n)
})

export function useTheme() {
  function toggle() {
    // Cycle: light → dark → system → light.
    pref.value = pref.value === 'light' ? 'dark'
              : pref.value === 'dark'  ? 'system'
              : 'light'
  }
  function setPref(p: ThemePref) { pref.value = p }
  function setFontSize(n: number) { fontSize.value = Math.max(10, Math.min(20, n)) }
  return { theme, pref, fontSize, toggle, setPref, setFontSize }
}
