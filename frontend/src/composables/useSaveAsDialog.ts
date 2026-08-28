import { reactive } from 'vue'

// Postman-style "Save As" folder picker, shown the first time a tab that
// isn't backed by a collection node (opened via "+"/Cmd+T, or an adhoc URL
// from History/a dropped link) gets saved. Same open/await-a-Promise shape
// as useDialog.ts.
const state = reactive({
  open: false,
  title: 'Save Request',
  _resolve: null as null | ((v: string | null) => void),
})

function finish(v: string | null) {
  const r = state._resolve
  state.open = false
  state._resolve = null
  if (r) r(v)
}

export function useSaveAsDialog() {
  // Resolves with the chosen parent folder id ('' = collection root), or
  // null if the user cancelled.
  function pickFolder(title = 'Save Request'): Promise<string | null> {
    return new Promise((res) => {
      state.open = true
      state.title = title
      state._resolve = res
    })
  }
  function choose(parentId: string) { finish(parentId) }
  function cancel() { finish(null) }

  return { state, pickFolder, choose, cancel }
}
