import { computed, defineComponent, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { SendIcon } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { createArticle, getArticle, updateArticle } from '../../api/conduit'
import { errorMessages } from '../../api/errors'
import ErrorMessages from '../../components/ErrorMessages/ErrorMessages.vue'
import { useAuthStore } from '../../stores/auth'
import { splitTags } from '../../utils/format'

export default defineComponent({
  name: 'EditorView',
  components: { Button, Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle, ErrorMessages, Field, FieldDescription, FieldGroup, FieldLabel, Input, SendIcon, Skeleton, Spinner, Textarea },
  setup() {
    const auth = useAuthStore()
    const route = useRoute()
    const router = useRouter()
    const title = ref('')
    const description = ref('')
    const body = ref('')
    const tags = ref('')
    const errors = ref<string[]>([])
    const loading = ref(false)
    const submitting = ref(false)
    const slug = computed(() => typeof route.params.slug === 'string' ? route.params.slug : '')
    const editing = computed(() => Boolean(slug.value))

    async function load() {
      if (!slug.value) return
      loading.value = true
      errors.value = []
      try {
        const article = await getArticle(slug.value)
        if (article.author.username !== auth.user?.username) {
          await router.replace({ name: 'article', params: { slug: article.slug } })
          return
        }
        title.value = article.title
        description.value = article.description
        body.value = article.body
        tags.value = article.tagList.join(', ')
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        loading.value = false
      }
    }

    async function submit() {
      submitting.value = true
      errors.value = []
      const details = { title: title.value, description: description.value, body: body.value, tagList: splitTags(tags.value) }
      try {
        const article = editing.value ? await updateArticle(slug.value, details) : await createArticle(details)
        await router.push({ name: 'article', params: { slug: article.slug } })
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        submitting.value = false
      }
    }

    watch(slug, load, { immediate: true })

    return { title, description, body, tags, errors, loading, submitting, editing, submit }
  },
})
