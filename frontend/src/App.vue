<script setup lang="ts">
import { ref, onMounted } from 'vue'
import IconRail, { type Mode } from './components/IconRail.vue'
import Sidebar from './components/Sidebar.vue'
import TabBar from './components/TabBar.vue'
import RequestWorkbench from './components/RequestWorkbench.vue'
import EnvironmentsModal from './components/EnvironmentsModal.vue'
import RunnerModal from './components/RunnerModal.vue'
import DialogModal from './components/DialogModal.vue'
import HistoryPanel from './components/HistoryPanel.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import { useEnv } from './composables/useEnv'

const { loadEnvironments, openModal } = useEnv()

// Left-rail mode. 'environments' is a modal, so it doesn't become a panel.
const mode = ref<Exclude<Mode, 'environments'>>('collections')

function onMode(m: Mode) {
  if (m === 'environments') { openModal(); return }
  mode.value = m
}

onMounted(loadEnvironments)
</script>

<template>
  <div class="app">
    <IconRail @mode="onMode" />

    <Sidebar v-show="mode === 'collections'" class="sidebar" />
    <HistoryPanel v-if="mode === 'history'" class="sidebar" />
    <SettingsPanel v-if="mode === 'settings'" class="sidebar" />

    <main class="main">
      <TabBar />
      <RequestWorkbench />
    </main>

    <EnvironmentsModal />
    <RunnerModal />
    <DialogModal />
  </div>
</template>

<style scoped>
.app { display: flex; height: 100%; overflow: hidden; }
.sidebar { width: 280px; flex-shrink: 0; }
.main { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-width: 0; }
</style>
