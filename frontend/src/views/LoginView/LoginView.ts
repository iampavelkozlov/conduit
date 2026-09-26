import { defineComponent, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { LogInIcon } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { errorMessages } from '../../api/errors'
import ErrorMessages from '../../components/ErrorMessages/ErrorMessages.vue'
import { useAuthStore } from '../../stores/auth'

export default defineComponent({
  name: 'LoginView',
  components: { Button, Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle, ErrorMessages, Field, FieldGroup, FieldLabel, Input, LogInIcon, Spinner },
  setup() {
    const auth = useAuthStore()
    const route = useRoute()
    const router = useRouter()
    const email = ref('')
    const password = ref('')
    const errors = ref<string[]>([])
    const submitting = ref(false)

    async function submit() {
      submitting.value = true
      errors.value = []
      try {
        await auth.login(email.value, password.value)
        const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
        await router.push(redirect)
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        submitting.value = false
      }
    }

    return { email, password, errors, submitting, submit }
  },
})
