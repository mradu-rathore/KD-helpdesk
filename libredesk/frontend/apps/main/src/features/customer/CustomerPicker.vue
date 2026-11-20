<!--
  KD: pick a customer (company) with type-to-search, or create one inline.
  Emits `select` with the customer object, or null for "No customer".
-->
<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <slot name="trigger" :open="open">
        <Button
          type="button"
          variant="outline"
          role="combobox"
          :aria-expanded="open"
          class="w-full justify-between bg-transparent px-3 font-normal"
        >
          <span class="flex min-w-0 items-center gap-2">
            <Building2 class="size-4 shrink-0 text-muted-foreground" />
            <span class="truncate" :class="{ 'text-muted-foreground': !modelValue }">
              {{ modelValue?.name || placeholder }}
            </span>
          </span>
          <ChevronsUpDown class="size-4 shrink-0 opacity-50" />
        </Button>
      </slot>
    </PopoverTrigger>
    <PopoverContent class="w-72 p-0" align="start">
      <div class="border-b border-border p-2">
        <Input
          ref="inputRef"
          v-model="term"
          class="h-8"
          placeholder="Search customers…"
          @keydown.down.prevent="move(1)"
          @keydown.up.prevent="move(-1)"
          @keydown.enter.prevent="chooseActive"
        />
      </div>
      <div class="max-h-64 overflow-y-auto p-1" role="listbox">
        <button
          v-if="allowNone && modelValue"
          type="button"
          :class="itemClass(-1)"
          @click="choose(null)"
          @mouseenter="active = -1"
        >
          <X class="size-4 text-muted-foreground" />
          <span class="text-muted-foreground">{{ noneLabel }}</span>
        </button>
        <p v-if="loading && !results.length" class="px-2 py-3 text-sm text-muted-foreground">
          Searching…
        </p>
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
          <Building2 class="size-4 text-muted-foreground" />
          <span class="min-w-0 flex-1 truncate text-left">{{ c.name }}</span>
          <span v-if="c.domain" class="truncate text-xs text-muted-foreground">{{ c.domain }}</span>
          <Check v-if="modelValue?.id === c.id" class="size-4" />
        </button>
        <p
          v-if="!loading && !results.length && !canCreate"
          class="px-2 py-3 text-sm text-muted-foreground"
        >
          No customers found
        </p>
        <button
          v-if="canCreate"
          type="button"
          :class="itemClass(results.length)"
          :disabled="creating"
          @click="createAndChoose"
          @mouseenter="active = results.length"
        >
          <Plus class="size-4" />
          <span class="truncate">Create “{{ term.trim() }}”</span>
        </button>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { Building2, Check, ChevronsUpDown, Plus, X } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Input } from '@shared-ui/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@shared-ui/components/ui/popover'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { useEmitter } from '@main/composables/useEmitter'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents'
import { useUserStore } from '@main/stores/user'
import { invalidateCached } from '@main/composables/useCachedResource'
import api from '@main/api'

const props = defineProps({
  modelValue: { type: Object, default: null },
  placeholder: { type: String, default: 'Select customer' },
  allowNone: { type: Boolean, default: true },
  noneLabel: { type: String, default: 'No customer' }
})
const emit = defineEmits(['select'])

const emitter = useEmitter()
const userStore = useUserStore()
const open = ref(false)
const term = ref('')
const results = ref([])
const loading = ref(false)
const creating = ref(false)
const active = ref(0)
const inputRef = ref(null)

const canCreate = computed(() => {
  const name = term.value.trim()
  if (!name || !userStore.can('contacts:write')) return false
  return !results.value.some((c) => c.name.toLowerCase() === name.toLowerCase())
})

let seq = 0
const search = async () => {
  const mine = ++seq
  loading.value = true
  try {
    const res = await api.getCustomers({ q: term.value.trim(), page_size: 50 })
    if (mine === seq) results.value = res.data.data.results || []
  } catch (err) {
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
  const count = results.value.length + (canCreate.value ? 1 : 0)
  if (!count) return
  active.value = (active.value + step + count) % count
}

const choose = (customer) => {
  open.value = false
  if ((customer?.id ?? null) !== (props.modelValue?.id ?? null)) emit('select', customer)
}

const chooseActive = () => {
  if (active.value === -1) return choose(null)
  if (active.value < results.value.length) return choose(results.value[active.value])
  if (canCreate.value) createAndChoose()
}

const createAndChoose = async () => {
  creating.value = true
  try {
    const res = await api.createCustomer({ name: term.value.trim() })
    invalidateCached('customers')
    choose(res.data.data)
  } catch (err) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(err).message
    })
  } finally {
    creating.value = false
  }
}
</script>
