<script setup lang="ts">
import { ref, computed } from 'vue'

// JsonTree renders an arbitrary JSON value as a collapsible tree. Designed for
// large response bodies — children of an object/array are rendered lazily
// (only when expanded) and a single search box highlights keys/values.

const props = defineProps<{ value: any; rootLabel?: string }>()

const search = ref('')

const hasQuery = computed(() => search.value.trim().length > 0)
const query = computed(() => search.value.trim().toLowerCase())

function matches(text: string): boolean {
  if (!hasQuery.value) return false
  return text.toLowerCase().includes(query.value)
}
</script>

<template>
  <div class="jt">
    <div class="jt-bar">
      <input
        v-model="search"
        class="jt-search"
        placeholder="Filter keys / values…"
        spellcheck="false"
      />
    </div>
    <div class="jt-body selectable">
      <JsonNode
        :keyName="rootLabel ?? ''"
        :data="value"
        :depth="0"
        :query="query"
        :matchFn="matches"
        :defaultOpen="true"
      />
    </div>
  </div>
</template>

<style scoped>
.jt { display: flex; flex-direction: column; flex: 1; min-height: 0; overflow: hidden; }
.jt-bar { padding: 6px 12px; border-bottom: 1px solid var(--border); background: var(--bg-elevated); flex-shrink: 0; }
.jt-search {
  width: 100%; background: var(--bg-input); border: 1px solid var(--border);
  border-radius: 4px; color: var(--text); font: 12px monospace; padding: 5px 8px;
}
.jt-search:focus { outline: none; border-color: var(--accent); }
.jt-body { flex: 1; overflow: auto; padding: 8px 4px; font: 12px/1.55 monospace; }
</style>
