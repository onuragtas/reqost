<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useEnv } from '../composables/useEnv'
import { useSettings } from '../composables/useSettings'
import { useTabs } from '../composables/useTabs'
import { ActiveWorkspaceID, ListWorkspaces } from '../../bindings/reqost/collectionservice'
import { List as ListPlugins } from '../../bindings/reqost/pluginservice'

// Footer bar showing whatever the user would otherwise have to dig into
// menus for: active workspace, active environment, proxy, plugin count, tab
// count. Click each tile to jump to its config surface.

const { active: activeEnv, openModal: openEnvModal } = useEnv()
const { settings } = useSettings()
const { tabs } = useTabs()

const wsName = ref('Workspace')
const pluginCount = ref(0)

async function refresh() {
  try {
    const ws: any[] = (await ListWorkspaces()) ?? []
    const id = await ActiveWorkspaceID()
    wsName.value = ws.find(w => w.id === id)?.name ?? 'Workspace'
  } catch { /* ignore */ }
  try {
    const ps: any[] = (await ListPlugins()) ?? []
    pluginCount.value = ps.filter(p => p.enabled).length
  } catch { /* ignore */ }
}

onMounted(refresh)
defineExpose({ refresh })
</script>

<template>
  <footer class="statusbar">
    <button class="cell" :title="`Workspace: ${wsName}`">
      <svg viewBox="0 0 16 16" aria-hidden="true"><rect x="1.5" y="3.5" width="13" height="9" rx="1" fill="none" stroke="currentColor" stroke-width="1.2"/></svg>
      {{ wsName }}
    </button>
    <button class="cell" :title="activeEnv?.name ? `Environment: ${activeEnv.name}` : 'No environment selected'" @click="openEnvModal">
      <svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="6" fill="none" stroke="currentColor" stroke-width="1.2"/><path d="M2 8h12M8 2c2 2 2 10 0 12M8 2c-2 2-2 10 0 12" fill="none" stroke="currentColor" stroke-width="1.2"/></svg>
      {{ activeEnv?.name || 'No env' }}
    </button>
    <span class="cell" v-if="settings.proxyURL" :title="`Proxy: ${settings.proxyURL}`">
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 4l5 4-5 4M8 4l5 4-5 4" fill="none" stroke="currentColor" stroke-width="1.2"/></svg>
      Proxy
    </span>
    <span class="cell" v-if="pluginCount" :title="`${pluginCount} plugin(s) active`">
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M2 6h4V2M10 14V10h4M2 10v4h4M14 6V2h-4" fill="none" stroke="currentColor" stroke-width="1.2"/></svg>
      {{ pluginCount }} plugin{{ pluginCount === 1 ? '' : 's' }}
    </span>
    <span class="grow"></span>
    <span class="cell" :title="`${tabs.length} open tab(s)`">{{ tabs.length }} tab{{ tabs.length === 1 ? '' : 's' }}</span>
  </footer>
</template>

<style scoped>
.statusbar {
  display: flex; align-items: center; gap: 4px;
  height: 24px; flex-shrink: 0;
  background: var(--bg-panel);
  border-top: 1px solid var(--border);
  padding: 0 8px;
  font-size: 11px; color: var(--text-dim);
}
.cell {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 2px 8px; border-radius: 3px;
  background: transparent;
  color: inherit;
}
button.cell { cursor: pointer; }
button.cell:hover { background: var(--bg-hover); color: var(--text); }
.cell svg { width: 11px; height: 11px; flex-shrink: 0; }
.grow { flex: 1; }
</style>
