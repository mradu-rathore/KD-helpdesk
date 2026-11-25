<!--
  KD: pick from suggestions or type your own (e.g. brand: Dahua, Ajax,
  Alta Labs, or anything else KD sells). shadcn Popover + Command, same
  trigger look as the other pickers.
-->
<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button
        :id="id"
        type="button"
        variant="outline"
        role="combobox"
        :aria-expanded="open"
        :disabled="disabled"
        class="w-full justify-between bg-transparent px-3 font-normal"
      >
        <span class="truncate" :class="{ 'text-muted-foreground': !modelValue }">{{ modelValue || placeholder }}</span>
        <ChevronsUpDown class="size-4 shrink-0 opacity-50" />
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-56 p-0" align="start">
      <!-- Filtering is done here (as the core ComboBox does): Command's own filter
           hides the "Use …" item, whose value changes as you type. -->
      <Command v-model:search-term="term" :filter-function="passThrough">
        <CommandInput class="h-9" :placeholder="searchPlaceholder" @keydown.enter.prevent="useTyped" />
        <CommandList>
          <CommandGroup>
            <CommandItem v-for="s in matches" :key="s" :value="s" @select="choose(s)">
              {{ s }}
              <Check class="ml-auto size-4" :class="s === modelValue ? 'opacity-100' : 'opacity-0'" />
            </CommandItem>
            <!-- fixed value: the Command keeps the value an item had when it appeared -->
            <CommandItem v-if="typed && !exact" key="__typed__" value="__typed__" @select="choose(typed)">
              <Plus class="size-4 text-muted-foreground" /> Use “{{ typed }}”
            </CommandItem>
          </CommandGroup>
          <p v-if="!typed && !matches.length" class="px-2 py-3 text-sm text-muted-foreground">Type to add one</p>
        </CommandList>
      </Command>
      <div v-if="modelValue" class="border-t border-border p-1">
        <Button type="button" variant="ghost" size="sm" class="h-7 w-full text-muted-foreground" @click="choose('')">
          Clear
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { Check, ChevronsUpDown, Plus } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@shared-ui/components/ui/popover'
import { Command, CommandGroup, CommandInput, CommandItem, CommandList } from '@shared-ui/components/ui/command'

const props = defineProps({
  modelValue: { type: String, default: '' },
  suggestions: { type: Array, default: () => [] },
  placeholder: { type: String, default: 'Choose' },
  searchPlaceholder: { type: String, default: 'Search or type…' },
  disabled: Boolean,
  id: { type: String, default: undefined }
})
const emit = defineEmits(['update:modelValue'])

const open = ref(false)
const term = ref('')
watch(open, (o) => o && (term.value = ''))

const typed = computed(() => term.value.trim())
const matches = computed(() =>
  props.suggestions.filter((s) => s.toLowerCase().includes(typed.value.toLowerCase()))
)
const exact = computed(() => props.suggestions.some((s) => s.toLowerCase() === typed.value.toLowerCase()))
const passThrough = (items) => items

function choose (v) {
  emit('update:modelValue', v)
  open.value = false
}
// Enter with no highlighted match keeps what was typed
const useTyped = () => typed.value && !exact.value && choose(typed.value)
</script>
