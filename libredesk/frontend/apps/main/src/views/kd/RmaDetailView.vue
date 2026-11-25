<!--
  KD: one RMA - where it is in the returns process, the next step, its
  details (saved together) and its history. Replies to the customer and
  internal notes happen on the linked ticket.
-->
<template>
  <KdPage :title="rma?.number || (loading ? 'Loading…' : 'RMA')" :subtitle="subtitle">
    <template #leading>
      <Button variant="ghost" size="icon" class="size-8" aria-label="Back to RMAs" as-child>
        <router-link :to="{ name: 'rmas' }"><ArrowLeft class="size-4" /></router-link>
      </Button>
    </template>
    <template v-if="rma" #actions>
      <span :class="[BADGE, rmaStatus(rma.status).badge]">
        {{ rmaStatus(rma.status).label }}<template v-if="rma.outcome"> · {{ outcomeLabel(rma.outcome) }}</template>
      </span>
      <Button v-if="rma.ticket_uuid" variant="outline" size="sm" class="h-8" @click="openTicket(rma.ticket_uuid)">
        <MessageSquare class="mr-1 size-3.5" /> Ticket #{{ rma.ticket_ref }}
      </Button>
      <template v-if="canWrite">
        <Button
          v-if="['requested', 'approved'].includes(rma.status)"
          variant="outline"
          size="sm"
          class="h-8"
          :disabled="busy"
          @click="openStep('reject')"
        >
          Reject
        </Button>
        <Button
          v-if="['received', 'testing'].includes(rma.status)"
          variant="outline"
          size="sm"
          class="h-8"
          :disabled="busy"
          @click="moveTo('with_supplier')"
        >
          <Truck class="mr-1 size-3.5" /> Send to supplier
        </Button>
        <Button v-if="next" size="sm" class="h-8" :disabled="busy" @click="next.to === 'resolved' ? openStep('resolve') : moveTo(next.to)">
          {{ next.label }}
        </Button>
        <Button v-else size="sm" variant="outline" class="h-8" :disabled="busy" @click="moveTo(reopenTo)">
          <RotateCcw class="mr-1 size-3.5" /> Reopen
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" size="icon" class="size-8" aria-label="Move to another step" :disabled="busy">
              <MoreHorizontal class="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" class="w-48">
            <DropdownMenuLabel class="text-xs text-muted-foreground">Move to</DropdownMenuLabel>
            <DropdownMenuItem
              v-for="s in RMA_STATUSES"
              :key="s.key"
              :disabled="s.key === rma.status && s.key !== 'resolved'"
              @select="s.key === 'resolved' ? openStep('resolve') : s.key === 'rejected' ? openStep('reject') : moveTo(s.key)"
            >
              <i class="mr-2 size-2 rounded-full" :class="s.badge.split(' ')[0]" /> {{ s.label }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </template>
    </template>

    <div v-if="loading" class="space-y-4 p-5">
      <Skeleton class="h-16 w-full" />
      <Skeleton class="h-72 w-full" />
    </div>

    <div v-else-if="!rma" class="flex flex-col items-center gap-2 py-16 text-center">
      <PackageOpen class="size-8 text-muted-foreground" />
      <p class="font-medium">RMA not found</p>
      <Button size="sm" variant="outline" as-child><router-link :to="{ name: 'rmas' }">Back to RMAs</router-link></Button>
    </div>

    <div v-else class="space-y-5 p-5 pb-20">
      <!-- Progress -->
      <div v-if="rma.status === 'rejected'" class="flex items-start gap-3 rounded-lg border border-border bg-muted/40 p-4">
        <Ban class="mt-0.5 size-4 shrink-0 text-muted-foreground" />
        <div class="text-sm">
          <p class="font-medium">Rejected {{ rma.resolved_at ? `on ${formatDay(rma.resolved_at)}` : '' }}</p>
          <p class="text-muted-foreground">{{ rejectReason || 'No reason recorded.' }}</p>
        </div>
      </div>
      <ol v-else class="grid grid-cols-3 gap-y-3 rounded-lg border border-border px-2 py-4 sm:grid-cols-6" aria-label="RMA progress">
        <li v-for="(step, i) in steps" :key="step.key" class="relative flex flex-col items-center px-1 text-center">
          <span v-if="i > 0" class="absolute right-1/2 top-3 hidden h-px w-full sm:block" :class="step.state === 'upcoming' || step.state === 'skipped' ? 'bg-border' : 'bg-foreground/60'" />
          <span
            class="relative z-10 flex size-6 items-center justify-center rounded-full border text-[11px] font-semibold"
            :class="{
              'border-foreground bg-foreground text-background': step.state === 'done',
              'border-foreground bg-background text-foreground ring-4 ring-foreground/10': step.state === 'current',
              'border-dashed border-border bg-background text-muted-foreground': step.state === 'skipped',
              'border-border bg-background text-muted-foreground': step.state === 'upcoming'
            }"
          >
            <Check v-if="step.state === 'done'" class="size-3.5" />
            <template v-else>{{ i + 1 }}</template>
          </span>
          <span class="mt-1.5 text-xs" :class="step.state === 'current' ? 'font-semibold' : step.state === 'upcoming' || step.state === 'skipped' ? 'text-muted-foreground' : ''">
            {{ step.label }}
          </span>
          <span class="text-[11px] text-muted-foreground">{{ step.note }}</span>
        </li>
      </ol>

      <div class="grid gap-5 lg:grid-cols-[1fr_18rem] xl:grid-cols-[1fr_20rem]">
        <!-- Details agents fill in (saved together) -->
        <form class="min-w-0 space-y-5" @submit.prevent="save()">
          <section :class="CARD">
            <h2 :class="H2">Product and fault</h2>
            <div class="grid gap-3 sm:grid-cols-[9rem_1fr_5rem]">
              <div>
                <label for="rma-brand" :class="LABEL">Brand</label>
                <Input id="rma-brand" v-model="form.brand" list="rma-brands" :disabled="!canWrite" @change="suggestSupplier" />
                <datalist id="rma-brands"><option v-for="b in RMA_BRANDS" :key="b" :value="b" /></datalist>
              </div>
              <div>
                <label for="rma-product" :class="LABEL">Product (model or code)</label>
                <Input id="rma-product" v-model="form.product" :disabled="!canWrite" />
              </div>
              <div>
                <label for="rma-qty" :class="LABEL">Qty</label>
                <Input id="rma-qty" v-model.number="form.quantity" type="number" min="1" :disabled="!canWrite" />
              </div>
            </div>
            <div class="mt-3 grid gap-3 sm:grid-cols-2">
              <div>
                <label for="rma-serials" :class="LABEL">
                  Serial numbers
                  <span v-if="serialCount" class="font-normal">· {{ serialCount }}{{ serialCount !== Number(form.quantity) ? ` (qty is ${form.quantity})` : '' }}</span>
                </label>
                <Textarea id="rma-serials" v-model="form.serial_numbers" rows="4" class="font-mono text-xs" placeholder="One per line" :disabled="!canWrite" />
              </div>
              <div>
                <label for="rma-fault" :class="LABEL">Fault</label>
                <Textarea id="rma-fault" v-model="form.fault" rows="4" :disabled="!canWrite" />
              </div>
            </div>
          </section>

          <section :class="CARD">
            <h2 :class="H2">Purchase and warranty</h2>
            <div class="grid gap-3 sm:grid-cols-3">
              <div>
                <label for="rma-invoice" :class="LABEL">Invoice / order no.</label>
                <Input id="rma-invoice" v-model="form.invoice_number" placeholder="Look it up in Intact" :disabled="!canWrite" />
              </div>
              <div>
                <label for="rma-date" :class="LABEL">Purchase date</label>
                <Input id="rma-date" v-model="form.purchase_date" type="date" :max="today" :disabled="!canWrite" />
                <p v-if="boughtAgo" class="mt-1 text-xs text-muted-foreground">Bought {{ boughtAgo }}</p>
              </div>
              <div>
                <label for="rma-warranty" :class="LABEL">Warranty</label>
                <select id="rma-warranty" v-model="form.warranty" :class="[SELECT_CLASS, 'w-full']" :disabled="!canWrite">
                  <option value="">Not checked yet</option>
                  <option v-for="w in RMA_WARRANTY" :key="w.key" :value="w.key">{{ w.label }}</option>
                </select>
              </div>
            </div>
          </section>

          <section :class="CARD">
            <h2 :class="H2">Testing and supplier</h2>
            <div class="grid gap-3 sm:grid-cols-2">
              <div>
                <label for="rma-test" :class="LABEL">Test result</label>
                <select id="rma-test" v-model="form.test_result" :class="[SELECT_CLASS, 'w-full']" :disabled="!canWrite">
                  <option value="">Not tested yet</option>
                  <option v-for="o in RMA_TEST_RESULTS" :key="o.key" :value="o.key">{{ o.label }}</option>
                </select>
              </div>
              <div>
                <label for="rma-tracking" :class="LABEL">Return tracking no.</label>
                <Input id="rma-tracking" v-model="form.return_tracking" placeholder="Courier tracking for the goods" :disabled="!canWrite" />
              </div>
              <div>
                <label for="rma-supplier" :class="LABEL">Supplier</label>
                <Input id="rma-supplier" v-model="form.supplier" placeholder="Who it goes back to" :disabled="!canWrite" />
              </div>
              <div>
                <label for="rma-supplier-ref" :class="LABEL">Supplier's RMA no.</label>
                <Input id="rma-supplier-ref" v-model="form.supplier_rma_ref" placeholder="Their reference, once claimed" :disabled="!canWrite" />
                <p v-if="rma.sent_to_supplier_at" class="mt-1 text-xs text-muted-foreground">Sent {{ formatDay(rma.sent_to_supplier_at) }}</p>
              </div>
            </div>
          </section>

          <section :class="CARD">
            <h2 :class="H2">Resolution</h2>
            <div class="grid gap-3 sm:grid-cols-3">
              <div>
                <p :class="LABEL">Outcome</p>
                <p class="flex h-9 items-center text-sm" :class="rma.outcome ? 'font-medium' : 'text-muted-foreground'">
                  {{ outcomeLabel(rma.outcome) || 'Set when resolving' }}
                </p>
              </div>
              <div>
                <label for="rma-credit" :class="LABEL">Credit note no.</label>
                <Input id="rma-credit" v-model="form.credit_note_number" :disabled="!canWrite" />
              </div>
              <div>
                <label for="rma-replacement" :class="LABEL">Replacement order no.</label>
                <Input id="rma-replacement" v-model="form.replacement_order_number" :disabled="!canWrite" />
              </div>
            </div>
          </section>

          <!-- Save bar -->
          <div
            v-if="dirty && canWrite"
            class="sticky bottom-0 z-10 flex items-center justify-between gap-3 rounded-lg border border-border bg-background/95 px-4 py-2.5 shadow-lg backdrop-blur"
          >
            <span class="text-sm text-muted-foreground">Unsaved changes</span>
            <div class="flex gap-2">
              <Button type="button" variant="ghost" size="sm" @click="resetForm">Discard</Button>
              <Button type="submit" size="sm" :disabled="busy">{{ busy ? 'Saving…' : 'Save changes' }}</Button>
            </div>
          </div>
        </form>

        <!-- Who and when, then history (like the ticket's side panel) -->
        <aside class="min-w-0 space-y-5">
          <section :class="CARD" class="space-y-4">
            <div>
              <p :class="LABEL">Customer</p>
              <CustomerPicker :model-value="form.customer" placeholder="Link a customer" @select="(c) => (form.customer = c)" />
              <p v-if="!form.customer" class="mt-1 flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400">
                <TriangleAlert class="size-3.5 shrink-0" /> No customer matched the sender
              </p>
              <router-link
                v-else-if="rma.customer_id && form.customer?.id === rma.customer_id"
                :to="{ name: 'customer-detail', params: { id: rma.customer_id } }"
                class="mt-1 inline-block text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
              >
                View customer
              </router-link>
            </div>
            <div>
              <p :class="LABEL">Requester</p>
              <p class="truncate text-sm font-medium">{{ rma.contact_name || '—' }}</p>
              <p class="truncate text-xs text-muted-foreground">{{ rma.contact_email }}</p>
            </div>
            <div v-if="rma.ticket_uuid">
              <p :class="LABEL">Owner</p>
              <SelectAgentCombobox
                :model-value="rma.assignee_id ?? undefined"
                placeholder="Unassigned"
                include-none
                current-user-first
                @select="selectOwner"
              />
              <p class="mt-1 text-xs text-muted-foreground">Same as the ticket's assignee</p>
            </div>
            <dl class="grid grid-cols-2 gap-3 border-t border-border pt-4 text-sm">
              <div>
                <dt :class="LABEL">Opened</dt>
                <dd>{{ formatDay(rma.created_at) }}</dd>
              </div>
              <div>
                <dt :class="LABEL">{{ rma.resolved_at ? 'Closed after' : 'Open for' }}</dt>
                <dd>{{ turnaround }}</dd>
              </div>
              <div>
                <dt :class="LABEL">Source</dt>
                <dd>{{ rma.source === 'form' ? 'RMA form' : 'Raised by agent' }}</dd>
              </div>
              <div>
                <dt :class="LABEL">Ticket</dt>
                <dd class="truncate">
                  <button v-if="rma.ticket_uuid" type="button" class="underline-offset-2 hover:underline" @click="openTicket(rma.ticket_uuid)">
                    #{{ rma.ticket_ref }}
                  </button>
                  <span v-if="rma.ticket_status" class="ml-1 text-xs text-muted-foreground">{{ rma.ticket_status }}</span>
                </dd>
              </div>
            </dl>
          </section>

          <section :class="CARD">
            <h2 :class="H2">History</h2>
            <ol class="space-y-3">
              <li v-for="e in history" :key="e.id" class="flex gap-2.5 text-sm">
                <i class="mt-1.5 size-2 shrink-0 rounded-full" :class="e.kind === 'status' ? rmaStatus(e.to_status).badge.split(' ')[0] : 'bg-muted-foreground/40'" />
                <div class="min-w-0">
                  <p>
                    <span class="font-medium">{{ e.actor }}</span>
                    <template v-if="e.kind === 'created'"> created the RMA</template>
                    <template v-else-if="e.kind === 'status'"> moved it to <span class="font-medium">{{ rmaStatus(e.to_status).label }}</span></template>
                    <template v-else> made changes</template>
                  </p>
                  <p v-if="e.note" class="whitespace-pre-wrap break-words text-muted-foreground">{{ e.note }}</p>
                  <p class="text-xs text-muted-foreground" :title="formatFull(e.created_at)">{{ ago(e.created_at) }}</p>
                </div>
              </li>
            </ol>
          </section>
        </aside>
      </div>
    </div>

    <RmaStepDialog v-model:open="stepOpen" :mode="stepMode" :rma="rma" :busy="busy" @submit="onStep" />
  </KdPage>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useNow } from '@vueuse/core'
import { differenceInCalendarDays, format, formatDistanceStrict, formatDistanceToNowStrict } from 'date-fns'
import {
  ArrowLeft,
  Ban,
  Check,
  MessageSquare,
  MoreHorizontal,
  PackageOpen,
  RotateCcw,
  TriangleAlert,
  Truck
} from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Input } from '@shared-ui/components/ui/input'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import { Textarea } from '@shared-ui/components/ui/textarea'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger
} from '@shared-ui/components/ui/dropdown-menu'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import KdPage from '@main/components/layout/KdPage.vue'
import SelectAgentCombobox from '@main/components/combobox/SelectAgentCombobox.vue'
import CustomerPicker from '@/features/customer/CustomerPicker.vue'
import RmaStepDialog from '@/features/rma/RmaStepDialog.vue'
import { useCachedResource, invalidateCached } from '@main/composables/useCachedResource'
import { useOpenTicket } from '@main/composables/useOpenTicket'
import { useEmitter } from '@main/composables/useEmitter'
import { useUserStore } from '@main/stores/user'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents'
import {
  BADGE,
  RMA_BRANDS,
  RMA_NEXT,
  RMA_STATUSES,
  RMA_STEPS,
  RMA_TEST_RESULTS,
  RMA_WARRANTY,
  SELECT_CLASS,
  defaultSupplier,
  outcomeLabel,
  rmaLine,
  rmaStatus
} from '@main/constants/kdRmas'
import api from '@main/api'

defineOptions({ name: 'RmaDetailView' })

const CARD = 'rounded-lg border border-border p-4'
const LABEL = 'mb-1 block text-xs font-medium text-muted-foreground'
const H2 = 'mb-3 text-sm font-semibold'

const route = useRoute()
const userStore = useUserStore()
const emitter = useEmitter()
const openTicket = useOpenTicket()
const now = useNow({ interval: 60_000 })
const today = format(new Date(), 'yyyy-MM-dd')
const canWrite = computed(() => userStore.can('conversations:update_status'))

const id = computed(() => route.params.id)
const { data, loading, mutate } = useCachedResource(
  computed(() => (id.value ? `rma:${id.value}` : null)),
  async () => {
    try {
      const res = await api.getRma(id.value)
      return res.data.data
    } catch (err) {
      if (err?.response?.status === 404) return { rma: null, events: [] }
      throw err
    }
  }
)
const rma = computed(() => data.value?.rma || null)
const history = computed(() => [...(data.value?.events || [])].reverse())

// ---- Editable details (saved together) ----
const FIELDS = [
  'brand', 'product', 'quantity', 'serial_numbers', 'fault', 'invoice_number', 'purchase_date',
  'warranty', 'test_result', 'supplier', 'supplier_rma_ref', 'credit_note_number',
  'replacement_order_number', 'return_tracking'
]
const form = reactive({ customer: null })
const snapshot = (r) => ({
  customer: r?.customer_id ? { id: r.customer_id, name: r.customer_name } : null,
  ...Object.fromEntries(FIELDS.map((f) => [f, r?.[f] ?? '']))
})
const resetForm = () => Object.assign(form, snapshot(rma.value))
watch(rma, resetForm, { immediate: true })

const payload = () => ({
  customer_id: form.customer?.id || 0,
  ...Object.fromEntries(FIELDS.map((f) => [f, f === 'quantity' ? Number(form.quantity) || 1 : String(form[f] ?? '')]))
})
const dirty = computed(() => {
  if (!rma.value) return false
  const a = snapshot(rma.value)
  return (form.customer?.id || 0) !== (a.customer?.id || 0) || FIELDS.some((f) => String(form[f] ?? '') !== String(a[f] ?? ''))
})

// Serials as the server will store them: split on lines, commas or semicolons.
const serialCount = computed(
  () => String(form.serial_numbers || '').split(/[\n,;]/).map((s) => s.trim()).filter(Boolean).length
)

// Fill in the supplier from the brand, if it's still empty.
const suggestSupplier = () => {
  if (!String(form.supplier || '').trim()) form.supplier = defaultSupplier(form.brand)
}

const busy = ref(false)
const toast = (description, variant) => emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description, variant })

const afterChange = (fresh) => {
  invalidateCached('rmas')
  invalidateCached('customer')
  invalidateCached('dashboard')
  return fresh
}

async function save ({ quiet = false } = {}) {
  if (!dirty.value) return true
  busy.value = true
  try {
    await api.updateRma(rma.value.id, payload())
    await reload()
    if (!quiet) toast('Saved')
    return true
  } catch (err) {
    toast(handleHTTPError(err).message, 'destructive')
    return false
  } finally {
    busy.value = false
  }
}

async function reload () {
  const res = await api.getRma(rma.value.id)
  mutate(afterChange(res.data.data))
}

// ---- Steps ----
const next = computed(() => RMA_NEXT[rma.value?.status])
const reopenTo = computed(() => (rma.value?.status === 'rejected' ? 'requested' : 'testing'))

async function moveTo (status, { outcome = '', note = '', fields = {} } = {}) {
  // Save typed changes (and the outcome's reference number) first, so nothing is lost.
  Object.assign(form, Object.fromEntries(Object.entries(fields).filter(([, v]) => v !== '')))
  if (!(await save({ quiet: true }))) return false
  busy.value = true
  try {
    await api.setRmaStatus(rma.value.id, { status, outcome, note })
    await reload()
    toast(`${rma.value.number} moved to ${rmaStatus(status).label}`)
    return true
  } catch (err) {
    toast(handleHTTPError(err).message, 'destructive')
    return false
  } finally {
    busy.value = false
  }
}

const stepOpen = ref(false)
const stepMode = ref('resolve')
const openStep = (mode) => {
  stepMode.value = mode
  stepOpen.value = true
}
async function onStep ({ status, outcome, note, fields }) {
  if (await moveTo(status, { outcome, note, fields })) stepOpen.value = false
}

async function selectOwner (agent) {
  const uuid = rma.value.ticket_uuid
  try {
    if (agent.value === 'none') await api.removeAssignee(uuid, 'user')
    else await api.updateAssignee(uuid, 'user', { assignee_id: parseInt(agent.value) })
    await reload()
    invalidateCached('')
  } catch (err) {
    toast(handleHTTPError(err).message, 'destructive')
  }
}

// ---- Display ----
const formatDay = (d) => (d ? format(new Date(d), 'd MMM yyyy') : '')
const formatFull = (d) => (d ? format(new Date(d), "d MMM yyyy 'at' HH:mm") : '')
const ago = (d) => (d ? formatDistanceToNowStrict(new Date(d), { addSuffix: true }) : '')

const steps = computed(() => {
  const r = rma.value
  if (!r) return []
  const current = RMA_STEPS.findIndex((s) => s.key === r.status)
  return RMA_STEPS.map((s, i) => {
    let state = i < current ? 'done' : i === current ? 'current' : 'upcoming'
    if (s.optional && state === 'done' && !r[s.dateField]) state = 'skipped'
    const date = s.dateField && r[s.dateField]
    const note = state === 'skipped' ? 'Skipped' : date ? format(new Date(date), 'd MMM') : ''
    return { ...s, state, note: s.key === 'resolved' && r.outcome ? outcomeLabel(r.outcome) : note }
  })
})

const rejectReason = computed(() => {
  const ev = [...(data.value?.events || [])].reverse().find((e) => e.kind === 'status' && e.to_status === 'rejected')
  return ev?.note || ''
})

const turnaround = computed(() => {
  const r = rma.value
  if (!r) return ''
  return formatDistanceStrict(new Date(r.created_at), r.resolved_at ? new Date(r.resolved_at) : now.value)
})

const boughtAgo = computed(() => {
  if (!form.purchase_date) return ''
  const days = differenceInCalendarDays(now.value, new Date(form.purchase_date))
  return days < 0 ? '' : formatDistanceToNowStrict(new Date(form.purchase_date), { addSuffix: true })
})

const subtitle = computed(() => {
  const r = rma.value
  if (!r) return ''
  return [rmaLine(r), r.customer_name || r.contact_name].filter(Boolean).join(' · ')
})
</script>
