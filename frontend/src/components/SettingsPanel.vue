<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Browser } from '@wailsio/runtime'
import { useTheme } from '../composables/useTheme'
import { useSettings } from '../composables/useSettings'
import { useUpdate } from '../composables/useUpdate'
import { RepoSlug } from '../../bindings/reqost/updateservice'
import { List as ListPlugins, SetEnabled as SetPluginEnabled, Dir as PluginDir } from '../../bindings/reqost/pluginservice'

const { theme, toggle } = useTheme()
const { settings, reset } = useSettings()
const { version, updateInfo, applying, applied, checkError, install } = useUpdate()

const repoSlug = ref<string>('')
const showNotes = ref(false)

const plugins = ref<any[]>([])
const pluginDir = ref<string>('')

async function refreshPlugins() {
  try { plugins.value = (await ListPlugins()) ?? [] } catch { plugins.value = [] }
  try { pluginDir.value = await PluginDir() ?? '' } catch { /* ignore */ }
}
async function togglePlugin(path: string, enabled: boolean) {
  await SetPluginEnabled(path, enabled)
  await refreshPlugins()
}

onMounted(refreshPlugins)
RepoSlug().then(s => { repoSlug.value = s }).catch(() => {})

function openReleases() {
  if (repoSlug.value) Browser.OpenURL(`https://github.com/${repoSlug.value}/releases`)
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
      <div class="row col">
        <label>Proxy URL</label>
        <input
          v-model="settings.proxyURL" class="proxy-input"
          placeholder="http://user:pass@proxy.host:8080  (leave empty to use $HTTPS_PROXY)"
        />
      </div>
    </section>

    <section class="block">
      <h4>Client certificates (mTLS)</h4>
      <p class="hint">First pattern that matches a request's host wins. Wildcards: <code>*.corp.local</code> or bare suffix <code>.internal</code>.</p>
      <div v-for="(c, i) in settings.clientCerts" :key="i" class="cert-row">
        <input v-model="c.hostPattern" placeholder="Host (e.g. *.corp.local)" class="cert-in" />
        <input v-model="c.certPath" placeholder="/path/to/client.crt" class="cert-in" />
        <input v-model="c.keyPath" placeholder="/path/to/client.key" class="cert-in" />
        <button class="cert-del" @click="settings.clientCerts.splice(i, 1)">✕</button>
      </div>
      <button class="ghost" @click="settings.clientCerts.push({ hostPattern: '', certPath: '', keyPath: '' })">+ Add certificate</button>
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
      <h4>Plugins</h4>
      <p class="hint">
        Drop <code>.js</code> files into <code class="dir">{{ pluginDir || '~/Library/Caches/reqost/plugins/' }}</code>.
        Export <code>onPreSend(req)</code>, <code>onPostReceive(req, resp)</code>, or
        <code>onTransformBody(req)</code>. They run in a goja sandbox with no I/O.
      </p>
      <div v-if="!plugins.length" class="hint" style="margin-top: 4px;">No plugins yet.</div>
      <div v-else class="plugin-list">
        <div v-for="p in plugins" :key="p.path" class="plugin-row">
          <input type="checkbox" :checked="p.enabled" @change="togglePlugin(p.path, ($event.target as HTMLInputElement).checked)" />
          <span class="plugin-name">{{ p.name }}</span>
        </div>
      </div>
      <button class="ghost" @click="refreshPlugins">Refresh</button>
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
.row.col { flex-direction: column; align-items: stretch; gap: 4px; }
.row.col label { font-size: 11px; color: var(--text-faint); }
.proxy-input {
  background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px;
  color: var(--text); font: 12px monospace; padding: 5px 8px;
}
.proxy-input:focus { outline: none; border-color: var(--accent); }
.cert-row { display: grid; grid-template-columns: 1fr 1fr 1fr 22px; gap: 4px; align-items: center; margin-top: 6px; }
.cert-in { background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; color: var(--text); font: 11px monospace; padding: 4px 6px; }
.cert-in:focus { outline: none; border-color: var(--accent); }
.cert-del { font-size: 11px; color: var(--danger); padding: 2px 4px; }
.cert-del:hover { background: var(--bg-hover); }
.hint code { background: var(--bg-input); padding: 0 4px; border-radius: 3px; font-size: 10.5px; }
.dir { word-break: break-all; font: 10.5px monospace; }
.plugin-list { display: flex; flex-direction: column; gap: 4px; margin: 6px 0; }
.plugin-row { display: flex; align-items: center; gap: 8px; padding: 4px 6px; background: var(--bg-input); border-radius: 4px; }
.plugin-name { font: 12px monospace; color: var(--text); flex: 1; word-break: break-all; }
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
