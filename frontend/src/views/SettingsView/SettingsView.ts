import { defineComponent, ref } from 'vue'
import { useRouter } from 'vue-router'

import { errorMessages } from '../../api/errors'
import ErrorMessages from '../../components/ErrorMessages/ErrorMessages.vue'
import { useAuthStore } from '../../stores/auth'

export default defineComponent({
  name: 'SettingsView',
  components: { ErrorMessages },
  setup() {
    const auth = useAuthStore()
    const router = useRouter()
    const image = ref(auth.user?.image ?? '')
    const username = ref(auth.user?.username ?? '')
    const bio = ref(auth.user?.bio ?? '')
    const email = ref(auth.user?.email ?? '')
    const password = ref('')
    const errors = ref<string[]>([])
    const submitting = ref(false)

    async function submit() {
      submitting.value = true
      errors.value = []
      try {
        await auth.update({
          image: image.value,
          username: username.value,
          bio: bio.value,
          email: email.value,
          ...(password.value ? { password: password.value } : {}),
        })
        await router.push({ name: 'profile', params: { username: auth.user?.username } })
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        submitting.value = false
      }
    }

    function logout() {
      auth.logout()
      void router.push({ name: 'home' })
    }

    return { image, username, bio, email, password, errors, submitting, submit, logout }
  },
})
