import { defineComponent } from 'vue'
import { ArrowLeftIcon, SearchXIcon } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'

export default defineComponent({
  name: 'NotFoundView',
  components: { ArrowLeftIcon, Button, Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle, SearchXIcon },
})
