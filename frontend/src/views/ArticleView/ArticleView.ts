import { defineComponent, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { MessageSquareIcon, SendIcon, Trash2Icon } from '@lucide/vue'

import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import RichTextContent from '@/components/RichTextContent/RichTextContent.vue'
import { addComment, deleteComment, getArticle, getComments } from '../../api/conduit'
import { errorMessages } from '../../api/errors'
import type { Article, Comment } from '../../api/types'
import ArticleActions from '../../components/ArticleActions/ArticleActions.vue'
import ErrorMessages from '../../components/ErrorMessages/ErrorMessages.vue'
import { useAuthStore } from '../../stores/auth'
import { avatarUrl, formatDate, initials } from '../../utils/format'

export default defineComponent({
  name: 'ArticleView',
  components: { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger, ArticleActions, Avatar, AvatarFallback, AvatarImage, Badge, Button, Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle, Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle, ErrorMessages, Field, FieldGroup, FieldLabel, MessageSquareIcon, RichTextContent, SendIcon, Skeleton, Spinner, Textarea, Trash2Icon },
  setup() {
    const auth = useAuthStore()
    const route = useRoute()
    const article = ref<Article | null>(null)
    const comments = ref<Comment[]>([])
    const commentBody = ref('')
    const errors = ref<string[]>([])
    const loading = ref(true)
    const submittingComment = ref(false)

    async function load() {
      loading.value = true
      errors.value = []
      try {
        const slug = String(route.params.slug)
        const [articleData, commentData] = await Promise.all([getArticle(slug), getComments(slug)])
        article.value = articleData
        comments.value = commentData
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        loading.value = false
      }
    }

    async function submitComment() {
      if (!article.value) return
      submittingComment.value = true
      errors.value = []
      try {
        const comment = await addComment(article.value.slug, commentBody.value)
        comments.value.unshift(comment)
        commentBody.value = ''
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        submittingComment.value = false
      }
    }

    async function removeComment(comment: Comment) {
      if (!article.value) return
      try {
        await deleteComment(article.value.slug, comment.id)
        comments.value = comments.value.filter((item) => item.id !== comment.id)
      } catch (error) {
        errors.value = errorMessages(error)
      }
    }

    function setErrors(messages: string[]) {
      errors.value = messages
    }

    watch(() => route.params.slug, load, { immediate: true })

    return {
      auth,
      article,
      comments,
      commentBody,
      errors,
      loading,
      submittingComment,
      submitComment,
      removeComment,
      setErrors,
      avatarUrl,
      formatDate,
      initials,
    }
  },
})
