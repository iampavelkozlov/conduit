import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { initializeTheme } from './composables/useTheme'
import { router } from './router'
import { useAuthStore } from './stores/auth'
import './styles/global.css'

const app = createApp(App)
const pinia = createPinia()

initializeTheme()

app.use(pinia)
app.use(router)

await useAuthStore(pinia).restoreSession()
await router.isReady()
app.mount('#app')
