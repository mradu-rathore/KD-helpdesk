<!--
  KD: date field as the app does it elsewhere (outline button + Popover +
  shadcn Calendar). v-model is a 'YYYY-MM-DD' string, '' when empty.
-->
<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button
        :id="id"
        type="button"
        variant="outline"
        :disabled="disabled"
        :class="cn('w-full justify-start gap-2 bg-transparent px-3 text-left font-normal', !modelValue && 'text-muted-foreground')"
      >
        <CalendarDays class="size-4 shrink-0 opacity-60" />
        <span class="truncate">{{ label }}</span>
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-auto p-0" align="start">
      <Calendar :model-value="value" :max-value="maxValue" initial-focus @update:model-value="pick" />
      <div v-if="modelValue" class="border-t border-border p-1.5">
        <Button type="button" variant="ghost" size="sm" class="h-7 w-full text-muted-foreground" @click="pick(null)">
          Clear date
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup>
import { computed, ref } from 'vue'
import { format } from 'date-fns'
import { parseDate } from '@internationalized/date'
import { CalendarDays } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Calendar } from '@shared-ui/components/ui/calendar'
import { Popover, PopoverContent, PopoverTrigger } from '@shared-ui/components/ui/popover'
import { cn } from '@shared-ui/lib/utils'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: 'Pick a date' },
  max: { type: String, default: '' }, // 'YYYY-MM-DD'
  disabled: Boolean,
  id: { type: String, default: undefined }
})
const emit = defineEmits(['update:modelValue'])
const open = ref(false)

const safeParse = (s) => {
  try {
    return s ? parseDate(s) : undefined
  } catch {
    return undefined
  }
}
const value = computed(() => safeParse(props.modelValue))
const maxValue = computed(() => safeParse(props.max))
const label = computed(() =>
  value.value ? format(new Date(`${props.modelValue}T00:00:00`), 'd MMM yyyy') : props.placeholder
)

function pick (v) {
  emit('update:modelValue', v ? v.toString() : '')
  open.value = false
}
</script>
