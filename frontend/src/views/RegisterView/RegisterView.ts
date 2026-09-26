import { defineComponent, ref } from 'vue'
import { useRouter } from 'vue-router'
import { UserPlusIcon } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { errorMessages } from '../../api/errors'
import ErrorMessages from '../../components/ErrorMessages/ErrorMessages.vue'
import { useAuthStore } from '../../stores/auth'

export default defineComponent({
  name: 'RegisterView',
  components: { Button, Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle, ErrorMessages, Field, FieldGroup, FieldLabel, Input, Spinner, UserPlusIcon },
  setup() {
    const auth = useAuthStore()
    const router = useRouter()
    const username = ref('')
    const email = ref('')
    const password = ref('')
    const errors = ref<string[]>([])
    const submitting = ref(false)

    async function submit() {
      submitting.value = true
      errors.value = []
      try {
        await auth.register(username.value, email.value, password.value)
        await router.push({ name: 'home' })
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        submitting.value = false
      }
    }

    return { username, email, password, errors, submitting, submit }
  },
})
