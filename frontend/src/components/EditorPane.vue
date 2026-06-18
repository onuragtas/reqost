<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { EditorState, Compartment, RangeSetBuilder } from '@codemirror/state'
import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection, Decoration, type DecorationSet, ViewPlugin, type ViewUpdate } from '@codemirror/view'
import {
  defaultHighlightStyle, syntaxHighlighting, bracketMatching, indentOnInput, foldGutter, foldKeymap,
} from '@codemirror/language'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { searchKeymap, highlightSelectionMatches } from '@codemirror/search'
import { autocompletion, completionKeymap, closeBrackets, closeBracketsKeymap, type CompletionContext } from '@codemirror/autocomplete'
import { json } from '@codemirror/lang-json'
import { javascript } from '@codemirror/lang-javascript'
import { xml } from '@codemirror/lang-xml'

// EditorPane wraps a CodeMirror 6 instance with reqost's textarea-style API
// (v-model + language prop). It is the same surface area as the old textareas
// it replaces, so callers don't need to know about EditorState/Compartments.
const props = withDefaults(defineProps<{
  modelValue: string
  language?: 'json' | 'javascript' | 'xml' | 'plain'
  placeholder?: string
  readonly?: boolean
  minHeight?: string
  // vars enables `{{name}}` highlighting + `{{` autocomplete from known keys.
  // Resolved value (when present) is shown in completion details; unknown
  // placeholders get a red underline.
  vars?: Record<string, string>
}>(), {
  language: 'plain',
  placeholder: '',
  readonly: false,
  minHeight: '120px',
  vars: () => ({}),
})

const emit = defineEmits<{ 'update:modelValue': [v: string] }>()

const host = ref<HTMLDivElement | null>(null)
const langCompartment = new Compartment()
const readonlyCompartment = new Compartment()
let view: EditorView | null = null
let suppressUpdate = false

// `currentVars` is read by the highlighter + completion source. We keep it as
// a closure-bound ref-like so reconfiguring isn't needed when vars change.
let currentVars: Record<string, string> = { ...props.vars }

// Decorate every {{name}} occurrence; unknown names get a `cm-var-missing`
// class for the red underline.
const varDeco = Decoration.mark({ class: 'cm-var' })
const varMissingDeco = Decoration.mark({ class: 'cm-var cm-var-missing' })
const VAR_RE = /\{\{\s*(\$?[\w.\-]+)\s*\}\}/g

function buildVarDecos(view: EditorView): DecorationSet {
  const b = new RangeSetBuilder<Decoration>()
  const text = view.state.doc.toString()
  let m: RegExpExecArray | null
  VAR_RE.lastIndex = 0
  while ((m = VAR_RE.exec(text))) {
    const name = m[1]
    const known = name.startsWith('$') || (name in currentVars)
    b.add(m.index, m.index + m[0].length, known ? varDeco : varMissingDeco)
  }
  return b.finish()
}

const varHighlightPlugin = ViewPlugin.fromClass(class {
  decorations: DecorationSet
  constructor(v: EditorView) { this.decorations = buildVarDecos(v) }
  update(u: ViewUpdate) {
    if (u.docChanged || u.viewportChanged) this.decorations = buildVarDecos(u.view)
  }
}, { decorations: v => v.decorations })

function varCompletionSource(ctx: CompletionContext) {
  // Trigger on `{{` followed by zero or more word chars.
  const before = ctx.matchBefore(/\{\{\s*[\w$.\-]*$/)
  if (!before) return null
  const partial = before.text.replace(/^\{\{\s*/, '')
  const options = Object.keys(currentVars).map(k => ({
    label: k,
    apply: `${k}}}`,
    detail: currentVars[k] ? truncate(currentVars[k], 40) : '',
    type: 'variable',
  }))
  // Add the most common dynamic helpers — Postman parity.
  for (const h of ['$timestamp', '$isoTimestamp', '$guid', '$randomUUID', '$randomInt', '$randomEmail']) {
    options.push({ label: h, apply: `${h}}}`, detail: 'dynamic', type: 'function' })
  }
  return {
    from: before.from + before.text.length - partial.length,
    options,
    validFor: /^[\w$.\-]*$/,
  }
}

function truncate(s: string, n: number) { return s.length > n ? s.slice(0, n - 1) + '…' : s }

function languageExt(lang: typeof props.language) {
  switch (lang) {
    case 'json':       return json()
    case 'javascript': return javascript()
    case 'xml':        return xml()
    default:           return []
  }
}

onMounted(() => {
  if (!host.value) return
  const state = EditorState.create({
    doc: props.modelValue,
    extensions: [
      lineNumbers(),
      foldGutter(),
      highlightActiveLine(),
      drawSelection(),
      history(),
      bracketMatching(),
      closeBrackets(),
      indentOnInput(),
      autocompletion({ override: [varCompletionSource] }),
      varHighlightPlugin,
      highlightSelectionMatches(),
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      keymap.of([
        ...closeBracketsKeymap,
        ...defaultKeymap,
        ...searchKeymap,
        ...historyKeymap,
        ...foldKeymap,
        ...completionKeymap,
        indentWithTab,
      ]),
      langCompartment.of(languageExt(props.language)),
      readonlyCompartment.of(EditorState.readOnly.of(props.readonly)),
      EditorView.theme({
        '&':            { fontFamily: 'monospace', fontSize: '12px' },
        '.cm-scroller': { lineHeight: '1.5' },
        '.cm-content':  { padding: '6px 0' },
        '.cm-gutters':  { background: 'transparent', borderRight: '1px solid var(--border)' },
      }),
      EditorView.updateListener.of(u => {
        if (!u.docChanged || suppressUpdate) return
        emit('update:modelValue', u.state.doc.toString())
      }),
    ],
  })
  view = new EditorView({ state, parent: host.value })
})

onBeforeUnmount(() => { view?.destroy(); view = null })

// External writes (e.g. example load) → push into CodeMirror without echoing
// back through `update:modelValue`.
watch(() => props.modelValue, (v) => {
  if (!view) return
  const cur = view.state.doc.toString()
  if (cur === v) return
  suppressUpdate = true
  view.dispatch({ changes: { from: 0, to: cur.length, insert: v } })
  suppressUpdate = false
})

watch(() => props.language, (l) => {
  view?.dispatch({ effects: langCompartment.reconfigure(languageExt(l)) })
})
watch(() => props.readonly, (r) => {
  view?.dispatch({ effects: readonlyCompartment.reconfigure(EditorState.readOnly.of(r)) })
})

// Trigger a redecorate when the variable set changes (e.g. env switch).
watch(() => props.vars, (v) => {
  currentVars = { ...(v ?? {}) }
  if (!view) return
  view.dispatch({}) // forces ViewPlugin.update with viewportChanged sometimes;
  // for guaranteed redraw, dispatch a no-op effect:
  view.requestMeasure()
}, { deep: true })
</script>

<template>
  <div ref="host" class="editor-pane" :style="{ minHeight }"></div>
</template>

<style scoped>
.editor-pane {
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg-input);
  overflow: auto;
  font: 12px monospace;
}
.editor-pane :deep(.cm-editor) { height: 100%; }
.editor-pane :deep(.cm-editor.cm-focused) { outline: none; }
.editor-pane:focus-within { border-color: var(--accent); }
.editor-pane :deep(.cm-line) { color: var(--text); }
.editor-pane :deep(.tok-keyword) { color: #c678dd; }
.editor-pane :deep(.tok-string)  { color: #98c379; }
.editor-pane :deep(.tok-number)  { color: #d19a66; }
.editor-pane :deep(.tok-comment) { color: var(--text-faint); font-style: italic; }
.editor-pane :deep(.cm-var) {
  color: var(--accent);
  background: color-mix(in srgb, var(--accent) 18%, transparent);
  border-radius: 2px;
}
.editor-pane :deep(.cm-var-missing) {
  color: var(--danger);
  background: color-mix(in srgb, var(--danger) 18%, transparent);
  text-decoration: underline wavy var(--danger);
}
</style>
