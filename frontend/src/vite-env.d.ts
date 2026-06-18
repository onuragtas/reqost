/// <reference types="vite/client" />

declare module 'vue-virtual-scroller' {
  import type { Plugin, Component } from 'vue'
  const plugin: Plugin
  export default plugin
  export const RecycleScroller: Component
  export const DynamicScroller: Component
  export const DynamicScrollerItem: Component
}
