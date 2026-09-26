import { computed, defineComponent, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { SettingsIcon, UserMinusIcon, UserPlusIcon } from '@lucide/vue'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import RichTextContent from '@/components/RichTextContent/RichTextContent.vue'
import { getArticles, getProfile, setFollowing } from '../../api/conduit'
import { errorMessages } from '../../api/errors'
import type { ArticleSummary, Profile } from '../../api/types'
import ArticleList from '../../components/ArticleList/ArticleList.vue'
import ErrorMessages from '../../components/ErrorMessages/ErrorMessages.vue'
import { useAuthStore } from '../../stores/auth'
import { avatarUrl, initials } from '../../utils/format'

export default defineComponent({
  name: 'ProfileView',
  components: { ArticleList, Avatar, AvatarFallback, AvatarImage, Button, Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle, ErrorMessages, RichTextContent, SettingsIcon, Skeleton, Spinner, Tabs, TabsList, TabsTrigger, UserMinusIcon, UserPlusIcon },
  setup() {
    const auth = useAuthStore()
    const route = useRoute()
    const router = useRouter()
    const profile = ref<Profile | null>(null)
    const articles = ref<ArticleSummary[]>([])
    const errors = ref<string[]>([])
    const loading = ref(true)
    const followingBusy = ref(false)
    const username = computed(() => String(route.params.username))
    const favorites = computed(() => route.query.tab === 'favorites')
    const ownProfile = computed(() => auth.user?.username === username.value)

    async function load() {
      loading.value = true
      errors.value = []
      try {
        const [profileData, articlePage] = await Promise.all([
          getProfile(username.value),
          getArticles(favorites.value ? { favorited: username.value, limit: 100 } : { author: username.value, limit: 100 }),
        ])
        profile.value = profileData
        articles.value = articlePage.articles
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        loading.value = false
      }
    }

    async function toggleFollow() {
      if (!auth.isAuthenticated) {
        await router.push({ name: 'login' })
        return
      }
      if (!profile.value) return

      followingBusy.value = true
      try {
        profile.value = await setFollowing(profile.value.username, !profile.value.following)
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        followingBusy.value = false
      }
    }

    function replaceArticle(article: ArticleSummary) {
      const index = articles.value.findIndex((item) => item.slug === article.slug)
      if (index !== -1) articles.value[index] = article
    }

    watch(() => route.fullPath, load, { immediate: true })

    return { auth, profile, articles, errors, loading, followingBusy, username, favorites, ownProfile, toggleFollow, replaceArticle, avatarUrl, initials }
  },
})
