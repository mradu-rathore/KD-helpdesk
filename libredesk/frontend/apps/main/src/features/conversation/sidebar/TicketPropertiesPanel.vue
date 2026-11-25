<!--
  KD: Zendesk-style left panel on the ticket page - requester, assignee, group,
  priority, tags and the ticket's fields (custom attributes), stacked as labelled
  fields like Zendesk's ticket properties.
-->
<template>
  <div v-if="conv" class="flex h-full min-h-0 flex-col overflow-y-auto overscroll-contain">
    <div class="space-y-4 border-b border-border p-4">
      <div>
        <p :class="LABEL">Requester</p>
        <p class="truncate text-sm font-medium">{{ requesterName }}</p>
        <p v-if="conv.contact?.email" class="truncate text-xs text-muted-foreground">{{ conv.contact.email }}</p>
      </div>

      <div>
        <p :class="LABEL">Customer</p>
        <CustomerPicker
          :model-value="customer"
          :allow-none="customerSource === 'ticket'"
          none-label="Clear (use the requester's customer)"
          @select="selectCustomer"
        />
        <p v-if="customerSource === 'requester'" class="mt-1 text-xs text-muted-foreground">
          From the requester
        </p>
        <router-link
          v-if="customer"
          :to="{ name: 'customer-detail', params: { id: customer.id } }"
          class="mt-1 inline-block text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
        >
          {{ customer.open_tickets }} open · {{ customer.tickets }} total tickets
        </router-link>
      </div>

      <div>
        <p :class="LABEL">{{ t('globals.terms.assignee') }}</p>
        <SelectAgentCombobox
          v-model="conv.assigned_user_id"
          include-none
          current-user-first
          @select="selectAgent"
        />
        <button
          v-if="String(conv.assigned_user_id) !== String(userStore.userID)"
          type="button"
          class="mt-1 text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
          @click="takeIt"
        >
          take it
        </button>
      </div>

      <div>
        <p :class="LABEL">Group</p>
        <SelectTeamCombobox :model-value="conv.assigned_team_id" include-none @select="selectTeam" />
      </div>

      <div>
        <p :class="LABEL">{{ t('globals.terms.priority', 1) }}</p>
        <SelectComboBox
          v-model="conv.priority_id"
          :items="conversationStore.priorityOptions"
          :placeholder="t('placeholders.selectPriority')"
          type="priority"
          @select="selectPriority"
        />
      </div>

      <div>
        <p :class="LABEL">Tags</p>
        <SelectTagCombobox
          multiple
          compact
          keep-open-on-select
          :model-value="conv.tags || []"
          @update:modelValue="onTagsChange"
        />
      </div>
    </div>

    <!-- KD: the ticket's RMAs -->
    <div class="border-b border-border p-4">
      <div class="mb-2 flex items-center justify-between">
        <p :class="LABEL" class="mb-0">RMAs</p>
        <button
          v-if="userStore.can('conversations:update_status')"
          type="button"
          class="inline-flex items-center gap-0.5 text-xs text-muted-foreground hover:text-foreground"
          @click="rmaFormOpen = true"
        >
          <Plus class="size-3" /> Create RMA
        </button>
      </div>
      <ul v-if="rmas.length" class="space-y-1.5">
        <li v-for="r in rmas" :key="r.id">
          <router-link
            :to="{ name: 'rma-detail', params: { id: r.id } }"
            class="block rounded-md border border-border px-2.5 py-1.5 transition-colors hover:bg-accent/50"
          >
            <span class="flex items-center justify-between gap-2">
              <span class="text-sm font-medium tabular-nums">{{ r.number }}</span>
              <span :class="[BADGE, rmaStatus(r.status).badge]">
                {{ rmaStatus(r.status).label }}
              </span>
            </span>
            <span class="block truncate text-xs text-muted-foreground">{{ rmaLine(r) }}</span>
          </router-link>
        </li>
      </ul>
      <p v-else class="text-xs text-muted-foreground">No returns on this ticket.</p>
    </div>
    <RmaFormDialog
      v-model:open="rmaFormOpen"
      :ticket="{ uuid: conv.uuid, reference_number: conv.reference_number }"
      @saved="loadRmas(conv.uuid)"
    />

    <!-- Ticket info + KD ticket fields (custom attributes) -->
    <div class="p-4">
      <p :class="LABEL" class="mb-2">Ticket fields</p>
      <ConversationInfo />
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useConversationStore } from '@/stores/conversation'
import { useUserStore } from '@/stores/user'
import { TAG_ACTION } from '@/constants/conversation'
import ConversationInfo from './ConversationInfo.vue'
import SelectComboBox from '@main/components/combobox/SelectCombobox.vue'
import SelectAgentCombobox from '@main/components/combobox/SelectAgentCombobox.vue'
import SelectTeamCombobox from '@main/components/combobox/SelectTeamCombobox.vue'
import SelectTagCombobox from '@main/components/combobox/SelectTagCombobox.vue'
import CustomerPicker from '@/features/customer/CustomerPicker.vue'
import RmaFormDialog from '@/features/rma/RmaFormDialog.vue'
import { Plus } from 'lucide-vue-next'
import { BADGE, rmaLine, rmaStatus } from '@main/constants/kdRmas'
import { invalidateCached } from '@main/composables/useCachedResource'
import { useEmitter } from '@main/composables/useEmitter'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import api from '@main/api'

const LABEL = 'mb-1 text-xs font-medium text-muted-foreground'

const { t } = useI18n()
const conversationStore = useConversationStore()
const userStore = useUserStore()
const conv = computed(() => conversationStore.current)
const requesterName = computed(() => {
  const c = conv.value?.contact
  return c ? [c.first_name, c.last_name].filter(Boolean).join(' ') : ''
})

// KD: the ticket's customer (company) - set on the ticket, or the requester's
const emitter = useEmitter()
const customer = ref(null)
const customerSource = ref(null)
const customerCache = new Map() // uuid -> { customer, source }, so tab switches are instant

watch(
  () => conv.value?.uuid,
  async (uuid) => {
    if (!uuid) return
    const cached = customerCache.get(uuid)
    customer.value = cached?.customer ?? null
    customerSource.value = cached?.source ?? null
    try {
      const res = await api.getTicketCustomer(uuid)
      customerCache.set(uuid, res.data.data)
      if (conv.value?.uuid !== uuid) return
      customer.value = res.data.data.customer
      customerSource.value = res.data.data.source
    } catch {
      // no access or offline: leave the field empty
    }
  },
  { immediate: true }
)

// KD: RMAs on this ticket (cached per ticket, refreshed on every open)
const rmas = ref([])
const rmaFormOpen = ref(false)
const rmaCache = new Map()
async function loadRmas (uuid) {
  try {
    const res = await api.getTicketRmas(uuid)
    rmaCache.set(uuid, res.data.data)
    if (conv.value?.uuid === uuid) rmas.value = res.data.data
  } catch {
    // no access or offline: leave the list as it is
  }
}
watch(
  () => conv.value?.uuid,
  (uuid) => {
    if (!uuid) return
    rmas.value = rmaCache.get(uuid) || []
    loadRmas(uuid)
  },
  { immediate: true }
)

async function selectCustomer (next) {
  const uuid = conv.value.uuid
  const previous = { customer: customer.value, source: customerSource.value }
  customer.value = next // optimistic
  customerSource.value = next ? 'ticket' : null
  try {
    const res = await api.setTicketCustomer(uuid, next?.id)
    customerCache.set(uuid, res.data.data)
    invalidateCached('customer')
    if (conv.value?.uuid !== uuid) return
    customer.value = res.data.data.customer
    customerSource.value = res.data.data.source
  } catch (err) {
    customer.value = previous.customer
    customerSource.value = previous.source
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(err).message
    })
  }
}

function selectAgent (agent) {
  if (agent.value === 'none') return conversationStore.removeAssignee('user')
  conv.value.assigned_user_id = agent.value
  conversationStore.updateAssignee('user', { assignee_id: parseInt(agent.value) })
}

// Zendesk's "take it": assign the ticket to yourself in one click
function takeIt () {
  selectAgent({ value: String(userStore.userID) })
}

function selectTeam (team) {
  if (team.value === 'none') return conversationStore.removeAssignee('team')
  conversationStore.updateAssignee('team', { assignee_id: parseInt(team.value) })
}

function selectPriority (priority) {
  conv.value.priority = priority.label
  conv.value.priority_id = priority.value
  conversationStore.updatePriority(priority.label)
}

function onTagsChange (newTags) {
  const current = conv.value?.tags || []
  if (newTags.length === current.length && newTags.every((tag) => current.includes(tag))) return
  conversationStore.updateConversationTags(conv.value.uuid, TAG_ACTION.SET, newTags)
}
</script>
