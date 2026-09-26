<script setup lang="ts">
import { computed } from 'vue'

import { sanitizeRichText } from '@/utils/richText'

const props = defineProps<{
  content: string | null | undefined
}>()

const safeContent = computed(() => sanitizeRichText(props.content))
</script>

<template>
  <div class="rich-text" v-html="safeContent"></div>
</template>
