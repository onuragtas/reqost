<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { EditorState, Compartment, RangeSetBuilder } from '@codemirror/state'
import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection, Decoration, type DecorationSet, ViewPlugin, type ViewUpdate } from '@codemirror/view'
import {
  defaultHighlightStyle, syntaxHighlighting, bracketMatching, indentOnInput, foldGutter, foldKeymap,
} from '@codemirror/language'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { searchKeymap, highlightSelectionMatches, search } from '@codemirror/search'
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
      // `top: true` floats the panel above the editor (looks like a modal
      // overlay instead of squeezing the gutter).
      // `caseSensitive: false`, `regexp: false` are the defaults — keep them
      // explicit so an upstream change can't flip them on us.
      search({ top: true, caseSensitive: false, regexp: false, wholeWord: false }),
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
        '&':            { fontFamily: 'monospace', fontSize: '12px', backgroundColor: 'transparent' },
        '.cm-scroller': { lineHeight: '1.5' },
        '.cm-content':  { padding: '6px 0', caretColor: 'var(--accent)' },
        '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--accent)' },
        '.cm-gutters':  { background: 'transparent', borderRight: '1px solid var(--border)', color: 'var(--text-faint)' },
        '.cm-activeLine, .cm-activeLineGutter': {
          backgroundColor: 'color-mix(in srgb, var(--accent) 6%, transparent)',
        },
        '.cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection': {
          backgroundColor: 'color-mix(in srgb, var(--accent) 32%, transparent) !important',
        },
        '.cm-searchMatch': {
          backgroundColor: 'color-mix(in srgb, var(--warn-text) 35%, transparent)',
          outline: '1px solid color-mix(in srgb, var(--warn-text) 60%, transparent)',
        },
        '.cm-searchMatch-selected': {
          backgroundColor: 'color-mix(in srgb, var(--accent) 50%, transparent)',
          outline: '1px solid var(--accent)',
        },
        '.cm-selectionMatch': {
          backgroundColor: 'color-mix(in srgb, var(--accent) 22%, transparent)',
        },
        // ── Search / find panel ──
        '.cm-panels': {
          backgroundColor: 'var(--bg-elevated)',
          color: 'var(--text)',
          border: '0',
          borderTop: '1px solid var(--border)',
        },
        '.cm-panels-top': { borderTop: '0', borderBottom: '1px solid var(--border)' },
        '.cm-panel.cm-search': {
          background: 'var(--bg-elevated)',
          padding: '8px 10px',
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          gap: '6px',
          fontFamily: 'inherit',
        },
        '.cm-panel.cm-search label': {
          display: 'inline-flex',
          alignItems: 'center',
          gap: '4px',
          fontSize: '11px',
          color: 'var(--text-dim)',
          cursor: 'pointer',
          padding: '2px 4px',
          borderRadius: '3px',
        },
        '.cm-panel.cm-search label:hover': { color: 'var(--text)' },
        '.cm-panel.cm-search label input[type=checkbox]': {
          accentColor: 'var(--accent)',
          margin: '0 2px 0 0',
        },
        '.cm-panel.cm-search input[type=text], .cm-textfield': {
          background: 'var(--bg-input)',
          color: 'var(--text)',
          border: '1px solid var(--border-strong)',
          borderRadius: '5px',
          padding: '5px 9px',
          font: '12px monospace',
          minWidth: '180px',
          outline: 'none',
        },
        '.cm-panel.cm-search input[type=text]:focus, .cm-textfield:focus': {
          borderColor: 'var(--accent)',
          boxShadow: '0 0 0 2px color-mix(in srgb, var(--accent) 20%, transparent)',
        },
        '.cm-panel.cm-search button, .cm-button': {
          background: 'var(--bg-input)',
          color: 'var(--text-dim)',
          border: '1px solid var(--border-strong)',
          borderRadius: '5px',
          padding: '4px 10px',
          font: '11px sans-serif',
          textTransform: 'none',
          cursor: 'pointer',
          backgroundImage: 'none',
        },
        '.cm-panel.cm-search button:hover, .cm-button:hover': {
          background: 'var(--bg-hover)',
          color: 'var(--text)',
          borderColor: 'var(--accent)',
        },
        '.cm-panel.cm-search button[name=close]': {
          marginLeft: 'auto',
          color: 'var(--text-faint)',
          background: 'transparent',
          border: '0',
          fontSize: '15px',
          padding: '2px 6px',
          lineHeight: '1',
        },
        '.cm-panel.cm-search button[name=close]:hover': { color: 'var(--danger)' },
        // ── Autocomplete tooltip ──
        '.cm-tooltip.cm-tooltip-autocomplete': {
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-strong)',
          borderRadius: '6px',
          boxShadow: '0 8px 24px rgba(0,0,0,0.35)',
          color: 'var(--text)',
        },
        '.cm-tooltip.cm-tooltip-autocomplete > ul > li': {
          padding: '4px 10px',
          fontFamily: 'monospace',
          fontSize: '12px',
        },
        '.cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected]': {
          background: 'var(--bg-hover)',
          color: 'var(--text)',
        },
        '.cm-completionLabel': { color: 'var(--accent)' },
        '.cm-completionDetail': { color: 'var(--text-faint)', fontStyle: 'normal', marginLeft: '8px' },
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
