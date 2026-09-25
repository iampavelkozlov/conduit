import { storeToRefs } from 'pinia'
import { defineComponent } from 'vue'
import { useRouter } from 'vue-router'

import { useAuthStore } from '../../stores/auth'
import { avatarUrl } from '../../utils/format'

export default defineComponent({
  name: 'AppHeader',
  setup() {
    const auth = useAuthStore()
    const router = useRouter()
    const { user } = storeToRefs(auth)

    function logout() {
      auth.logout()
      void router.push({ name: 'home' })
    }

    return { user, logout, avatarUrl }
  },
})
