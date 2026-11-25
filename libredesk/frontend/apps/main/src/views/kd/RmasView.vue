<!-- KD: RMAs list - status tabs, search and the returns queue. -->
<template>
  <KdPage title="RMAs" :subtitle="subtitle">
    <template #actions>
      <SearchInput
        v-model="term"
        class="w-60"
        placeholder="Search RMA, serial, invoice"
        clear-label="Clear search"
      />
      <select v-model="brand" :class="[SELECT_CLASS, 'h-8 w-28']" aria-label="Brand">
        <option value="">All brands</option>
        <option v-for="b in RMA_BRANDS" :key="b" :value="b">{{ b }}</option>
      </select>
      <Button v-if="canWrite" size="sm" class="h-8" @click="showNew = true">
        <Plus class="mr-1 size-4" /> New RMA
      </Button>
    </template>

    <!-- Status tabs (same look as the customer page's tabs) -->
    <div class="border-b border-border px-3">
      <div class="flex gap-1 overflow-x-auto [scrollbar-width:none]" role="tablist">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          role="tab"
          :aria-selected="status === tab.key"
          class="shrink-0 px-2.5 py-2 text-sm transition-colors"
          :class="status === tab.key
            ? 'font-medium text-foreground shadow-[inset_0_-2px_0_0_currentColor]'
            : 'text-muted-foreground hover:text-foreground'"
          @click="status = tab.key"
        >
          {{ tab.label }}
          <span class="ml-1 rounded bg-muted px-1.5 text-xs tabular-nums">{{ tab.count }}</span>
        </button>
      </div>
    </div>

    <table class="w-full text-sm">
      <thead class="sticky top-0 z-10 bg-background">
        <tr class="border-b border-border text-left text-xs text-muted-foreground">
          <th class="w-28 px-5 py-2 font-medium">RMA</th>
          <th class="w-36 px-3 py-2 font-medium">Status</th>
          <th class="min-w-[14rem] px-3 py-2 font-medium">Product</th>
          <th class="w-52 px-3 py-2 font-medium">Customer</th>
          <th class="w-44 px-3 py-2 font-medium max-lg:hidden">Serial</th>
          <th class="w-36 px-3 py-2 font-medium max-md:hidden">Owner</th>
          <th class="w-28 px-5 py-2 text-right font-medium">Opened</th>
        </tr>
      </thead>
      <tbody v-if="loading">
        <tr v-for="i in 8" :key="i" class="border-b border-border/60">
          <td class="px-5 py-3"><Skeleton class="h-4 w-16" /></td>
          <td class="px-3 py-3"><Skeleton class="h-4 w-16" /></td>
          <td class="px-3 py-3"><Skeleton class="h-4 w-48" /></td>
          <td class="px-3 py-3"><Skeleton class="h-4 w-32" /></td>
          <td class="px-3 py-3 max-lg:hidden"><Skeleton class="h-4 w-24" /></td>
          <td class="px-3 py-3 max-md:hidden"><Skeleton class="h-4 w-20" /></td>
          <td class="px-5 py-3"><Skeleton class="ml-auto h-4 w-14" /></td>
        </tr>
      </tbody>
      <tbody v-else>
        <tr
          v-for="r in rmas"
          :key="r.id"
          class="cursor-pointer border-b border-border/60 transition-colors hover:bg-accent/50"
          tabindex="0"
          @click="openRma(r)"
          @keydown.enter="openRma(r)"
        >
          <td class="px-5 py-2.5 font-medium tabular-nums">{{ r.number }}</td>
          <td class="whitespace-nowrap px-3 py-2.5">
            <span :class="[BADGE, rmaStatus(r.status).badge]">{{ rmaStatus(r.status).label }}</span>
            <span v-if="r.outcome" class="ml-1.5 text-xs text-muted-foreground">{{ outcomeLabel(r.outcome) }}</span>
          </td>
          <td class="max-w-0 truncate px-3 py-2.5 font-medium" :title="rmaLine(r)">{{ rmaLine(r) }}</td>
          <td class="max-w-0 truncate px-3 py-2.5" :title="r.contact_email">
            <span v-if="r.customer_name" class="inline-flex items-center gap-1.5">
              <Building2 class="size-3.5 shrink-0 text-muted-foreground" />{{ r.customer_name }}
            </span>
            <span v-else class="inline-flex items-center gap-1.5 text-amber-600 dark:text-amber-400">
              <TriangleAlert class="size-3.5 shrink-0" /> No customer
            </span>
          </td>
          <td class="whitespace-nowrap px-3 py-2.5 text-muted-foreground max-lg:hidden">
            <template v-if="serialList(r.serial_numbers).length">
              <span class="font-mono text-xs">{{ serialList(r.serial_numbers)[0] }}</span>
              <span v-if="serialList(r.serial_numbers).length > 1" class="ml-1 text-xs">+{{ serialList(r.serial_numbers).length - 1 }}</span>
            </template>
            <template v-else>—</template>
          </td>
          <td class="max-w-0 truncate px-3 py-2.5 max-md:hidden" :class="r.assignee_name ? '' : 'text-muted-foreground'">
            {{ r.assignee_name || 'Unassigned' }}
          </td>
          <td class="whitespace-nowrap px-5 py-2.5 text-right text-muted-foreground" :title="formatFullTimestamp(r.created_at)">
            {{ shortDate(r.created_at) }}
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="!loading && !rmas.length" class="flex flex-col items-center gap-2 px-6 py-16 text-center">
      <PackageOpen class="size-8 text-muted-foreground" />
      <p class="font-medium">{{ emptyTitle }}</p>
      <p class="max-w-md text-sm text-muted-foreground">
        RMAs arrive from the RMA Google Form, or raise one from a ticket or with New RMA.
      </p>
    </div>

    <div v-if="totalPages > 1" class="flex items-center justify-end gap-2 border-t border-border px-5 py-2 text-sm">
      <span class="text-muted-foreground">Page {{ page }} of {{ totalPages }}</span>
      <Button size="sm" variant="outline" class="h-7" :disabled="page <= 1" @click="page--">Previous</Button>
      <Button size="sm" variant="outline" class="h-7" :disabled="page >= totalPages" @click="page++">Next</Button>
    </div>

    <RmaFormDialog v-model:open="showNew" @saved="openRma" />
  </KdPage>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { refDebounced, useStorage } from '@vueuse/core'
import { Building2, PackageOpen, Plus, TriangleAlert } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import SearchInput from '@shared-ui/components/SearchInput.vue'
import { formatFullTimestamp } from '@shared-ui/utils/datetime.js'
import KdPage from '@main/components/layout/KdPage.vue'
import RmaFormDialog from '@/features/rma/RmaFormDialog.vue'
import { useCachedResource } from '@main/composables/useCachedResource'
import { useUserStore } from '@main/stores/user'
import {
  BADGE,
  OPEN_STATUSES,
  RMA_BRANDS,
  RMA_STATUSES,
  SELECT_CLASS,
  outcomeLabel,
  rmaLine,
  rmaStatus,
  serialList,
  shortDate
} from '@main/constants/kdRmas'
import api from '@main/api'

defineOptions({ name: 'RmasView' })

const PAGE_SIZE = 50
const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const canWrite = computed(() => userStore.can('conversations:update_status'))

const term = ref('')
const debouncedTerm = refDebounced(term, 200)
const status = useStorage('kd_rmas_status', 'open')
const brand = useStorage('kd_rmas_brand', '')
const page = ref(1)
const showNew = ref(false)

watch([debouncedTerm, status, brand], () => (page.value = 1))

// Links like /rmas?status=approved (from the dashboard) pick the tab.
watch(
  () => route.query.status,
  (s) => {
    if (s && (s === 'open' || s === 'all' || RMA_STATUSES.some((x) => x.key === s))) status.value = s
  },
  { immediate: true }
)

const key = computed(
  () => `rmas?q=${debouncedTerm.value.trim()}&status=${status.value}&brand=${brand.value}&page=${page.value}`
)
const { data, loading } = useCachedResource(key, async () => {
  const res = await api.getRmas({
    q: debouncedTerm.value.trim(),
    status: status.value === 'all' ? '' : status.value,
    brand: brand.value,
    page: page.value,
    page_size: PAGE_SIZE
  })
  return res.data.data
})

const rmas = computed(() => data.value?.results || [])
const totalPages = computed(() => data.value?.total_pages || 1)
const counts = computed(() => data.value?.counts || {})

const tabs = computed(() => {
  const c = counts.value
  const open = OPEN_STATUSES.reduce((n, s) => n + (c[s] || 0), 0)
  const all = RMA_STATUSES.reduce((n, s) => n + (c[s.key] || 0), 0)
  return [
    { key: 'open', label: 'Open', count: open },
    ...RMA_STATUSES.map((s) => ({ key: s.key, label: s.label, count: c[s.key] || 0 })),
    { key: 'all', label: 'All', count: all }
  ]
})

const subtitle = computed(() => {
  const total = data.value?.total
  if (total === undefined) return ''
  return `${total} ${total === 1 ? 'RMA' : 'RMAs'}`
})
const emptyTitle = computed(() =>
  debouncedTerm.value.trim() || brand.value ? 'No RMAs match your search' : 'No RMAs here'
)

const openRma = (r) => r?.id && router.push({ name: 'rma-detail', params: { id: r.id } })
</script>
