import { reactive } from 'vue'

// Module-level command registry — components register their actions on mount,
// the palette modal renders them, and Cmd+K opens it. Quick switcher (Cmd+P)
// reuses the same modal in 'search' mode and hits the FTS5 index.

export interface Command {
  id: string
  label: string
  hint?: string
  group?: string
  run: () => void
}

export type PaletteMode = 'commands' | 'search'

const registry = reactive(new Map<string, Command>())

const state = reactive({
  open: false,
  mode: 'commands' as PaletteMode,
})

export function useCommands() {
  function register(cmd: Command) {
    registry.set(cmd.id, cmd)
  }
  function unregister(id: string) {
    registry.delete(id)
  }
  function commands(): Command[] {
    return Array.from(registry.values())
  }
  function open(mode: PaletteMode = 'commands') {
    state.mode = mode
    state.open = true
  }
  function close() { state.open = false }

  return { state, register, unregister, commands, open, close }
}
