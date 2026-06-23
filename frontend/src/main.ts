import { createApp } from 'vue'
import VueVirtualScroller from 'vue-virtual-scroller'
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'
import App from './App.vue'

// macOS WKWebView auto-capitalizes the first letter and pops the autocorrect
// suggestion on text inputs — wrong for URLs, keys, tokens, JSON, scripts, etc.
// Turn it off everywhere: set the (inheritable) hints on the root, then stamp
// every current/future <input>/<textarea> via a MutationObserver so no
// component has to remember.
function disableAutofix(el: Element) {
  el.setAttribute('autocapitalize', 'off')
  el.setAttribute('autocorrect', 'off')
  el.setAttribute('autocomplete', 'off')
  el.setAttribute('spellcheck', 'false')
}
function stampAll(root: ParentNode) {
  root.querySelectorAll?.('input, textarea').forEach(disableAutofix)
}
const root = document.documentElement
root.setAttribute('autocapitalize', 'off')
root.setAttribute('autocorrect', 'off')
stampAll(document)
new MutationObserver((mutations) => {
  for (const m of mutations) {
    m.addedNodes.forEach((n) => {
      if (n.nodeType !== 1) return
      const el = n as Element
      if (el.matches?.('input, textarea')) disableAutofix(el)
      stampAll(el)
    })
  }
}).observe(document.body, { childList: true, subtree: true })

createApp(App)
  .use(VueVirtualScroller)
  .mount('#app')
