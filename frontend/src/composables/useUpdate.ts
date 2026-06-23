import { ref, readonly } from 'vue'
import { CurrentVersion, CheckForUpdate, ApplyUpdate, RestartApp } from '../../bindings/reqost/updateservice'
// ApplyUpdate no longer takes an argument — the backend caches the Info from CheckForUpdate.

const version    = ref<string>('…')
const updateInfo = ref<any>(null)   // non-null when a newer release is available
const applying   = ref(false)
const applied    = ref(false)
const restarting = ref(false)
const checking   = ref(false)       // a manual check is in flight
const upToDate    = ref(false)      // last manual check found nothing newer
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

// check is the manual "Check for updates" action — unlike autoCheck it always
// queries and surfaces the outcome (found / up-to-date / error).
async function check() {
  if (checking.value) return
  checking.value = true
  checkError.value = ''
  upToDate.value = false
  try {
    if (version.value === '…') {
      try { version.value = await CurrentVersion() } catch { version.value = 'dev' }
    }
    const info: any = await CheckForUpdate()
    if (info?.available) updateInfo.value = info
    else upToDate.value = true
  } catch (e: any) {
    checkError.value = e?.message ?? String(e)
  } finally {
    checking.value = false
  }
}

async function install() {
  if (!updateInfo.value || applied.value) return
  applying.value = true
  checkError.value = ''
  try {
    await ApplyUpdate()
    applied.value = true
    updateInfo.value = null
  } catch (e: any) {
    checkError.value = e?.message ?? String(e)
    return
  } finally {
    applying.value = false
  }
  // Binary swapped in — relaunch into the new version. The backend spawns a
  // detached relauncher and exits this process shortly after, so the window
  // will disappear and reopen on its own.
  await restart()
}

async function restart() {
  restarting.value = true
  try {
    await RestartApp()
  } catch (e: any) {
    // Relaunch failed — leave a hint so the user can quit/reopen manually.
    restarting.value = false
    checkError.value = `Auto-restart failed (${e?.message ?? e}). Quit and reopen to finish updating.`
  }
}

export function useUpdate() {
  return {
    version: readonly(version),
    updateInfo: readonly(updateInfo),
    applying: readonly(applying),
    applied: readonly(applied),
    restarting: readonly(restarting),
    checking: readonly(checking),
    upToDate: readonly(upToDate),
    checkError: readonly(checkError),
    autoCheck,
    check,
    install,
    restart,
  }
}
