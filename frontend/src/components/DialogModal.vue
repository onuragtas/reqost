<script setup lang="ts">
import { watch, nextTick, ref } from 'vue'
import { Browser } from '@wailsio/runtime'
import { useDialog } from '../composables/useDialog'

const { state, submit, cancel } = useDialog()
const input = ref<HTMLInputElement | null>(null)
const area = ref<HTMLTextAreaElement | null>(null)

watch(() => state.open, (open) => {
  if (!open) return
  nextTick(() => {
    if (state.mode === 'prompt') input.value?.focus()
    if (state.mode === 'prompt-multiline') area.value?.focus()
  })
})

function openLink(url: string) { Browser.OpenURL(url) }
</script>

<template>
  <div v-if="state.open" class="overlay" @click.self="cancel" @keydown.esc="cancel">
    <div class="dialog">
      <div class="title">{{ state.title }}</div>
      <a
        v-if="state.link"
        class="dialog-link"
        :href="state.link.url"
        target="_blank"
        @click.prevent="openLink(state.link!.url)"
      >{{ state.link.label }} ↗</a>
      <input
        v-if="state.mode === 'prompt'"
        ref="input"
        v-model="state.value"
        class="input"
        :placeholder="state.placeholder"
        @keyup.enter="submit"
        @keyup.esc="cancel"
      />
      <textarea
        v-if="state.mode === 'prompt-multiline'"
        ref="area"
        v-model="state.value"
        class="textarea"
        :placeholder="state.placeholder"
        spellcheck="false"
        @keydown.esc="cancel"
        @keydown.meta.enter="submit"
        @keydown.ctrl.enter="submit"
      />
      <div class="actions">
        <button class="cancel" @click="cancel">Cancel</button>
        <button class="ok" :class="{ danger: state.danger }" @click="submit">{{ state.okLabel }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 300; }
.dialog { width: 420px; max-width: 90vw; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 10px; padding: 18px; box-shadow: 0 20px 60px rgba(0,0,0,0.5); }
.title { font-size: 14px; color: var(--text); margin-bottom: 8px; }
.dialog-link { display: inline-block; font-size: 11px; color: var(--accent); margin-bottom: 12px; }
.dialog-link:hover { text-decoration: underline; }
.input { width: 100%; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 6px; color: var(--text); font: 13px monospace; padding: 9px 11px; margin-bottom: 14px; }
.input:focus { outline: none; border-color: var(--accent); }
.textarea {
  width: 100%; min-height: 180px; resize: vertical;
  background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 6px;
  color: var(--text); font: 12px/1.5 monospace; padding: 10px 12px; margin-bottom: 14px;
  white-space: pre; overflow: auto;
}
.textarea:focus { outline: none; border-color: var(--accent); }
.actions { display: flex; justify-content: flex-end; gap: 8px; }
.cancel { color: var(--text-dim); font-size: 13px; padding: 7px 14px; border-radius: 6px; }
.cancel:hover { background: var(--bg-hover); }
.ok { background: var(--accent); color: var(--accent-text); font-weight: 600; font-size: 13px; padding: 7px 16px; border-radius: 6px; }
.ok.danger { background: var(--danger); color: #fff; }
</style>
