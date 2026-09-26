import { computed, defineComponent, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { HashIcon, SparklesIcon } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Pagination, PaginationContent, PaginationItem, PaginationNext, PaginationPrevious } from '@/components/ui/pagination'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getArticles, getFeed, getTags } from '../../api/conduit'
import { errorMessages } from '../../api/errors'
import type { ArticleSummary } from '../../api/types'
import ArticleList from '../../components/ArticleList/ArticleList.vue'
import ErrorMessages from '../../components/ErrorMessages/ErrorMessages.vue'
import { useAuthStore } from '../../stores/auth'

const pageSize = 10

export default defineComponent({
  name: 'HomeView',
  components: { ArticleList, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle, ErrorMessages, HashIcon, Pagination, PaginationContent, PaginationItem, PaginationNext, PaginationPrevious, SparklesIcon, Tabs, TabsList, TabsTrigger },
  setup() {
    const auth = useAuthStore()
    const route = useRoute()
    const router = useRouter()
    const articles = ref<ArticleSummary[]>([])
    const tags = ref<string[]>([])
    const total = ref(0)
    const loading = ref(true)
    const errors = ref<string[]>([])

    const page = computed(() => Math.max(1, Number(route.query.page) || 1))
    const activeTag = computed(() => String(route.query.tag ?? ''))
    const following = computed(() => route.query.feed === 'following')
    const pageCount = computed(() => Math.ceil(total.value / pageSize))

    async function load() {
      loading.value = true
      errors.value = []
      const offset = (page.value - 1) * pageSize

      try {
        const [articlePage, popularTags] = await Promise.all([
          following.value ? getFeed(offset, pageSize) : getArticles({ tag: activeTag.value || undefined, offset, limit: pageSize }),
          getTags(),
        ])
        articles.value = articlePage.articles
        total.value = articlePage.articlesCount
        tags.value = popularTags
      } catch (error) {
        errors.value = errorMessages(error)
      } finally {
        loading.value = false
      }
    }

    function replaceArticle(article: ArticleSummary) {
      const index = articles.value.findIndex((item) => item.slug === article.slug)
      if (index !== -1) articles.value[index] = article
    }

    function openFeed(feed: 'global' | 'following') {
      void router.push(feed === 'following' ? { name: 'home', query: { feed: 'following' } } : { name: 'home' })
    }

    function openPage(nextPage: number) {
      void router.push({ name: 'home', query: { ...route.query, page: nextPage } })
    }

    watch(() => route.fullPath, load, { immediate: true })

    return {
      auth,
      articles,
      tags,
      total,
      loading,
      errors,
      page,
      pageCount,
      activeTag,
      following,
      replaceArticle,
      openFeed,
      openPage,
    }
  },
})
