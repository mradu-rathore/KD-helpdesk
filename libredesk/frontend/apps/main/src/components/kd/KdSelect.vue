<!--
  KD: shadcn Select for a plain list of { value, label } options.
  `noneLabel` adds a first "nothing chosen" entry that maps to '' - shadcn's
  Select can't hold an empty value itself.
-->
<template>
  <Select :model-value="inner" :disabled="disabled" @update:model-value="onChange">
    <SelectTrigger :id="id" :aria-label="ariaLabel" :class="cn(size === 'sm' && 'h-8', triggerClass)">
      <SelectValue :placeholder="placeholder" />
    </SelectTrigger>
    <SelectContent>
      <SelectItem v-if="noneLabel" :value="NONE">
        <span :class="{ 'text-muted-foreground': noneMuted }">{{ noneLabel }}</span>
      </SelectItem>
      <SelectItem v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
    </SelectContent>
  </Select>
</template>

<script setup>
import { computed } from 'vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@shared-ui/components/ui/select'
import { cn } from '@shared-ui/lib/utils'

const NONE = '__none__'

const props = defineProps({
  modelValue: { type: String, default: '' },
  options: { type: Array, default: () => [] }, // [{ value, label }]
  placeholder: { type: String, default: 'Select' },
  noneLabel: { type: String, default: '' },
  // Grey "nothing chosen" like a placeholder; off for filters ("All brands").
  noneMuted: { type: Boolean, default: true },
  size: { type: String, default: 'default' }, // 'default' | 'sm'
  disabled: Boolean,
  id: { type: String, default: undefined },
  ariaLabel: { type: String, default: undefined },
  triggerClass: { type: [String, Array, Object], default: '' }
})
const emit = defineEmits(['update:modelValue'])

const inner = computed(() => props.modelValue || (props.noneLabel ? NONE : undefined))
const onChange = (v) => emit('update:modelValue', v === NONE ? '' : v)
</script>
