import { defineComponent, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { addComment, deleteComment, getArticle, getComments } from '../../api/conduit'
import { errorMessages } from '../../api/errors'
import type { Article, Comment } from '../../api/types'
import ArticleActions from '../../components/ArticleActions/ArticleActions.vue'
import ErrorMessages from '../../components/ErrorMessages/ErrorMessages.vue'
import { useAuthStore } from '../../stores/auth'
import { avatarUrl, formatDate } from '../../utils/format'

export default defineComponent({
  name: 'ArticleView',
  components: { ArticleActions, ErrorMessages },
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
      if (!article.value || !window.confirm('Delete this comment?')) return
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
    }
  },
})
