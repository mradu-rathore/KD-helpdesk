<!--
  KD: log a phone call in a few seconds, while still on the phone. Find the
  caller by number, name or email (or type a new caller), pick one of their
  open tickets or a new one, type what was said, done. From a ticket it goes
  straight on that ticket.
-->
<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-h-[90vh] overflow-y-auto sm:max-w-lg">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2"><PhoneCall class="size-4" /> Log a call</DialogTitle>
        <DialogDescription>
          <template v-if="ticket">On ticket #{{ ticket.reference_number }}{{ ticket.contact_name ? ` with ${ticket.contact_name}` : '' }}.</template>
          <template v-else>Who called, and what about. Nothing is sent to the customer.</template>
        </DialogDescription>
      </DialogHeader>

      <!-- shortcut on the form, as New ticket does: the dialog component doesn't pass key events on -->
      <form class="space-y-4" novalidate @submit.prevent="save" @keydown.meta.enter.prevent="save" @keydown.ctrl.enter.prevent="save">
        <!-- Caller -->
        <div v-if="!ticket" class="space-y-1.5">
          <Label for="kd-call-caller">Caller</Label>

          <div v-if="caller" class="flex items-center gap-3 rounded-lg border border-border px-3 py-2">
            <component :is="caller.kind === 'customer' ? Building2 : UserRound" class="size-4 shrink-0 text-muted-foreground" />
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{{ caller.name }}</p>
              <p class="truncate text-xs text-muted-foreground">
                {{ [caller.phone, caller.kind === 'contact' ? caller.customer_name : 'Main line'].filter(Boolean).join(' · ') || 'No phone on file' }}
              </p>
            </div>
            <Button type="button" variant="ghost" size="sm" class="h-7" @click="clearCaller">Change</Button>
          </div>

          <div v-else-if="newCaller" class="space-y-2 rounded-lg border border-border p-3">
            <div class="flex items-center justify-between">
              <p class="text-sm font-medium">New caller</p>
              <Button type="button" variant="ghost" size="sm" class="h-7" @click="clearCaller">Search instead</Button>
            </div>
            <div class="grid gap-2 sm:grid-cols-2">
              <Input ref="nameRef" v-model="newCaller.name" placeholder="Name" aria-label="Caller's name" />
              <Input v-model="newCaller.phone" placeholder="Phone" aria-label="Caller's phone" />
            </div>
            <CustomerPicker :model-value="newCaller.customer" placeholder="Their company (optional)" @select="(c) => (newCaller.customer = c)" />
          </div>

          <template v-else>
            <div class="relative">
              <Search class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                id="kd-call-caller"
                ref="searchRef"
                v-model="term"
                class="pl-9"
                placeholder="Phone number, name or email"
                autocomplete="off"
                @keydown.enter.prevent="pickFirst"
              />
            </div>
            <div v-if="term.trim().length >= 2" class="max-h-56 overflow-y-auto rounded-lg border border-border p-1">
              <p v-if="searching && !hasResults" class="px-2 py-2 text-sm text-muted-foreground">Searching…</p>
              <button
                v-for="c in results.contacts"
                :key="'c' + c.id"
                type="button"
                class="flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left hover:bg-accent"
                @click="pick(c)"
              >
                <UserRound class="size-4 shrink-0 text-muted-foreground" />
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-sm font-medium">{{ c.name }}</span>
                  <span class="block truncate text-xs text-muted-foreground">{{ [c.phone, c.customer_name].filter(Boolean).join(' · ') }}</span>
                </span>
                <span v-if="c.open_tickets.length" class="shrink-0 text-xs text-muted-foreground">{{ c.open_tickets.length }} open</span>
              </button>
              <button
                v-for="c in results.customers"
                :key="'k' + c.id"
                type="button"
                class="flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left hover:bg-accent"
                @click="pick(c)"
              >
                <Building2 class="size-4 shrink-0 text-muted-foreground" />
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-sm font-medium">{{ c.name }}</span>
                  <span class="block truncate text-xs text-muted-foreground">{{ c.phone ? `${c.phone} · main line` : 'Company' }}</span>
                </span>
              </button>
              <button
                type="button"
                class="flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent"
                @click="startNewCaller"
              >
                <Plus class="size-4 shrink-0 text-muted-foreground" />
                New caller “{{ term.trim() }}”
              </button>
            </div>
          </template>
        </div>

        <!-- Which ticket -->
        <div v-if="!ticket && (caller || newCaller)" class="space-y-1.5">
          <Label>Ticket</Label>
          <RadioGroup v-model="target" class="gap-1.5">
            <Label
              v-for="opt in targetOptions"
              :key="opt.value"
              :for="`kd-call-target-${opt.value}`"
              class="flex cursor-pointer items-center gap-2.5 rounded-lg border px-3 py-2 font-normal transition-colors"
              :class="target === opt.value ? 'border-foreground bg-accent/50' : 'border-border hover:bg-accent/30'"
            >
              <RadioGroupItem :id="`kd-call-target-${opt.value}`" :value="opt.value" />
              <span class="min-w-0 flex-1 truncate text-sm">{{ opt.label }}</span>
              <span v-if="opt.status" :class="[BADGE, statusClass(opt.status)]">{{ opt.status }}</span>
            </Label>
          </RadioGroup>
          <Input v-if="target === 'new'" v-model="subject" placeholder="Subject (optional - taken from the notes)" aria-label="Subject" />
        </div>

        <!-- The call -->
        <div class="grid gap-3 sm:grid-cols-[1fr_7rem]">
          <div class="space-y-1.5">
            <Label>Direction</Label>
            <Tabs v-model="direction">
              <TabsList class="grid h-9 w-full grid-cols-2">
                <TabsTrigger value="incoming"><PhoneIncoming class="mr-1.5 size-3.5" /> Incoming</TabsTrigger>
                <TabsTrigger value="outgoing"><PhoneOutgoing class="mr-1.5 size-3.5" /> Outgoing</TabsTrigger>
              </TabsList>
            </Tabs>
          </div>
          <div class="space-y-1.5">
            <Label for="kd-call-mins">Minutes</Label>
            <Input id="kd-call-mins" v-model="minutes" type="number" min="0" placeholder="—" />
          </div>
        </div>

        <div class="space-y-1.5">
          <Label for="kd-call-notes">What was said</Label>
          <Textarea
            id="kd-call-notes"
            ref="notesRef"
            v-model="notes"
            rows="4"
            placeholder="The problem, what you advised, what happens next"
          />
        </div>

        <Label class="flex cursor-pointer items-center gap-2 font-normal">
          <Checkbox v-model:checked="callback" /> Call back needed
        </Label>

        <DialogFooter class="items-center gap-2 pt-1">
          <p class="mr-auto text-xs text-muted-foreground max-sm:hidden">{{ modKey }}Enter to log</p>
          <Button type="button" variant="outline" @click="emit('update:open', false)">Cancel</Button>
          <Button type="submit" :disabled="saving">{{ saving ? 'Logging…' : 'Log call' }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { Building2, PhoneCall, PhoneIncoming, PhoneOutgoing, Plus, Search, UserRound } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Checkbox } from '@shared-ui/components/ui/checkbox'
import { Input } from '@shared-ui/components/ui/input'
import { Label } from '@shared-ui/components/ui/label'
import { Textarea } from '@shared-ui/components/ui/textarea'
import { RadioGroup, RadioGroupItem } from '@shared-ui/components/ui/radio-group'
import { Tabs, TabsList, TabsTrigger } from '@shared-ui/components/ui/tabs'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@shared-ui/components/ui/dialog'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import CustomerPicker from '@/features/customer/CustomerPicker.vue'
import { useEmitter } from '@main/composables/useEmitter'
import { useOpenTicket } from '@main/composables/useOpenTicket'
import { invalidateCached } from '@main/composables/useCachedResource'
import { EMITTER_EVENTS, KD_EVENTS } from '@main/constants/emitterEvents'
import { statusClass } from '@main/constants/kdTickets'
import { BADGE } from '@main/constants/kdRmas'
import { useConversationStore } from '@main/stores/conversation'
import api from '@main/api'

const props = defineProps({
  open: Boolean,
  // { uuid, reference_number, contact_name } to log straight on that ticket
  ticket: { type: Object, default: null }
})
const emit = defineEmits(['update:open'])

const emitter = useEmitter()
const openTicket = useOpenTicket()
const conversationStore = useConversationStore()
const modKey = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘' : 'Ctrl+'

const term = ref('')
const results = ref({ contacts: [], customers: [] })
const searching = ref(false)
const caller = ref(null)
const newCaller = ref(null)
const target = ref('new')
const subject = ref('')
const direction = ref('incoming')
const minutes = ref('')
const notes = ref('')
const callback = ref(false)
const saving = ref(false)
const searchRef = ref(null)
const notesRef = ref(null)
const nameRef = ref(null)

const focus = (r) => nextTick(() => (r.value?.$el || r.value)?.focus?.())

function reset () {
  term.value = ''
  results.value = { contacts: [], customers: [] }
  caller.value = null
  newCaller.value = null
  target.value = 'new'
  subject.value = ''
  direction.value = 'incoming'
  minutes.value = ''
  notes.value = ''
  callback.value = false
}
watch(
  () => props.open,
  (open) => {
    if (!open) return
    reset()
    focus(props.ticket ? notesRef : searchRef)
  }
)

const hasResults = computed(() => results.value.contacts.length + results.value.customers.length > 0)
const search = useDebounceFn(async (q) => {
  try {
    const res = await api.findCallers(q)
    if (term.value.trim() === q) results.value = res.data.data
  } catch {
    // keep the last results
  } finally {
    searching.value = false
  }
}, 200)
watch(term, (t) => {
  const q = t.trim()
  if (q.length < 2) return (results.value = { contacts: [], customers: [] })
  searching.value = true
  search(q)
})

function pick (c) {
  caller.value = c
  // Most calls are about something already open: preselect their latest ticket
  target.value = c.open_tickets?.[0]?.uuid || 'new'
  focus(notesRef)
}
const pickFirst = () => {
  const first = results.value.contacts[0] || results.value.customers[0]
  if (first) pick(first)
}
function startNewCaller () {
  const t = term.value.trim()
  const looksLikeNumber = /^[\d\s+()-]+$/.test(t)
  newCaller.value = { name: looksLikeNumber ? '' : t, phone: looksLikeNumber ? t : '', customer: null }
  target.value = 'new'
  // Typed a number: ask for the name next; typed a name: go to the notes
  focus(looksLikeNumber ? nameRef : notesRef)
}
const clearCaller = () => {
  caller.value = null
  newCaller.value = null
  focus(searchRef)
}

const targetOptions = computed(() => [
  ...(caller.value?.open_tickets || []).map((tk) => ({
    value: tk.uuid,
    label: `#${tk.reference_number} ${tk.subject || '(no subject)'}`,
    status: tk.status
  })),
  { value: 'new', label: 'New ticket' }
])

const toast = (description, variant) => emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description, variant })

async function save () {
  if (saving.value) return
  const error = !props.ticket && !caller.value && !newCaller.value
    ? 'Choose who called'
    : newCaller.value && !newCaller.value.name.trim() && !newCaller.value.customer
      ? "Type the caller's name"
      : !notes.value.trim()
        ? 'Add what the call was about'
        : ''
  if (error) return toast(error, 'destructive')

  const onTicket = props.ticket?.uuid || (target.value !== 'new' ? target.value : '')
  const data = {
    conversation_uuid: onTicket,
    direction: direction.value,
    duration_mins: minutes.value === '' ? null : Number(minutes.value),
    notes: notes.value,
    callback_needed: callback.value,
    subject: onTicket ? '' : subject.value
  }
  if (!onTicket) {
    if (caller.value?.kind === 'contact') {
      Object.assign(data, { contact_id: caller.value.id, customer_id: caller.value.customer_id || 0, phone: caller.value.phone })
    } else if (caller.value?.kind === 'customer') {
      Object.assign(data, { customer_id: caller.value.id, phone: caller.value.phone })
    } else if (newCaller.value) {
      Object.assign(data, {
        new_caller_name: newCaller.value.name,
        phone: newCaller.value.phone,
        customer_id: newCaller.value.customer?.id || 0
      })
    }
  } else if (caller.value?.phone) {
    data.phone = caller.value.phone
  }

  saving.value = true
  try {
    const res = await api.logCall(data)
    const { call, new_ticket: isNew } = res.data.data
    toast(isNew ? `Call logged - ticket #${call.ticket_ref} created` : `Call logged on #${call.ticket_ref}`)
    emitter.emit(KD_EVENTS.CALL_LOGGED, { uuid: call.ticket_uuid })
    invalidateCached('')
    conversationStore.fetchSidebarCounts?.({ force: true })
    emit('update:open', false)
    if (!props.ticket) openTicket(call.ticket_uuid)
  } catch (err) {
    toast(handleHTTPError(err).message, 'destructive')
  } finally {
    saving.value = false
  }
}
</script>
