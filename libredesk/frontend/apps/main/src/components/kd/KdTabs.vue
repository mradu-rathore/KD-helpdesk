<!--
  KD: underline tabs with counts, on shadcn Tabs - the same style as the
  ticket sidebar's Details tab. Only the tab strip; render the content below.
-->
<template>
  <Tabs :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)">
    <TabsList class="h-auto w-full justify-start gap-4 overflow-x-auto rounded-none bg-transparent p-0 [scrollbar-width:none]">
      <TabsTrigger v-for="tab in tabs" :key="tab.key" :value="tab.key" :class="TAB_CLASS">
        {{ tab.label }}
        <span
          v-if="tab.count !== undefined"
          class="ml-1.5 rounded bg-muted px-1.5 text-[11px] font-medium tabular-nums text-muted-foreground"
        >
          {{ tab.count }}
        </span>
      </TabsTrigger>
    </TabsList>
  </Tabs>
</template>

<script setup>
import { Tabs, TabsList, TabsTrigger } from '@shared-ui/components/ui/tabs'

defineProps({
  modelValue: { type: String, required: true },
  tabs: { type: Array, required: true } // [{ key, label, count? }]
})
const emit = defineEmits(['update:modelValue'])

// As ConversationSideBar's SIDEBAR_TAB_CLASS, at body-text size.
const TAB_CLASS =
  'shrink-0 rounded-none border-b-2 border-transparent px-0 py-2.5 text-sm font-normal text-muted-foreground hover:text-foreground focus-visible:ring-inset focus-visible:ring-offset-0 data-[state=active]:border-foreground data-[state=active]:bg-transparent data-[state=active]:font-medium data-[state=active]:text-foreground data-[state=active]:shadow-none'
</script>
