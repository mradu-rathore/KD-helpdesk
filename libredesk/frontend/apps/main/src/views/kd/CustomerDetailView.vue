<!-- KD: one customer (company) - details, its tickets and its contacts. -->
<template>
  <KdPage :title="customer?.name || (loading ? 'Loading…' : 'Customer')" :subtitle="subtitle">
    <template #leading>
      <Button variant="ghost" size="icon" class="size-8" aria-label="Back to customers" as-child>
        <router-link :to="{ name: 'customers' }"><ArrowLeft class="size-4" /></router-link>
      </Button>
      <span v-if="customer" :class="initialsClass(customer.name, 'size-9 text-xs')">
        {{ initials(customer.name) }}
      </span>
    </template>
    <template #actions>
      <template v-if="customer && userStore.can('contacts:write')">
        <Button size="sm" variant="outline" class="h-8" @click="showForm = true">
          <Pencil class="mr-1 size-3.5" /> Edit
        </Button>
      </template>
      <Button
        v-if="customer && userStore.can('contacts:delete')"
        size="sm"
        variant="ghost"
        class="h-8 text-muted-foreground hover:text-destructive"
        aria-label="Delete customer"
        @click="confirmDelete = true"
      >
        <Trash2 class="size-4" />
      </Button>
    </template>

    <div v-if="loading" class="space-y-4 p-5">
      <Skeleton class="h-20 w-full" />
      <Skeleton class="h-64 w-full" />
    </div>

    <div v-else-if="customer" class="grid gap-5 p-5 lg:grid-cols-[18rem_1fr]">
      <!-- Details -->
      <aside class="space-y-4">
        <div class="grid grid-cols-2 gap-2">
          <div class="rounded-lg border border-border p-3">
            <p class="text-xs text-muted-foreground">Open tickets</p>
            <p class="text-2xl font-semibold tabular-nums">{{ customer.open_tickets }}</p>
          </div>
          <div class="rounded-lg border border-border p-3">
            <p class="text-xs text-muted-foreground">All tickets</p>
            <p class="text-2xl font-semibold tabular-nums">{{ customer.tickets }}</p>
          </div>
        </div>
        <dl class="space-y-3 rounded-lg border border-border p-4 text-sm">
          <div>
            <dt class="text-xs text-muted-foreground">Website / domain</dt>
            <dd>
              <a
                v-if="customer.domain"
                :href="`https://${customer.domain}`"
                target="_blank"
                rel="noopener noreferrer"
                class="underline-offset-2 hover:underline"
                >{{ customer.domain }}</a
              >
              <span v-else class="text-muted-foreground">—</span>
            </dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">Phone</dt>
            <dd>{{ customer.phone || '—' }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">Notes</dt>
            <dd class="whitespace-pre-wrap">{{ customer.notes || '—' }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">Customer since</dt>
            <dd>{{ formatDate(customer.created_at) }}</dd>
          </div>
        </dl>
      </aside>

      <!-- Tickets / contacts -->
      <section class="min-w-0">
        <div class="mb-3 flex items-center gap-1 border-b border-border">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            type="button"
            class="-mb-px border-b-2 px-3 py-2 text-sm transition-colors"
            :class="
              activeTab === tab.key
                ? 'border-foreground font-medium text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            "
            @click="activeTab = tab.key"
          >
            {{ tab.label }}
            <span class="ml-1 rounded bg-muted px-1.5 text-xs tabular-nums">{{ tab.count }}</span>
          </button>
          <div class="ml-auto pb-1">
            <Popover v-if="activeTab === 'contacts' && userStore.can('contacts:write')" v-model:open="addOpen">
              <PopoverTrigger as-child>
                <Button size="sm" variant="outline" class="h-7"><Plus class="mr-1 size-3.5" /> Add contact</Button>
              </PopoverTrigger>
              <PopoverContent class="w-80 p-0" align="end">
                <div class="border-b border-border p-2">
                  <Input v-model="contactTerm" class="h-8" placeholder="Search contacts by name or email" />
                </div>
                <div class="max-h-64 overflow-y-auto p-1">
                  <p v-if="contactTerm.trim().length < 2" class="px-2 py-3 text-sm text-muted-foreground">
                    Type at least 2 characters
                  </p>
                  <p v-else-if="!contactResults.length" class="px-2 py-3 text-sm text-muted-foreground">
                    No contacts found
                  </p>
                  <button
                    v-for="c in contactResults"
                    :key="c.id"
                    type="button"
                    class="flex w-full flex-col items-start rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent"
                    @click="linkContact(c)"
                  >
                    <span class="font-medium">{{ fullName(c) }}</span>
                    <span class="text-xs text-muted-foreground">{{ c.email }}</span>
                  </button>
                </div>
              </PopoverContent>
            </Popover>
          </div>
        </div>

        <!-- Tickets -->
        <div v-if="activeTab === 'tickets'" class="overflow-hidden rounded-lg border border-border">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-border bg-muted/40 text-left text-xs text-muted-foreground">
                <th class="w-24 px-3 py-2 font-medium">Status</th>
                <th class="w-16 px-2 py-2 font-medium">ID</th>
                <th class="px-2 py-2 font-medium">Subject</th>
                <th class="px-2 py-2 font-medium">Requester</th>
                <th class="px-3 py-2 text-right font-medium">Updated</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="tk in tickets"
                :key="tk.uuid"
                class="cursor-pointer border-b border-border/60 last:border-0 transition-colors hover:bg-accent/50"
                @click="openTicket(tk.uuid)"
              >
                <td class="px-3 py-2">
                  <span :class="['inline-block rounded px-1.5 py-0.5 text-[11px] font-semibold', statusClass(tk.status)]">
                    {{ tk.status }}
                  </span>
                </td>
                <td class="px-2 py-2 text-muted-foreground">#{{ tk.reference_number }}</td>
                <td class="max-w-0 truncate px-2 py-2 font-medium">{{ tk.subject || '(no subject)' }}</td>
                <td class="truncate px-2 py-2 text-muted-foreground">{{ tk.contact_name }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right text-muted-foreground">{{ formatDate(tk.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
          <p v-if="!tickets.length" class="px-4 py-10 text-center text-sm text-muted-foreground">
            No tickets yet. Tickets from linked contacts, or set to this customer, show up here.
          </p>
        </div>

        <!-- Contacts -->
        <div v-else class="overflow-hidden rounded-lg border border-border">
          <div
            v-for="c in contacts"
            :key="c.id"
            class="group flex items-center gap-3 border-b border-border/60 px-3 py-2.5 last:border-0"
          >
            <router-link
              :to="{ name: 'contact-detail', params: { id: c.id } }"
              class="flex min-w-0 flex-1 items-center gap-3"
            >
              <span class="inline-flex size-8 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-medium">
                {{ initials(fullName(c)) }}
              </span>
              <span class="min-w-0">
                <span class="block truncate text-sm font-medium group-hover:underline">{{ fullName(c) }}</span>
                <span class="block truncate text-xs text-muted-foreground">
                  {{ [c.email, c.phone].filter(Boolean).join(' · ') }}
                </span>
              </span>
            </router-link>
            <Button
              v-if="userStore.can('contacts:write')"
              size="sm"
              variant="ghost"
              class="h-7 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
              @click="unlinkContact(c)"
            >
              Remove
            </Button>
          </div>
          <p v-if="!contacts.length" class="px-4 py-10 text-center text-sm text-muted-foreground">
            No contacts linked yet.
          </p>
        </div>
      </section>
    </div>

    <div v-else class="py-16 text-center text-sm text-muted-foreground">Customer not found.</div>

    <CustomerFormDialog v-model:open="showForm" :customer="customer" @saved="reload" />

    <AlertDialog v-model:open="confirmDelete">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {{ customer?.name }}?</AlertDialogTitle>
          <AlertDialogDescription>
            The customer is removed. Its contacts and tickets are kept, just no longer linked to it.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" @click="deleteCustomer">Delete</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </KdPage>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDebounceFn } from '@vueuse/core'
import { format } from 'date-fns'
import { ArrowLeft, Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Input } from '@shared-ui/components/ui/input'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import { Popover, PopoverContent, PopoverTrigger } from '@shared-ui/components/ui/popover'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@shared-ui/components/ui/alert-dialog'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import KdPage from '@main/components/layout/KdPage.vue'
import CustomerFormDialog from '@/features/customer/CustomerFormDialog.vue'
import { initials, initialsClass } from '@/features/customer/initials'
import { statusClass } from '@main/constants/kdTickets'
import { useCachedResource, invalidateCached } from '@main/composables/useCachedResource'
import { useOpenTicket } from '@main/composables/useOpenTicket'
import { useEmitter } from '@main/composables/useEmitter'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents'
import { useUserStore } from '@main/stores/user'
import api from '@main/api'

defineOptions({ name: 'CustomerDetailView' })

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const emitter = useEmitter()
const openTicket = useOpenTicket()

const id = computed(() => (route.name === 'customer-detail' ? route.params.id : null))
const { data, loading, reload } = useCachedResource(
  computed(() => (id.value ? `customer:${id.value}` : null)),
  async () => (await api.getCustomer(id.value)).data.data
)

const customer = computed(() => data.value?.customer || null)
const tickets = computed(() => data.value?.tickets || [])
const contacts = computed(() => data.value?.contacts || [])
const subtitle = computed(() =>
  customer.value ? [customer.value.domain, customer.value.phone].filter(Boolean).join(' · ') : ''
)

const activeTab = ref('tickets')
const tabs = computed(() => [
  { key: 'tickets', label: 'Tickets', count: tickets.value.length },
  { key: 'contacts', label: 'Contacts', count: contacts.value.length }
])

const showForm = ref(false)
const confirmDelete = ref(false)

const fullName = (c) => [c.first_name, c.last_name].filter(Boolean).join(' ')
const formatDate = (d) => (d ? format(new Date(d), 'd MMM yyyy') : '—')

const toastError = (err) =>
  emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { variant: 'destructive', description: handleHTTPError(err).message })

// Add contact
const addOpen = ref(false)
const contactTerm = ref('')
const contactResults = ref([])
const searchContacts = useDebounceFn(async () => {
  const q = contactTerm.value.trim()
  if (q.length < 2) return (contactResults.value = [])
  try {
    const res = await api.searchContacts({ query: q, limit: 20 })
    const linked = new Set(contacts.value.map((c) => c.id))
    contactResults.value = (res.data.data || []).filter((c) => !linked.has(c.id))
  } catch {
    contactResults.value = []
  }
}, 200)
watch(contactTerm, searchContacts)
watch(addOpen, (open) => {
  if (open) {
    contactTerm.value = ''
    contactResults.value = []
  }
})

const afterChange = () => {
  invalidateCached('customers')
  reload()
}

const linkContact = async (c) => {
  addOpen.value = false
  try {
    await api.linkCustomerContact(customer.value.id, c.id)
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: `${fullName(c)} linked` })
    afterChange()
  } catch (err) {
    toastError(err)
  }
}

const unlinkContact = async (c) => {
  try {
    await api.unlinkCustomerContact(customer.value.id, c.id)
    afterChange()
  } catch (err) {
    toastError(err)
  }
}

const deleteCustomer = async () => {
  try {
    await api.deleteCustomer(customer.value.id)
    invalidateCached('customer')
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: 'Customer deleted' })
    router.push({ name: 'customers' })
  } catch (err) {
    toastError(err)
  }
}
</script>
