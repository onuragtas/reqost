<script setup lang="ts">
import { ref } from 'vue'
import { useTheme } from '../composables/useTheme'

export type Mode = 'collections' | 'environments' | 'history' | 'settings'

const emit = defineEmits<{ mode: [m: Mode] }>()
const { theme, toggle } = useTheme()

const active = ref<Mode>('collections')
function pick(m: Mode) {
  active.value = m
  emit('mode', m)
}

const MODES: { id: Mode; label: string }[] = [
  { id: 'collections', label: 'Collections' },
  { id: 'environments', label: 'Environments' },
  { id: 'history', label: 'History' },
  { id: 'settings', label: 'Settings' },
]
</script>

<template>
  <nav class="rail">
    <div class="group">
      <button
        v-for="m in MODES"
        :key="m.id"
        class="rail-btn"
        :class="{ active: active === m.id }"
        :title="m.label"
        @click="pick(m.id)"
      >
        <!-- Collections: stacked layers -->
        <svg v-if="m.id === 'collections'" viewBox="0 0 24 24"><path d="M3 7l9-4 9 4-9 4-9-4z"/><path d="M3 12l9 4 9-4"/><path d="M3 17l9 4 9-4"/></svg>
        <!-- Environments: globe -->
        <svg v-else-if="m.id === 'environments'" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3c2.5 2.5 2.5 15.5 0 18M12 3c-2.5 2.5-2.5 15.5 0 18"/></svg>
        <!-- History: clock -->
        <svg v-else-if="m.id === 'history'" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/></svg>
        <!-- Settings: gear -->
        <svg v-else viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M19.1 4.9L17 7M7 17l-2.1 2.1"/></svg>
      </button>
    </div>

    <button class="rail-btn theme" :title="theme === 'dark' ? 'Light mode' : 'Dark mode'" @click="toggle">
      <svg v-if="theme === 'dark'" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M19.1 4.9l-1.4 1.4M6.3 17.7l-1.4 1.4"/></svg>
      <svg v-else viewBox="0 0 24 24"><path d="M21 12.8A8 8 0 1 1 11.2 3 6.5 6.5 0 0 0 21 12.8z"/></svg>
    </button>
  </nav>
</template>

<style scoped>
.rail {
  width: 48px;
  flex-shrink: 0;
  background: var(--rail-bg);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
}
.group { display: flex; flex-direction: column; gap: 4px; }

.rail-btn {
  width: 36px;
  height: 36px;
  border-radius: 7px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-faint);
}
.rail-btn:hover { background: var(--bg-hover); color: var(--text-dim); }
.rail-btn.active { color: var(--accent); background: color-mix(in srgb, var(--accent) 14%, transparent); }
.rail-btn.theme { color: var(--text-dim); }

svg {
  width: 19px;
  height: 19px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
