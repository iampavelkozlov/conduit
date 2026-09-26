<script setup lang="ts">
import { MoonIcon, SunIcon } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { useTheme } from '@/composables/useTheme'

const { theme, toggleTheme } = useTheme()
</script>

<template>
  <TooltipProvider>
    <Tooltip>
      <TooltipTrigger as-child>
        <Button variant="ghost" size="icon" type="button" :aria-label="theme === 'light' ? 'Switch to dark theme' : 'Switch to light theme'" @click="toggleTheme">
          <MoonIcon v-if="theme === 'light'" />
          <SunIcon v-else />
        </Button>
      </TooltipTrigger>
      <TooltipContent>{{ theme === 'light' ? 'Dark theme' : 'Light theme' }}</TooltipContent>
    </Tooltip>
  </TooltipProvider>
</template>
