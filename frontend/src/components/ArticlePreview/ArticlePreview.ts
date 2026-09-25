import { defineComponent, ref, type PropType } from 'vue'
import { useRouter } from 'vue-router'

import { setFavorite } from '../../api/conduit'
import type { ArticleSummary } from '../../api/types'
import { useAuthStore } from '../../stores/auth'
import { avatarUrl, formatDate } from '../../utils/format'

export default defineComponent({
  name: 'ArticlePreview',
  props: {
    article: { type: Object as PropType<ArticleSummary>, required: true },
  },
  emits: {
    updated: (_article: ArticleSummary) => true,
  },
  setup(props, { emit }) {
    const auth = useAuthStore()
    const router = useRouter()
    const busy = ref(false)

    async function toggleFavorite() {
      if (!auth.isAuthenticated) {
        await router.push({ name: 'login' })
        return
      }

      busy.value = true
      try {
        const article = await setFavorite(props.article.slug, !props.article.favorited)
        emit('updated', article)
      } finally {
        busy.value = false
      }
    }

    return { busy, toggleFavorite, avatarUrl, formatDate }
  },
})
