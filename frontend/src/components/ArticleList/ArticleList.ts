import { defineComponent, type PropType } from 'vue'

import type { ArticleSummary } from '../../api/types'
import ArticlePreview from '../ArticlePreview/ArticlePreview.vue'

export default defineComponent({
  name: 'ArticleList',
  components: { ArticlePreview },
  props: {
    articles: { type: Array as PropType<ArticleSummary[]>, required: true },
    loading: { type: Boolean, default: false },
  },
  emits: {
    updated: (_article: ArticleSummary) => true,
  },
})
