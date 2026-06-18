<script setup lang="ts">
import { ref, computed } from 'vue'

// JsonNode renders one key/value entry of the tree. Recursive component, so
// nested objects/arrays expand inline. Children render only after the user
// opens the node — keeps multi-MB responses snappy.
const props = defineProps<{
  keyName: string
  data: any
  depth: number
  query: string
  matchFn: (text: string) => boolean
  defaultOpen?: boolean
  arrayIndex?: boolean
}>()

const open = ref<boolean>(!!props.defaultOpen || props.depth < 1)

const kind = computed<'object' | 'array' | 'string' | 'number' | 'boolean' | 'null' | 'undefined'>(() => {
  const v = props.data
  if (v === null) return 'null'
  if (v === undefined) return 'undefined'
  if (Array.isArray(v)) return 'array'
  return typeof v as any
})

const isLeaf = computed(() => kind.value !== 'object' && kind.value !== 'array')

const entries = computed<[string, any][]>(() => {
  if (kind.value === 'array') return (props.data as any[]).map((v, i) => [String(i), v])
  if (kind.value === 'object') return Object.entries(props.data)
  return []
})

const summary = computed(() => {
  if (kind.value === 'array') return `Array(${(props.data as any[]).length})`
  if (kind.value === 'object') return `Object {${entries.value.length}}`
  return ''
})

const valueDisplay = computed(() => {
  if (kind.value === 'string') return JSON.stringify(props.data)
  if (kind.value === 'null')   return 'null'
  if (kind.value === 'undefined') return 'undefined'
  return String(props.data)
})

const keyMatch = computed(() => !!props.query && props.matchFn(props.keyName))
const valueMatch = computed(() => !!props.query && isLeaf.value && props.matchFn(valueDisplay.value))
function toggle() {
  if (isLeaf.value) return
  open.value = !open.value
}
</script>

<template>
  <div class="node">
    <div
      class="row"
      :class="{ open, leaf: isLeaf }"
      @click="toggle"
    >
      <span class="caret" :class="{ hidden: isLeaf }">{{ open ? '▾' : '▸' }}</span>
      <span v-if="keyName" class="key" :class="{ idx: arrayIndex, hit: keyMatch }">{{ keyName }}</span>
      <span v-if="keyName" class="colon">:</span>

      <template v-if="isLeaf">
        <span class="val" :class="['k-' + kind, { hit: valueMatch }]">{{ valueDisplay }}</span>
      </template>
      <template v-else>
        <span v-if="!open" class="sum">{{ summary }}</span>
        <span v-else class="brk">{{ kind === 'array' ? '[' : '{' }}</span>
      </template>
    </div>

    <div v-if="!isLeaf && open" class="children" :style="{ paddingLeft: `${(depth + 1) * 14}px` }">
      <JsonNode
        v-for="([k, v]) in entries"
        :key="k"
        :keyName="k"
        :data="v"
        :depth="depth + 1"
        :query="query"
        :matchFn="matchFn"
        :arrayIndex="kind === 'array'"
      />
      <div class="brk close" :style="{ paddingLeft: '0' }">{{ kind === 'array' ? ']' : '}' }}</div>
    </div>
  </div>
</template>

<style scoped>
.node { display: contents; }
.row {
  display: flex; align-items: center; gap: 4px;
  padding: 1px 8px; cursor: pointer; border-radius: 3px;
}
.row:hover:not(.leaf) { background: var(--bg-hover); }
.row.leaf { cursor: default; }
.caret { width: 12px; color: var(--text-faint); text-align: center; font-size: 9px; }
.caret.hidden { visibility: hidden; }
.key { color: #61affe; }
.key.idx { color: var(--text-faint); }
.key.hit, .val.hit { background: color-mix(in srgb, var(--accent) 30%, transparent); color: var(--text); border-radius: 2px; padding: 0 2px; }
.colon { color: var(--text-faint); }
.val { word-break: break-all; }
.k-string  { color: #49cc90; }
.k-number  { color: #fca130; }
.k-boolean { color: #50e3c2; }
.k-null,
.k-undefined { color: var(--text-faint); font-style: italic; }
.sum { color: var(--text-faint); font-style: italic; }
.brk { color: var(--text-dim); }
.brk.close { color: var(--text-dim); padding: 1px 8px; }
.children { display: flex; flex-direction: column; }
</style>
