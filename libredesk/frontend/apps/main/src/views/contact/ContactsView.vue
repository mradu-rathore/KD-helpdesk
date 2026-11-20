<!-- KD: contacts as a table, with each contact's customer and tickets. -->
<template>
  <KdPage title="Contacts" :subtitle="subtitle">
    <template #actions>
      <SearchInput
        v-model="term"
        class="w-64"
        placeholder="Search name, email, phone, customer"
        clear-label="Clear search"
      />
      <Button v-if="userStore.can('contacts:write')" size="sm" class="h-8" @click="showCreate = true">
        <Plus class="mr-1 size-4" /> New contact
      </Button>
    </template>

    <table class="w-full table-fixed text-sm">
      <thead class="sticky top-0 z-10 bg-background">
        <tr class="border-b border-border text-left text-xs text-muted-foreground">
          <th class="w-[24%] px-5 py-2 font-medium">Name</th>
          <th class="w-[28%] px-3 py-2 font-medium">Email</th>
          <th class="w-[14%] px-3 py-2 font-medium">Phone</th>
          <th class="w-[20%] px-3 py-2 font-medium">Customer</th>
          <th class="w-[7%] px-3 py-2 text-right font-medium">Open</th>
          <th class="w-[7%] px-5 py-2 text-right font-medium">Tickets</th>
        </tr>
      </thead>
      <tbody v-if="loading">
        <tr v-for="i in 10" :key="i" class="border-b border-border/60">
          <td class="px-5 py-3"><Skeleton class="h-4 w-36" /></td>
          <td class="px-3 py-3"><Skeleton class="h-4 w-44" /></td>
          <td class="px-3 py-3"><Skeleton class="h-4 w-24" /></td>
          <td class="px-3 py-3"><Skeleton class="h-4 w-28" /></td>
          <td class="px-3 py-3"><Skeleton class="ml-auto h-4 w-5" /></td>
          <td class="px-5 py-3"><Skeleton class="ml-auto h-4 w-5" /></td>
        </tr>
      </tbody>
      <tbody v-else>
        <tr
          v-for="c in contacts"
          :key="c.id"
          class="cursor-pointer border-b border-border/60 transition-colors hover:bg-accent/50"
          tabindex="0"
          @click="openContact(c)"
          @keydown.enter="openContact(c)"
        >
          <td class="px-5 py-2.5">
            <div class="flex items-center gap-2.5">
              <Avatar class="size-7">
                <AvatarImage :src="c.avatar_url || ''" />
                <AvatarFallback class="text-[10px]">{{ initials(fullName(c)) }}</AvatarFallback>
              </Avatar>
              <span class="min-w-0 truncate font-medium">{{ fullName(c) }}</span>
            </div>
          </td>
          <td class="truncate px-3 py-2.5 text-muted-foreground" :title="c.email">{{ c.email || '—' }}</td>
          <td class="px-3 py-2.5 text-muted-foreground">{{ c.phone || '—' }}</td>
          <td class="px-3 py-2.5">
            <router-link
              v-if="c.customer_id"
              :to="{ name: 'customer-detail', params: { id: c.customer_id } }"
              class="inline-flex max-w-full items-center gap-1.5 truncate hover:underline"
              @click.stop
            >
              <Building2 class="size-3.5 shrink-0 text-muted-foreground" />
              <span class="truncate">{{ c.customer_name }}</span>
            </router-link>
            <span v-else class="text-muted-foreground">—</span>
          </td>
          <td class="px-3 py-2.5 text-right tabular-nums" :class="c.open_tickets ? 'font-medium' : 'text-muted-foreground'">
            {{ c.open_tickets }}
          </td>
          <td class="px-5 py-2.5 text-right tabular-nums text-muted-foreground">{{ c.tickets }}</td>
        </tr>
      </tbody>
    </table>

    <p v-if="!loading && !contacts.length" class="py-16 text-center text-sm text-muted-foreground">
      {{ term ? 'No contacts match your search' : 'No contacts yet' }}
    </p>

    <div
      v-if="totalPages > 1"
      class="flex items-center justify-end gap-2 border-t border-border px-5 py-2 text-sm"
    >
      <span class="text-muted-foreground">Page {{ page }} of {{ totalPages }}</span>
      <Button size="sm" variant="outline" class="h-7" :disabled="page <= 1" @click="page--">Previous</Button>
      <Button size="sm" variant="outline" class="h-7" :disabled="page >= totalPages" @click="page++">Next</Button>
    </div>

    <CreateContactDialog v-model:open="showCreate" @created="onCreated" />
  </KdPage>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { refDebounced } from '@vueuse/core'
import { Building2, Plus } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import { Avatar, AvatarFallback, AvatarImage } from '@shared-ui/components/ui/avatar'
import SearchInput from '@shared-ui/components/SearchInput.vue'
import KdPage from '@main/components/layout/KdPage.vue'
import CreateContactDialog from '@/features/contact/CreateContactDialog.vue'
import { useCachedResource, invalidateCached } from '@main/composables/useCachedResource'
import { initials } from '@/features/customer/initials'
import { useUserStore } from '@main/stores/user'
import api from '@main/api'

defineOptions({ name: 'ContactsView' })

const PAGE_SIZE = 50
const router = useRouter()
const userStore = useUserStore()
const term = ref('')
const debouncedTerm = refDebounced(term, 200)
const page = ref(1)
const showCreate = ref(false)

watch(debouncedTerm, () => (page.value = 1))

const { data, loading } = useCachedResource(
  computed(() => `contacts?q=${debouncedTerm.value.trim()}&page=${page.value}`),
  async () =>
    (await api.getKdContacts({ q: debouncedTerm.value.trim(), page: page.value, page_size: PAGE_SIZE })).data.data
)

const contacts = computed(() => data.value?.results || [])
const totalPages = computed(() => data.value?.total_pages || 1)
const subtitle = computed(() => {
  const total = data.value?.total
  return total === undefined ? '' : `${total} ${total === 1 ? 'contact' : 'contacts'}`
})

const fullName = (c) => [c.first_name, c.last_name].filter(Boolean).join(' ')
const openContact = (c) => router.push({ name: 'contact-detail', params: { id: c.id } })
const onCreated = (c) => {
  invalidateCached('contacts')
  if (c?.id) openContact(c)
}
</script>
