<!-- KD: Customers (companies) list. -->
<template>
  <KdPage title="Customers" :subtitle="subtitle">
    <template #actions>
      <SearchInput
        v-model="term"
        class="w-56"
        placeholder="Search name or domain"
        clear-label="Clear search"
      />
      <Button v-if="userStore.can('contacts:write')" size="sm" class="h-8" @click="openNew">
        <Plus class="mr-1 size-4" /> New customer
      </Button>
    </template>

    <table class="w-full text-sm">
      <thead class="sticky top-0 z-10 bg-background">
        <tr class="border-b border-border text-left text-xs text-muted-foreground">
          <th class="px-5 py-2 font-medium">Name</th>
          <th class="px-3 py-2 font-medium">Domain</th>
          <th class="px-3 py-2 font-medium">Phone</th>
          <th class="px-3 py-2 text-right font-medium">Contacts</th>
          <th class="px-3 py-2 text-right font-medium">Open tickets</th>
          <th class="px-5 py-2 text-right font-medium">All tickets</th>
        </tr>
      </thead>
      <tbody v-if="loading">
        <tr v-for="i in 8" :key="i" class="border-b border-border/60">
          <td class="px-5 py-3"><Skeleton class="h-4 w-40" /></td>
          <td class="px-3 py-3"><Skeleton class="h-4 w-28" /></td>
          <td class="px-3 py-3"><Skeleton class="h-4 w-24" /></td>
          <td class="px-3 py-3"><Skeleton class="ml-auto h-4 w-6" /></td>
          <td class="px-3 py-3"><Skeleton class="ml-auto h-4 w-6" /></td>
          <td class="px-5 py-3"><Skeleton class="ml-auto h-4 w-6" /></td>
        </tr>
      </tbody>
      <tbody v-else>
        <tr
          v-for="c in customers"
          :key="c.id"
          class="cursor-pointer border-b border-border/60 transition-colors hover:bg-accent/50"
          tabindex="0"
          @click="openCustomer(c)"
          @keydown.enter="openCustomer(c)"
        >
          <td class="px-5 py-2.5">
            <div class="flex items-center gap-2.5">
              <span :class="initialsClass(c.name)">{{ initials(c.name) }}</span>
              <span class="truncate font-medium">{{ c.name }}</span>
            </div>
          </td>
          <td class="px-3 py-2.5 text-muted-foreground">{{ c.domain || '—' }}</td>
          <td class="px-3 py-2.5 text-muted-foreground">{{ c.phone || '—' }}</td>
          <td class="px-3 py-2.5 text-right tabular-nums">{{ c.contacts }}</td>
          <td class="px-3 py-2.5 text-right tabular-nums">
            <span :class="c.open_tickets ? 'font-medium text-foreground' : 'text-muted-foreground'">
              {{ c.open_tickets }}
            </span>
          </td>
          <td class="px-5 py-2.5 text-right tabular-nums text-muted-foreground">{{ c.tickets }}</td>
        </tr>
      </tbody>
    </table>

    <div v-if="!loading && !customers.length" class="flex flex-col items-center gap-2 py-16 text-center">
      <Building2 class="size-8 text-muted-foreground" />
      <p class="font-medium">{{ term ? 'No customers match your search' : 'No customers yet' }}</p>
      <p class="text-sm text-muted-foreground">
        Customers are companies. Link contacts to one and their tickets appear on it.
      </p>
      <Button v-if="!term && userStore.can('contacts:write')" size="sm" class="mt-2" @click="openNew">
        New customer
      </Button>
    </div>

    <div
      v-if="totalPages > 1"
      class="flex items-center justify-end gap-2 border-t border-border px-5 py-2 text-sm"
    >
      <span class="text-muted-foreground">Page {{ page }} of {{ totalPages }}</span>
      <Button size="sm" variant="outline" class="h-7" :disabled="page <= 1" @click="page--">Previous</Button>
      <Button size="sm" variant="outline" class="h-7" :disabled="page >= totalPages" @click="page++">Next</Button>
    </div>

    <CustomerFormDialog v-model:open="showForm" @saved="onSaved" />
  </KdPage>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { refDebounced } from '@vueuse/core'
import { Building2, Plus } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import SearchInput from '@shared-ui/components/SearchInput.vue'
import KdPage from '@main/components/layout/KdPage.vue'
import CustomerFormDialog from '@/features/customer/CustomerFormDialog.vue'
import { useCachedResource } from '@main/composables/useCachedResource'
import { initials, initialsClass } from '@/features/customer/initials'
import { useUserStore } from '@main/stores/user'
import api from '@main/api'

defineOptions({ name: 'CustomersView' })

const PAGE_SIZE = 50
const router = useRouter()
const userStore = useUserStore()
const term = ref('')
const debouncedTerm = refDebounced(term, 200)
const page = ref(1)
const showForm = ref(false)

watch(debouncedTerm, () => (page.value = 1))

const key = computed(() => `customers?q=${debouncedTerm.value.trim()}&page=${page.value}`)
const { data, loading, reload } = useCachedResource(key, async () => {
  const res = await api.getCustomers({
    q: debouncedTerm.value.trim(),
    page: page.value,
    page_size: PAGE_SIZE
  })
  return res.data.data
})

const customers = computed(() => data.value?.results || [])
const totalPages = computed(() => data.value?.total_pages || 1)
const subtitle = computed(() => {
  const total = data.value?.total
  return total === undefined ? '' : `${total} ${total === 1 ? 'customer' : 'customers'}`
})

const openNew = () => (showForm.value = true)
const openCustomer = (c) => router.push({ name: 'customer-detail', params: { id: c.id } })
const onSaved = (c) => {
  reload()
  if (c?.id) openCustomer(c)
}
</script>
