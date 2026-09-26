<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'

import { sanitizeRichText } from '@/utils/richText'
import { highlightCodeBlocks } from '@/utils/syntaxHighlight'

const props = defineProps<{
  content: string | null | undefined
}>()

const safeContent = computed(() => sanitizeRichText(props.content))
const contentRoot = ref<HTMLElement>()

watch(safeContent, async () => {
  await nextTick()
  if (contentRoot.value) highlightCodeBlocks(contentRoot.value)
}, { immediate: true })
</script>

<template>
  <div ref="contentRoot" class="rich-text" v-html="safeContent"></div>
</template>
