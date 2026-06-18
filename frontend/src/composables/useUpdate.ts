import { ref, readonly } from 'vue'
import { CurrentVersion, CheckForUpdate, ApplyUpdate } from '../../bindings/reqost/updateservice'

const version   = ref<string>('…')
const updateInfo = ref<any>(null)   // non-null when a newer release is available
const applying   = ref(false)
const applied    = ref(false)
const checkError = ref<string>('')
let   checked    = false            // only auto-check once per session

async function autoCheck() {
  if (checked) return
  checked = true
  try { version.value = await CurrentVersion() } catch { version.value = 'dev' }
  try {
    const info: any = await CheckForUpdate()
    if (info?.available) updateInfo.value = info
  } catch { /* silent — no network, no noise */ }
}

async function install() {
  if (!updateInfo.value || applied.value) return
  applying.value = true
  checkError.value = ''
  try {
    await ApplyUpdate(updateInfo.value)
    applied.value = true
    updateInfo.value = null
  } catch (e: any) {
    checkError.value = e?.message ?? String(e)
  } finally {
    applying.value = false
  }
}

export function useUpdate() {
  return {
    version: readonly(version),
    updateInfo: readonly(updateInfo),
    applying: readonly(applying),
    applied: readonly(applied),
    checkError: readonly(checkError),
    autoCheck,
    install,
  }
}
