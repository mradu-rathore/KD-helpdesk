<!--
  KD: log a ticket straight into the helpdesk (no email is sent): subject,
  customer and contact, status, priority, owner, and a description.
-->
<template>
  <form class="flex min-h-0 flex-1 flex-col" novalidate @submit.prevent="submit" @keydown.meta.enter.prevent="submit" @keydown.ctrl.enter.prevent="submit">
    <div class="min-h-0 flex-1 overflow-y-auto">
      <!-- Subject -->
      <div class="px-5 pt-4">
        <input
          ref="subjectRef"
          v-model="form.subject"
          type="text"
          maxlength="300"
          placeholder="Subject"
          aria-label="Subject"
          class="w-full bg-transparent text-lg font-semibold outline-none placeholder:text-muted-foreground/70"
        />
      </div>

      <!-- Properties -->
      <div class="grid grid-cols-1 gap-x-4 gap-y-3 px-5 py-4 sm:grid-cols-2">
        <div class="space-y-1">
          <div class="flex items-center justify-between">
            <label :class="LABEL">Customer</label>
            <button
              v-if="userStore.can('contacts:write')"
              type="button"
              class="inline-flex items-center gap-0.5 text-xs text-muted-foreground hover:text-foreground"
              @click="customerDialogOpen = true"
            >
              <Plus class="size-3" /> New customer
            </button>
          </div>
          <CustomerPicker :model-value="form.customer" placeholder="Choose customer" @select="selectCustomer" />
        </div>

        <div class="space-y-1">
          <label :class="LABEL">Contact <span class="font-normal normal-case">(optional)</span></label>
          <ContactPicker
            :model-value="form.contact"
            :customer-id="form.customer?.id || 0"
            placeholder="Who raised it?"
            @select="selectContact"
          />
        </div>

        <div class="space-y-1">
          <label :class="LABEL">Status</label>
          <SelectComboBox
            v-model="form.status_id"
            :items="conversationStore.statusOptionsNoSnooze"
            placeholder="Select status"
            type="status"
          />
        </div>

        <div class="space-y-1">
          <label :class="LABEL">Priority</label>
          <SelectComboBox
            v-model="form.priority_id"
            :items="conversationStore.priorityOptions"
            placeholder="Select priority"
            type="priority"
          />
        </div>

        <div class="space-y-1">
          <label :class="LABEL">Group</label>
          <SelectTeamCombobox v-model="form.team_id" include-none placeholder="No group" />
        </div>

        <div class="space-y-1">
          <label :class="LABEL">Assignee</label>
          <SelectAgentCombobox v-model="form.agent_id" include-none current-user-first placeholder="Unassigned" />
        </div>
      </div>

      <!-- Description -->
      <div class="border-t border-border px-5 py-4">
        <label for="kd-ticket-body" :class="LABEL">Description</label>
        <Textarea
          id="kd-ticket-body"
          v-model="form.body"
          rows="8"
          placeholder="Describe the issue - what happened, which product or site, anything the customer told you."
          class="mt-1 min-h-40 resize-y"
        />
      </div>
    </div>

    <footer class="flex shrink-0 items-center justify-between gap-3 border-t border-border px-5 py-3">
      <p class="text-xs text-muted-foreground max-sm:hidden">
        Logged in the helpdesk - nothing is emailed. {{ modKey }}Enter to create.
      </p>
      <div class="ml-auto flex gap-2">
        <Button type="button" variant="ghost" @click="emit('close')">Cancel</Button>
        <Button type="submit" :disabled="saving">{{ saving ? 'Creating…' : 'Create ticket' }}</Button>
      </div>
    </footer>

    <CustomerFormDialog v-model:open="customerDialogOpen" @saved="selectCustomer" />
  </form>
</template>

<script setup>
import { nextTick, onMounted, reactive, ref, watch } from 'vue'
import { Plus } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Textarea } from '@shared-ui/components/ui/textarea'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import SelectComboBox from '@main/components/combobox/SelectCombobox.vue'
import SelectAgentCombobox from '@main/components/combobox/SelectAgentCombobox.vue'
import SelectTeamCombobox from '@main/components/combobox/SelectTeamCombobox.vue'
import CustomerPicker from '@/features/customer/CustomerPicker.vue'
import ContactPicker from '@/features/customer/ContactPicker.vue'
import CustomerFormDialog from '@/features/customer/CustomerFormDialog.vue'
import { useNewConversationDraft } from './useNewConversationDraft.js'
import { useConversationStore } from '@main/stores/conversation'
import { useUserStore } from '@main/stores/user'
import { useEmitter } from '@main/composables/useEmitter'
import { useOpenTicket } from '@main/composables/useOpenTicket'
import { invalidateCached } from '@main/composables/useCachedResource'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents'
import api from '@main/api'

const props = defineProps({
  initialContact: { type: Object, default: null }
})
const emit = defineEmits(['close'])

const LABEL = 'block text-xs font-medium text-muted-foreground'
const modKey = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘' : 'Ctrl+'

const conversationStore = useConversationStore()
const userStore = useUserStore()
const emitter = useEmitter()
const openTicket = useOpenTicket()
const draft = useNewConversationDraft('email')

const subjectRef = ref(null)
const saving = ref(false)
const customerDialogOpen = ref(false)

const blank = () => ({
  subject: '',
  body: '',
  customer: null,
  contact: null,
  status_id: '',
  priority_id: '',
  team_id: 'none',
  agent_id: 'none'
})
const form = reactive({ ...blank(), ...(draft.value?.kd ? draft.value : {}) })

// Keep a draft while there's something worth keeping.
watch(
  form,
  (v) => {
    const hasContent = v.subject.trim() || v.body.trim() || v.customer || v.contact
    draft.value = hasContent ? { ...v, kd: true } : null
  },
  { deep: true }
)

const selectCustomer = (customer) => {
  form.customer = customer
  // A contact at another customer no longer fits.
  if (customer && form.contact?.customer_id && form.contact.customer_id !== customer.id) form.contact = null
}

const selectContact = (contact) => {
  form.contact = contact
  if (contact?.customer_id && !form.customer) {
    form.customer = { id: contact.customer_id, name: contact.customer_name }
  }
}

const idOrZero = (v) => (v && v !== 'none' ? Number(v) : 0)

const submit = async () => {
  if (saving.value) return
  const error = !form.subject.trim()
    ? 'Add a subject'
    : !form.customer && !form.contact
      ? 'Choose a customer or a contact'
      : !form.body.trim()
        ? 'Add a description'
        : ''
  if (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { variant: 'destructive', description: error })
    return
  }
  saving.value = true
  try {
    const res = await api.createKdTicket({
      subject: form.subject.trim(),
      body: form.body,
      customer_id: form.customer?.id || 0,
      contact_id: form.contact?.id || 0,
      status_id: idOrZero(form.status_id),
      priority_id: idOrZero(form.priority_id),
      team_id: idOrZero(form.team_id),
      agent_id: idOrZero(form.agent_id)
    })
    const ticket = res.data.data
    draft.value = null
    Object.assign(form, blank())
    invalidateCached('')
    conversationStore.fetchSidebarCounts?.({ force: true })
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: `Ticket #${ticket.reference_number} created` })
    emit('close')
    openTicket(ticket.uuid)
  } catch (err) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { variant: 'destructive', description: handleHTTPError(err).message })
  } finally {
    saving.value = false
  }
}

const focus = () => subjectRef.value?.focus()
defineExpose({ focus })

onMounted(async () => {
  await Promise.all([conversationStore.fetchStatuses(), conversationStore.fetchPriorities()])
  // Default status: Open.
  if (!form.status_id) {
    const open = conversationStore.statusOptionsNoSnooze.find((s) => s.label.toLowerCase() === 'open')
    if (open) form.status_id = open.value
  }
  if (props.initialContact?.id && !form.contact) {
    selectContact({ ...props.initialContact })
  }
  await nextTick()
  focus()
})
</script>
