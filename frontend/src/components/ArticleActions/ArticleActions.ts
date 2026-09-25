import { computed, defineComponent, ref, type PropType } from 'vue'
import { useRouter } from 'vue-router'

import { deleteArticle, setFavorite, setFollowing } from '../../api/conduit'
import { errorMessages } from '../../api/errors'
import type { Article } from '../../api/types'
import { useAuthStore } from '../../stores/auth'

export default defineComponent({
  name: 'ArticleActions',
  props: {
    article: { type: Object as PropType<Article>, required: true },
  },
  emits: {
    updated: (_article: Article) => true,
    failed: (_messages: string[]) => true,
  },
  setup(props, { emit }) {
    const auth = useAuthStore()
    const router = useRouter()
    const busy = ref(false)
    const ownArticle = computed(() => auth.user?.username === props.article.author.username)

    async function requireUser() {
      if (auth.isAuthenticated) return true
      await router.push({ name: 'login' })
      return false
    }

    async function toggleFollow() {
      if (!await requireUser()) return
      busy.value = true
      try {
        const author = await setFollowing(props.article.author.username, !props.article.author.following)
        emit('updated', { ...props.article, author })
      } catch (error) {
        emit('failed', errorMessages(error))
      } finally {
        busy.value = false
      }
    }

    async function toggleFavorite() {
      if (!await requireUser()) return
      busy.value = true
      try {
        emit('updated', await setFavorite(props.article.slug, !props.article.favorited))
      } catch (error) {
        emit('failed', errorMessages(error))
      } finally {
        busy.value = false
      }
    }

    async function removeArticle() {
      if (!window.confirm('Delete this article?')) return
      busy.value = true
      try {
        await deleteArticle(props.article.slug)
        await router.push({ name: 'home' })
      } catch (error) {
        emit('failed', errorMessages(error))
        busy.value = false
      }
    }

    return { ownArticle, busy, toggleFollow, toggleFavorite, removeArticle }
  },
})
