<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useUpdate } from '../composables/useUpdate'

const { version, updateInfo, applying, applied, checkError, autoCheck, install } = useUpdate()

const showPopover = ref(false)

onMounted(autoCheck)

async function onInstall() {
  await install()
  if (applied.value) showPopover.value = false
}
</script>

<template>
  <div class="titlebar">
    <span class="app-name">ReQost</span>

    <div class="right">
      <span class="ver-badge">{{ version }}</span>

      <!-- update available pill -->
      <div v-if="updateInfo" class="upd-wrap">
        <button class="upd-pill" @click.stop="showPopover = !showPopover">
          ↑ {{ updateInfo.latest }}
        </button>

        <div v-if="showPopover" class="upd-pop" @click.stop>
          <div class="pop-title">Update available</div>
          <div class="pop-meta">{{ version }} → {{ updateInfo.latest }}</div>
          <p v-if="checkError" class="pop-err">{{ checkError }}</p>
          <div class="pop-actions">
            <button class="pop-btn primary" :disabled="applying" @click="onInstall">
              {{ applying ? 'Installing…' : 'Install & relaunch' }}
            </button>
            <button class="pop-btn" @click="showPopover = false">Later</button>
          </div>
          <p class="pop-hint">Quit and reopen to apply after install.</p>
        </div>
      </div>
    </div>

    <!-- click-outside dismiss -->
    <div v-if="showPopover" class="backdrop" @click="showPopover = false" />
  </div>
</template>

<style scoped>
.titlebar {
  height: 50px;
  flex-shrink: 0;
  background: var(--rail-bg);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  padding-left: 88px;
  padding-right: 12px;
  -webkit-app-region: drag;
  user-select: none;
  -webkit-user-select: none;
}

.app-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-faint);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  -webkit-app-region: no-drag;
}

.right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
  -webkit-app-region: no-drag;
}

.ver-badge {
  font: 10px monospace;
  color: var(--text-faint);
  opacity: 0.6;
}

/* ── update pill ── */
.upd-wrap {
  position: relative;
}

.upd-pill {
  background: color-mix(in srgb, var(--ok) 18%, transparent);
  border: 1px solid color-mix(in srgb, var(--ok) 50%, transparent);
  border-radius: 12px;
  color: var(--ok);
  font-size: 11px;
  font-weight: 600;
  padding: 3px 10px;
  cursor: pointer;
  animation: pulse 2.4s ease-in-out infinite;
}
.upd-pill:hover { filter: brightness(1.15); }

@keyframes pulse {
  0%, 100% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--ok) 35%, transparent); }
  50%       { box-shadow: 0 0 0 4px color-mix(in srgb, var(--ok) 0%, transparent); }
}

/* ── popover ── */
.upd-pop {
  position: absolute;
  top: calc(100% + 10px);
  right: 0;
  width: 260px;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0,0,0,.35);
  padding: 14px 16px;
  z-index: 200;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.pop-title { font-size: 13px; font-weight: 700; color: var(--text); }
.pop-meta  { font: 11px monospace; color: var(--text-dim); }
.pop-hint  { font-size: 10px; color: var(--text-faint); margin-top: 2px; }
.pop-err   { font-size: 11px; color: var(--danger); }

.pop-actions { display: flex; gap: 6px; margin-top: 4px; }

.pop-btn {
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  color: var(--text-dim);
  font-size: 12px;
  padding: 6px 0;
  cursor: pointer;
}
.pop-btn:hover:not(:disabled) { color: var(--text); background: var(--bg-hover); }
.pop-btn.primary {
  background: var(--ok);
  color: #06140d;
  border-color: transparent;
  font-weight: 600;
}
.pop-btn.primary:hover:not(:disabled) { filter: brightness(1.1); }
.pop-btn:disabled { opacity: 0.5; cursor: default; }

.backdrop {
  position: fixed;
  inset: 0;
  z-index: 199;
}
</style>
