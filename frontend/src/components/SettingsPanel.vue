<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Browser } from '@wailsio/runtime'
import { useTheme } from '../composables/useTheme'
import { useSettings } from '../composables/useSettings'
import { useUpdate } from '../composables/useUpdate'
import { RepoSlug } from '../../bindings/reqost/updateservice'
import { List as ListPlugins, SetEnabled as SetPluginEnabled, Dir as PluginDir, Reload as ReloadPlugins } from '../../bindings/reqost/pluginservice'
import { Events } from '@wailsio/runtime'

const { pref, fontSize, setPref, setFontSize } = useTheme()
const showShortcuts = ref(false)
const { settings, reset } = useSettings()
const { version, updateInfo, applying, applied, checkError, install } = useUpdate()

const repoSlug = ref<string>('')
const showNotes = ref(false)

// `{{` can't appear literally inside a template — Vue's tokenizer treats it
// as the start of an interpolation. Keep it as a script const and bind it.
const KBD_OPEN_VAR = '{{'

const plugins = ref<any[]>([])
const pluginDir = ref<string>('')
const pluginConsole = ref<{ ts: number; plugin: string; level: string; message: string }[]>([])
const showConsole = ref(false)

async function refreshPlugins() {
  try { plugins.value = (await ReloadPlugins()) ?? (await ListPlugins()) ?? [] }
  catch { plugins.value = [] }
  try { pluginDir.value = await PluginDir() ?? '' } catch { /* ignore */ }
}
async function togglePlugin(path: string, enabled: boolean) {
  await SetPluginEnabled(path, enabled)
  await refreshPlugins()
}
function clearPluginConsole() { pluginConsole.value = [] }

onMounted(() => {
  refreshPlugins()
  Events.On('plugin:console', (ev: any) => {
    const d = ev?.data ?? ev
    pluginConsole.value.push({
      ts: Date.now(),
      plugin:  d?.plugin  ?? '?',
      level:   d?.level   ?? 'log',
      message: d?.message ?? '',
    })
    if (pluginConsole.value.length > 500) {
      pluginConsole.value.splice(0, pluginConsole.value.length - 500)
    }
  })
})
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
        <div class="seg">
          <button :class="{ active: pref === 'light' }"  @click="setPref('light')">Light</button>
          <button :class="{ active: pref === 'dark' }"   @click="setPref('dark')">Dark</button>
          <button :class="{ active: pref === 'system' }" @click="setPref('system')">System</button>
        </div>
      </div>
      <div class="row">
        <label>Font size</label>
        <input
          type="range" min="10" max="20" step="1"
          :value="fontSize" @input="setFontSize(Number(($event.target as HTMLInputElement).value))"
        />
        <span class="font-val">{{ fontSize }}px</span>
      </div>
      <div class="row">
        <label>Keyboard shortcuts</label>
        <button class="pill" @click="showShortcuts = true">Show cheat sheet</button>
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
      <h4>TLS</h4>
      <div class="row col">
        <label>Custom CA bundle (PEM)</label>
        <input
          v-model="settings.caFilePath" class="proxy-input"
          placeholder="/etc/ssl/extra-roots.pem  (additional roots; system roots stay trusted)"
        />
      </div>
      <p class="hint" style="margin-bottom: 10px">Appended to the system trust store — so corporate Zscaler / mitmproxy roots Just Work without disabling SSL verify.</p>

      <h4 style="margin-top: 12px">Client certificates (mTLS)</h4>
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
      <div class="row" style="margin-top: 6px;">
        <button class="ghost" @click="refreshPlugins">Refresh</button>
        <button class="pill" @click="showConsole = !showConsole">
          {{ showConsole ? 'Hide console' : 'Console' }}
          <span v-if="pluginConsole.length" class="cnt">{{ pluginConsole.length }}</span>
        </button>
      </div>
      <div v-if="showConsole" class="plugin-console">
        <div class="pc-bar">
          <span>Plugin console — {{ pluginConsole.length }} line(s)</span>
          <button class="pill" @click="clearPluginConsole">Clear</button>
        </div>
        <div v-if="!pluginConsole.length" class="hint">No output yet. Plugins' <code>console.log</code> will appear here.</div>
        <div v-for="(l, i) in pluginConsole" :key="i" class="pc-row" :class="`lvl-${l.level}`">
          <span class="pc-plug">{{ l.plugin }}</span>
          <span class="pc-lvl">{{ l.level }}</span>
          <span class="pc-msg">{{ l.message }}</span>
        </div>
      </div>
    </section>

    <section class="block">
      <h4>About</h4>
      <p class="hint">reqost — fast Postman-style API client for very large collections.</p>
      <button class="ghost" @click="reset">Reset to defaults</button>
    </section>

    <!-- Keyboard shortcuts cheat sheet -->
    <div v-if="showShortcuts" class="cheat-overlay" @click.self="showShortcuts = false">
      <div class="cheat">
        <header class="cheat-head">
          <span>Keyboard shortcuts</span>
          <button class="cheat-close" @click="showShortcuts = false">✕</button>
        </header>
        <div class="cheat-body">
          <div class="cheat-group">
            <h5>Global</h5>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>K</kbd><span>Command palette</span></div>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>P</kbd><span>Quick request switcher</span></div>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>/</kbd><span>This cheat sheet</span></div>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>\</kbd><span>Cycle request / response split</span></div>
          </div>
          <div class="cheat-group">
            <h5>Request</h5>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>↵</kbd><span>Send</span></div>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>⇧</kbd><kbd>↵</kbd><span>Send &amp; Save</span></div>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>S</kbd><span>Save</span></div>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>W</kbd><span>Close tab</span></div>
          </div>
          <div class="cheat-group">
            <h5>Tabs</h5>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>1</kbd>…<kbd>9</kbd><span>Switch to nth tab</span></div>
            <div class="kbd-row"><kbd>Middle-click</kbd><span>Close tab</span></div>
            <div class="kbd-row"><kbd>Right-click</kbd><span>Close Others / Right / All</span></div>
          </div>
          <div class="cheat-group">
            <h5>Editor</h5>
            <div class="kbd-row"><kbd>⌘</kbd><kbd>F</kbd><span>Find in body / response</span></div>
            <div class="kbd-row"><kbd>{{ KBD_OPEN_VAR }}</kbd><span>Variable autocomplete</span></div>
          </div>
        </div>
      </div>
    </div>
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
.cnt { background: var(--accent); color: var(--accent-text); border-radius: 8px; padding: 0 6px; font-size: 10px; margin-left: 4px; }
.plugin-console {
  margin-top: 8px; max-height: 220px; overflow: auto;
  background: var(--bg-input); border: 1px solid var(--border);
  border-radius: 5px; padding: 6px;
}
.pc-bar { display: flex; justify-content: space-between; align-items: center; font-size: 10px; color: var(--text-faint); padding: 0 4px 6px; border-bottom: 1px solid var(--border); margin-bottom: 4px; }
.pc-row { display: grid; grid-template-columns: 100px 40px 1fr; gap: 6px; padding: 2px 4px; font: 11px monospace; color: var(--text); border-radius: 3px; }
.pc-row:hover { background: var(--bg-hover); }
.pc-plug { color: var(--accent); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pc-lvl { text-transform: uppercase; font-size: 9px; color: var(--text-faint); }
.pc-row.lvl-warn  .pc-lvl, .pc-row.lvl-warn  .pc-msg { color: var(--warn-text); }
.pc-row.lvl-error .pc-lvl, .pc-row.lvl-error .pc-msg { color: var(--danger); }
.pc-msg { word-break: break-word; white-space: pre-wrap; }
.seg { display: inline-flex; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; overflow: hidden; }
.seg button { font-size: 11px; padding: 3px 9px; color: var(--text-dim); border-radius: 0; }
.seg button:hover { color: var(--text); background: var(--bg-hover); }
.seg button.active { color: var(--accent); background: color-mix(in srgb, var(--accent) 14%, transparent); }
.font-val { font: 11px monospace; color: var(--text-dim); min-width: 32px; text-align: right; }
input[type=range] { accent-color: var(--accent); width: 110px; }
.cheat-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 300; }
.cheat { width: 520px; max-width: 92vw; max-height: 80vh; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 10px; box-shadow: 0 20px 60px rgba(0,0,0,0.45); overflow: hidden; display: flex; flex-direction: column; }
.cheat-head { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border); font-size: 13px; font-weight: 700; color: var(--text); }
.cheat-close { color: var(--text-faint); font-size: 14px; padding: 2px 6px; }
.cheat-close:hover { color: var(--text); }
.cheat-body { padding: 14px 18px; overflow: auto; display: flex; flex-direction: column; gap: 16px; }
.cheat-group h5 { font-size: 10px; text-transform: uppercase; letter-spacing: 0.6px; color: var(--text-faint); margin-bottom: 6px; }
.kbd-row { display: flex; align-items: center; gap: 6px; padding: 4px 0; font-size: 12px; color: var(--text-dim); }
.kbd-row span { margin-left: 6px; color: var(--text); }
kbd {
  font: 600 11px monospace; color: var(--text);
  background: var(--bg-input); border: 1px solid var(--border-strong);
  border-bottom-width: 2px; border-radius: 4px;
  padding: 1px 6px; min-width: 18px; text-align: center;
}
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
