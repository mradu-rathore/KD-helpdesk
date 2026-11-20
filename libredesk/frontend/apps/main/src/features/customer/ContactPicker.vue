<!--
  KD: pick a contact (person) with type-to-search. With a customer chosen it
  lists that customer's contacts. Emits `select` with the contact, or null.
-->
<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button
        type="button"
        variant="outline"
        role="combobox"
        :aria-expanded="open"
        class="w-full justify-between bg-transparent px-3 font-normal"
      >
        <span class="flex min-w-0 items-center gap-2">
          <UserRound class="size-4 shrink-0 text-muted-foreground" />
          <span class="truncate" :class="{ 'text-muted-foreground': !modelValue }">
            {{ modelValue ? fullName(modelValue) : placeholder }}
          </span>
        </span>
        <ChevronsUpDown class="size-4 shrink-0 opacity-50" />
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-80 p-0" align="start">
      <div class="border-b border-border p-2">
        <Input
          ref="inputRef"
          v-model="term"
          class="h-8"
          placeholder="Search name or email…"
          @keydown.down.prevent="move(1)"
          @keydown.up.prevent="move(-1)"
          @keydown.enter.prevent="chooseActive"
        />
      </div>
      <div class="max-h-64 overflow-y-auto p-1" role="listbox">
        <button
          v-if="modelValue"
          type="button"
          :class="itemClass(-1)"
          @click="choose(null)"
          @mouseenter="active = -1"
        >
          <X class="size-4 text-muted-foreground" />
          <span class="text-muted-foreground">No contact</span>
        </button>
        <p v-if="customerId && !term" class="px-2 pb-1 pt-2 text-xs text-muted-foreground">
          Contacts at this customer
        </p>
        <p v-if="loading && !results.length" class="px-2 py-3 text-sm text-muted-foreground">Searching…</p>
        <button
          v-for="(c, i) in results"
          :key="c.id"
          type="button"
          role="option"
          :aria-selected="modelValue?.id === c.id"
          :class="itemClass(i)"
          @click="choose(c)"
          @mouseenter="active = i"
        >
          <span class="min-w-0 flex-1 text-left">
            <span class="block truncate">{{ fullName(c) }}</span>
            <span class="block truncate text-xs text-muted-foreground">
              {{ [c.email, !customerId && c.customer_name].filter(Boolean).join(' · ') }}
            </span>
          </span>
          <Check v-if="modelValue?.id === c.id" class="size-4" />
        </button>
        <p v-if="!loading && !results.length" class="px-2 py-3 text-sm text-muted-foreground">
          {{ customerId && !term ? 'No contacts at this customer yet' : 'No contacts found' }}
        </p>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup>
import { nextTick, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { Check, ChevronsUpDown, UserRound, X } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Input } from '@shared-ui/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@shared-ui/components/ui/popover'
import api from '@main/api'

const props = defineProps({
  modelValue: { type: Object, default: null },
  customerId: { type: Number, default: 0 },
  placeholder: { type: String, default: 'Select contact' }
})
const emit = defineEmits(['select'])

const open = ref(false)
const term = ref('')
const results = ref([])
const loading = ref(false)
const active = ref(0)
const inputRef = ref(null)

const fullName = (c) => [c.first_name, c.last_name].filter(Boolean).join(' ') || c.email || 'Contact'

let seq = 0
const search = async () => {
  const mine = ++seq
  loading.value = true
  try {
    const q = term.value.trim()
    const params = { q, page_size: 30 }
    // Without a search term, show the chosen customer's contacts.
    if (props.customerId && !q) params.customer_id = props.customerId
    const res = await api.getKdContacts(params)
    if (mine === seq) results.value = res.data.data.results || []
  } catch {
    if (mine === seq) results.value = []
  } finally {
    if (mine === seq) {
      loading.value = false
      active.value = 0
    }
  }
}
const searchDebounced = useDebounceFn(search, 150)

watch(term, searchDebounced)
watch(open, async (isOpen) => {
  if (!isOpen) return
  term.value = ''
  search()
  await nextTick()
  inputRef.value?.$el?.focus?.()
})

const itemClass = (i) => [
  'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm outline-none transition-colors',
  active.value === i ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/60'
]

const move = (step) => {
  const count = results.value.length
  if (!count) return
  active.value = (active.value + step + count) % count
}

const choose = (contact) => {
  open.value = false
  emit('select', contact)
}

const chooseActive = () => {
  if (active.value === -1) return choose(null)
  if (results.value[active.value]) choose(results.value[active.value])
}
</script>
