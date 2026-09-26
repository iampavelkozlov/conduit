import { defineComponent, ref, type PropType } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRightIcon, HeartIcon } from '@lucide/vue'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { setFavorite } from '../../api/conduit'
import type { ArticleSummary } from '../../api/types'
import { useAuthStore } from '../../stores/auth'
import { avatarUrl, formatDate, initials } from '../../utils/format'

export default defineComponent({
  name: 'ArticlePreview',
  components: {
    ArrowRightIcon,
    Avatar,
    AvatarFallback,
    AvatarImage,
    Badge,
    Button,
    Card,
    CardAction,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
    HeartIcon,
  },
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

    return { busy, toggleFavorite, avatarUrl, formatDate, initials }
  },
})
