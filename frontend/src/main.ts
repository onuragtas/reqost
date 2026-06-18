import { createApp } from 'vue'
import VueVirtualScroller from 'vue-virtual-scroller'
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'
import App from './App.vue'

createApp(App)
  .use(VueVirtualScroller)
  .mount('#app')
