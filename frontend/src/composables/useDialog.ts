import { reactive } from 'vue'

// In-app replacements for window.prompt/confirm, which macOS WKWebView (Wails)
// does not implement — they silently return null, breaking every create/rename/
// delete/import flow. These resolve a Promise from a real modal instead.

type Mode = 'prompt' | 'confirm'

const state = reactive({
  open: false,
  mode: 'prompt' as Mode,
  title: '',
  value: '',
  placeholder: '',
  okLabel: 'OK',
  danger: false,
  link: null as null | { url: string; label: string },
  _resolve: null as null | ((v: any) => void),
})

function finish(v: any) {
  const r = state._resolve
  state.open = false
  state._resolve = null
  if (r) r(v)
}

export function useDialog() {
  function prompt(title: string, defaultValue = '', placeholder = '', link?: { url: string; label: string }): Promise<string | null> {
    return new Promise((res) => {
      state.open = true
      state.mode = 'prompt'
      state.title = title
      state.value = defaultValue
      state.placeholder = placeholder
      state.okLabel = 'OK'
      state.danger = false
      state.link = link ?? null
      state._resolve = res
    })
  }
  function confirm(title: string, okLabel = 'Delete'): Promise<boolean> {
    return new Promise((res) => {
      state.open = true
      state.mode = 'confirm'
      state.title = title
      state.okLabel = okLabel
      state.danger = true
      state._resolve = res
    })
  }
  function submit() { state.link = null; finish(state.mode === 'prompt' ? state.value : true) }
  function cancel() { state.link = null; finish(state.mode === 'prompt' ? null : false) }

  return { state, prompt, confirm, submit, cancel }
}
