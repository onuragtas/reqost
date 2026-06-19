<script setup lang="ts">
import { ref, onMounted } from 'vue'
import TitleBar from './components/TitleBar.vue'
import IconRail, { type Mode } from './components/IconRail.vue'
import Sidebar from './components/Sidebar.vue'
import TabBar from './components/TabBar.vue'
import RequestWorkbench from './components/RequestWorkbench.vue'
import EnvironmentsModal from './components/EnvironmentsModal.vue'
import RunnerModal from './components/RunnerModal.vue'
import DialogModal from './components/DialogModal.vue'
import HistoryPanel from './components/HistoryPanel.vue'
import CommandPalette from './components/CommandPalette.vue'
import { useCommands } from './composables/useCommands'
import { useEnv } from './composables/useEnv'
import { useTheme } from './composables/useTheme'
import SettingsPanel from './components/SettingsPanel.vue'
import DesignPanel from './components/DesignPanel.vue'
import StatusBar from './components/StatusBar.vue'

const { loadEnvironments, openModal } = useEnv()
const { register, open: openPalette } = useCommands()
const { toggle: toggleTheme } = useTheme()

// Left-rail mode. 'environments' is a modal, so it doesn't become a panel.
const mode = ref<Exclude<Mode, 'environments'>>('collections')

function onMode(m: Mode) {
  if (m === 'environments') { openModal(); return }
  mode.value = m
}

// Global commands registered on mount — components register their own as well.
function registerGlobalCommands() {
  register({ id: 'go.collections', label: 'Go to: Collections', group: 'Navigation', run: () => (mode.value = 'collections') })
  register({ id: 'go.history',     label: 'Go to: History',     group: 'Navigation', run: () => (mode.value = 'history') })
  register({ id: 'go.settings',    label: 'Go to: Settings',    group: 'Navigation', run: () => (mode.value = 'settings') })
  register({ id: 'env.open',       label: 'Manage environments…', group: 'Navigation', run: openModal })
  register({ id: 'theme.toggle',   label: 'Toggle theme (light/dark)', group: 'Appearance', run: toggleTheme })
}

// Cmd+K → command palette; Cmd+P → quick request switcher; Cmd+/ → settings.
function onKey(e: KeyboardEvent) {
  if (!(e.metaKey || e.ctrlKey)) return
  const k = e.key.toLowerCase()
  if (k === 'k') { e.preventDefault(); openPalette('commands') }
  else if (k === 'p') { e.preventDefault(); openPalette('search') }
  else if (k === '/') { e.preventDefault(); mode.value = 'settings' }
}

onMounted(() => {
  loadEnvironments()
  registerGlobalCommands()
  window.addEventListener('keydown', onKey)
})
</script>

<template>
  <div class="app">
    <TitleBar />
    <div class="content">
      <IconRail @mode="onMode" />

      <Sidebar v-show="mode === 'collections'" class="sidebar" />
      <HistoryPanel v-if="mode === 'history'" class="sidebar" />
      <DesignPanel v-if="mode === 'design'" class="sidebar design-wide" />
      <SettingsPanel v-if="mode === 'settings'" class="sidebar" />

      <main class="main">
        <TabBar />
        <RequestWorkbench />
      </main>
    </div>

    <StatusBar />

    <EnvironmentsModal />
    <RunnerModal />
    <DialogModal />
    <CommandPalette />
  </div>
</template>

<style scoped>
.app { display: flex; flex-direction: column; height: 100%; overflow: hidden; }
.content { display: flex; flex: 1; overflow: hidden; min-height: 0; }
.sidebar { width: 280px; flex-shrink: 0; }
.sidebar.design-wide { width: 640px; }
.main { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-width: 0; }
</style>
