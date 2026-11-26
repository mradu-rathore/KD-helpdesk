<!-- KD: collapse / expand the main sidebar (also ⌘B / Ctrl+B). -->
<template>
  <Tooltip>
    <TooltipTrigger as-child>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        class="size-7 shrink-0 text-muted-foreground hover:text-foreground"
        :aria-label="label"
        :aria-expanded="!collapsed"
        @click="toggle"
      >
        <PanelLeft class="size-4" />
      </Button>
    </TooltipTrigger>
    <TooltipContent side="right">
      {{ label }} <kbd class="ml-1 font-sans text-[10px] opacity-70">{{ shortcut }}</kbd>
    </TooltipContent>
  </Tooltip>
</template>

<script setup>
import { computed } from 'vue'
import { PanelLeft } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@shared-ui/components/ui/tooltip'
import { useKdSidebar } from '@main/composables/useKdSidebar'

const { collapsed, toggle } = useKdSidebar()
const label = computed(() => (collapsed.value ? 'Show sidebar' : 'Hide sidebar'))
const shortcut = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘B' : 'Ctrl+B'
</script>
