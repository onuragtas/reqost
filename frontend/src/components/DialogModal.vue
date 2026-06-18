<script setup lang="ts">
import { watch, nextTick, ref } from 'vue'
import { useDialog } from '../composables/useDialog'

const { state, submit, cancel } = useDialog()
const input = ref<HTMLInputElement | null>(null)

watch(() => state.open, (open) => {
  if (open && state.mode === 'prompt') nextTick(() => input.value?.focus())
})
</script>

<template>
  <div v-if="state.open" class="overlay" @click.self="cancel" @keydown.esc="cancel">
    <div class="dialog">
      <div class="title">{{ state.title }}</div>
      <input
        v-if="state.mode === 'prompt'"
        ref="input"
        v-model="state.value"
        class="input"
        :placeholder="state.placeholder"
        @keyup.enter="submit"
        @keyup.esc="cancel"
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
.title { font-size: 14px; color: var(--text); margin-bottom: 12px; }
.input { width: 100%; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 6px; color: var(--text); font: 13px monospace; padding: 9px 11px; margin-bottom: 14px; }
.input:focus { outline: none; border-color: var(--accent); }
.actions { display: flex; justify-content: flex-end; gap: 8px; }
.cancel { color: var(--text-dim); font-size: 13px; padding: 7px 14px; border-radius: 6px; }
.cancel:hover { background: var(--bg-hover); }
.ok { background: var(--accent); color: var(--accent-text); font-weight: 600; font-size: 13px; padding: 7px 16px; border-radius: 6px; }
.ok.danger { background: var(--danger); color: #fff; }
</style>
