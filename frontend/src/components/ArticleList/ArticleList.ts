import { defineComponent, type PropType } from 'vue'
import { FileTextIcon } from '@lucide/vue'

import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import type { ArticleSummary } from '../../api/types'
import ArticlePreview from '../ArticlePreview/ArticlePreview.vue'

export default defineComponent({
  name: 'ArticleList',
  components: { ArticlePreview, Card, CardContent, CardHeader, Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle, FileTextIcon, Skeleton },
  props: {
    articles: { type: Array as PropType<ArticleSummary[]>, required: true },
    loading: { type: Boolean, default: false },
  },
  emits: {
    updated: (_article: ArticleSummary) => true,
  },
})
