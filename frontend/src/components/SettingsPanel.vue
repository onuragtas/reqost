<script setup lang="ts">
import { ref } from 'vue'
import { useTheme } from '../composables/useTheme'
import { useSettings } from '../composables/useSettings'
import { useUpdate } from '../composables/useUpdate'
import { RepoSlug } from '../../bindings/reqost/updateservice'

const { theme, toggle } = useTheme()
const { settings, reset } = useSettings()
const { version, updateInfo, applying, applied, checkError, install } = useUpdate()

const repoSlug = ref<string>('')
const showNotes = ref(false)

RepoSlug().then(s => { repoSlug.value = s }).catch(() => {})

function openReleases() {
  if (repoSlug.value) window.open(`https://github.com/${repoSlug.value}/releases`, '_blank')
}
</script>

<template>
  <aside class="settings-panel selectable">
    <header class="head">Settings</header>

    <section class="block">
      <h4>Appearance</h4>
      <div class="row">
        <label>Theme</label>
        <button class="pill" @click="toggle">{{ theme === 'dark' ? 'Dark' : 'Light' }}</button>
      </div>
    </section>

    <section class="block">
      <h4>Requests — defaults</h4>
      <div class="row">
        <label>Timeout (ms)</label>
        <input type="number" min="0" step="500" v-model.number="settings.defaultTimeoutMs" />
      </div>
      <p class="hint">0 = no timeout. Per-request overrides take precedence.</p>

      <div class="row">
        <label>Follow redirects</label>
        <input type="checkbox" v-model="settings.defaultFollowRedirects" />
      </div>
      <div class="row" v-if="settings.defaultFollowRedirects">
        <label>Max redirects</label>
        <input type="number" min="0" max="50" v-model.number="settings.defaultMaxRedirects" />
      </div>
      <div class="row">
        <label>Verify SSL</label>
        <input type="checkbox" v-model="settings.defaultVerifySSL" />
      </div>
    </section>

    <section class="block">
      <h4>Updates</h4>
      <div class="row">
        <label>Current version</label>
        <code class="ver">{{ version }}</code>
      </div>
      <div class="row" v-if="repoSlug">
        <label>Source</label>
        <button class="pill" @click="openReleases">{{ repoSlug }} ↗</button>
      </div>
      <template v-if="updateInfo && !applied">
        <p class="hint upd-avail">↑ {{ updateInfo.latest }} available — see the title bar to install.</p>
        <div v-if="updateInfo?.notes" class="notes-wrap">
          <button class="link" @click="showNotes = !showNotes">
            {{ showNotes ? 'Hide' : 'Show' }} release notes
          </button>
          <pre v-if="showNotes" class="notes selectable">{{ updateInfo.notes }}</pre>
        </div>
      </template>
      <p v-else-if="applied" class="hint">Installed — quit and reopen reqost to use it.</p>
      <p v-else class="hint">Updates are checked automatically on startup.</p>
      <p v-if="checkError" class="err">⚠ {{ checkError }}</p>
    </section>

    <section class="block">
      <h4>About</h4>
      <p class="hint">reqost — fast Postman-style API client for very large collections.</p>
      <button class="ghost" @click="reset">Reset to defaults</button>
    </section>
  </aside>
</template>

<style scoped>
.settings-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px 16px;
  background: var(--bg-panel);
  border-right: 1px solid var(--border);
  overflow-y: auto;
}
.head {
  font-size: 13px;
  font-weight: 700;
  color: var(--text);
  letter-spacing: 0.3px;
  padding-bottom: 4px;
}
.block {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: 6px;
}
.block h4 {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.6px;
  color: var(--text-faint);
  margin-bottom: 4px;
}
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  color: var(--text-dim);
}
.row input[type="number"] {
  width: 96px;
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text);
  font: 12px monospace;
  padding: 4px 6px;
  text-align: right;
}
.row input[type="number"]:focus { outline: none; border-color: var(--accent); }
.row input[type="checkbox"] { accent-color: var(--accent); }
.hint { font-size: 11px; color: var(--text-faint); line-height: 1.4; }
.pill {
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  border-radius: 12px;
  color: var(--text-dim);
  font-size: 11px;
  padding: 3px 10px;
}
.pill:hover { background: var(--bg-hover); color: var(--text); }
.ghost {
  align-self: flex-start;
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  color: var(--text-dim);
  font-size: 11px;
  padding: 5px 10px;
}
.ghost:hover { color: var(--danger); border-color: var(--danger); }
.ver { font: 11px monospace; color: var(--text); background: var(--bg-input); padding: 2px 6px; border-radius: 4px; }
.pill.primary { background: var(--accent); color: var(--accent-text); border-color: transparent; font-weight: 600; }
.pill.primary:hover:not(:disabled) { filter: brightness(1.1); }
.pill.install { background: var(--ok); color: #06140d; }
.pill:disabled { opacity: 0.55; cursor: default; }
.err { font-size: 11px; color: var(--danger); line-height: 1.4; word-break: break-word; }
.upd-avail { color: var(--ok); }
.notes-wrap { margin-top: 4px; display: flex; flex-direction: column; gap: 4px; }
.link { align-self: flex-start; color: var(--accent); font-size: 11px; background: transparent; padding: 0; }
.link:hover { text-decoration: underline; }
.notes {
  max-height: 200px; overflow: auto;
  background: var(--bg-input); border: 1px solid var(--border);
  border-radius: 4px; padding: 8px; font: 11px/1.5 monospace; color: var(--text);
  white-space: pre-wrap; word-break: break-word;
}
</style>
