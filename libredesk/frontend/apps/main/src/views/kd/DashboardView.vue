<!-- KD: support dashboard - totals, trend, per-agent workload, breakdowns. -->
<template>
  <KdPage title="Dashboard" :subtitle="updatedLabel">
    <template #actions>
      <Tabs :model-value="String(days)" @update:model-value="days = Number($event)">
        <TabsList class="h-8">
          <TabsTrigger v-for="d in RANGES" :key="d" :value="String(d)" class="px-2.5 text-xs">{{ d }} days</TabsTrigger>
        </TabsList>
      </Tabs>
      <Button variant="outline" size="sm" class="h-8" :disabled="refreshing" @click="reload">
        <RefreshCw class="mr-1 size-3.5" :class="{ 'animate-spin': refreshing }" /> Refresh
      </Button>
      <Button variant="ghost" size="sm" class="h-8" as-child>
        <router-link :to="{ name: 'overview' }">More reports</router-link>
      </Button>
    </template>

    <div class="space-y-5 p-5" :class="{ 'opacity-60 transition-opacity': refreshing && !loading }">
      <!-- KPI tiles -->
      <div class="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-7">
        <component
          :is="tile.to ? 'router-link' : 'div'"
          v-for="tile in tiles"
          :key="tile.label"
          :to="tile.to"
          class="rounded-xl border border-border p-4 transition-colors"
          :class="tile.to ? 'hover:border-foreground/30 hover:bg-accent/40' : ''"
        >
          <p class="text-xs text-muted-foreground">{{ tile.label }}</p>
          <Skeleton v-if="loading" class="mt-2 h-7 w-12" />
          <p v-else class="mt-1 text-2xl font-semibold tabular-nums">{{ tile.value }}</p>
          <p class="mt-0.5 text-[11px] text-muted-foreground">{{ tile.hint }}</p>
        </component>
      </div>

      <div class="grid gap-5 xl:grid-cols-3">
        <!-- Trend -->
        <section class="rounded-xl border border-border p-4 xl:col-span-2">
          <div class="mb-3 flex items-center justify-between">
            <h2 class="text-sm font-semibold">Tickets created vs solved</h2>
            <div class="flex items-center gap-3 text-xs text-muted-foreground">
              <span class="flex items-center gap-1"><i class="size-2.5 rounded-sm bg-sky-500" /> Created</span>
              <span class="flex items-center gap-1"><i class="size-2.5 rounded-sm bg-emerald-500" /> Solved</span>
            </div>
          </div>
          <Skeleton v-if="loading" class="h-44 w-full" />
          <div v-else class="flex h-44 items-end gap-[2px]" role="img" :aria-label="trendLabel">
            <div
              v-for="d in daily"
              :key="d.day"
              class="group relative flex h-full min-w-0 flex-1 items-end justify-center gap-[1px]"
            >
              <div class="w-1/2 max-w-3 rounded-t-sm bg-sky-500 transition-all duration-500" :style="{ height: barH(d.created) }" />
              <div class="w-1/2 max-w-3 rounded-t-sm bg-emerald-500 transition-all duration-500" :style="{ height: barH(d.solved) }" />
              <div
                class="pointer-events-none absolute bottom-full z-10 mb-1 hidden whitespace-nowrap rounded-md bg-foreground px-2 py-1 text-[11px] text-background shadow group-hover:block"
              >
                {{ dayLabel(d.day) }}: {{ d.created }} created, {{ d.solved }} solved
              </div>
            </div>
          </div>
          <div v-if="!loading && daily.length" class="mt-1 flex justify-between text-[11px] text-muted-foreground">
            <span>{{ dayLabel(daily[0].day) }}</span>
            <span>{{ dayLabel(daily[daily.length - 1].day) }}</span>
          </div>
        </section>

        <!-- By status -->
        <section class="rounded-xl border border-border p-4">
          <h2 class="mb-3 text-sm font-semibold">All tickets by status</h2>
          <BarList :items="data?.by_status || []" :loading="loading" :color="(n) => statusClass(n)" />
        </section>
      </div>

      <div class="grid gap-5 xl:grid-cols-3">
        <!-- Agents -->
        <section class="overflow-hidden rounded-xl border border-border xl:col-span-2">
          <h2 class="border-b border-border px-4 py-3 text-sm font-semibold">Tickets per agent</h2>
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-border text-left text-xs text-muted-foreground">
                <th class="px-4 py-2 font-medium">Agent</th>
                <th class="px-3 py-2 text-right font-medium">Open</th>
                <th class="px-3 py-2 text-right font-medium">Pending</th>
                <th class="px-3 py-2 text-right font-medium">Solved ({{ days }}d)</th>
                <th class="px-4 py-2 text-right font-medium">Avg first reply</th>
              </tr>
            </thead>
            <tbody>
              <template v-if="loading">
                <tr v-for="i in 4" :key="i" class="border-b border-border/60">
                  <td class="px-4 py-2.5" colspan="5"><Skeleton class="h-4 w-full" /></td>
                </tr>
              </template>
              <tr v-for="a in agents" v-else :key="a.id" class="border-b border-border/60 last:border-0">
                <td class="px-4 py-2">
                  <div class="flex items-center gap-2">
                    <Avatar class="size-6">
                      <AvatarImage :src="a.avatar_url || ''" />
                      <AvatarFallback class="text-[10px]">{{ initials(a.name) }}</AvatarFallback>
                    </Avatar>
                    <span class="truncate">{{ a.name }}</span>
                  </div>
                </td>
                <td class="px-3 py-2 text-right tabular-nums" :class="a.open ? 'font-medium' : 'text-muted-foreground'">{{ a.open }}</td>
                <td class="px-3 py-2 text-right tabular-nums text-muted-foreground">{{ a.waiting }}</td>
                <td class="px-3 py-2 text-right tabular-nums">{{ a.solved }}</td>
                <td class="px-4 py-2 text-right tabular-nums text-muted-foreground">{{ duration(a.avg_first_response_secs) }}</td>
              </tr>
            </tbody>
          </table>
        </section>

        <div class="space-y-5">
          <section class="rounded-xl border border-border p-4">
            <h2 class="mb-3 text-sm font-semibold">Open tickets by priority</h2>
            <BarList :items="data?.by_priority || []" :loading="loading" :color="priorityColor" />
          </section>
          <section class="rounded-xl border border-border p-4">
            <div class="mb-3 flex items-center justify-between">
              <h2 class="text-sm font-semibold">Top customers (open tickets)</h2>
              <router-link :to="{ name: 'customers' }" class="text-xs text-muted-foreground hover:text-foreground">All</router-link>
            </div>
            <BarList :items="data?.by_customer || []" :loading="loading" empty="No open tickets linked to customers" />
          </section>
        </div>
      </div>

      <!-- KD: returns (RMAs) -->
      <div class="flex items-center justify-between border-t border-border pt-5">
        <h2 class="text-base font-semibold">Returns (RMAs)</h2>
        <router-link :to="{ name: 'rmas' }" class="text-xs text-muted-foreground hover:text-foreground">All RMAs</router-link>
      </div>
      <div class="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-7">
        <component
          :is="tile.to ? 'router-link' : 'div'"
          v-for="tile in rmaTiles"
          :key="tile.label"
          :to="tile.to"
          class="rounded-xl border border-border p-4 transition-colors"
          :class="tile.to ? 'hover:border-foreground/30 hover:bg-accent/40' : ''"
        >
          <p class="text-xs text-muted-foreground">{{ tile.label }}</p>
          <Skeleton v-if="loading" class="mt-2 h-7 w-12" />
          <p v-else class="mt-1 text-2xl font-semibold tabular-nums">{{ tile.value }}</p>
          <p class="mt-0.5 text-[11px] text-muted-foreground">{{ tile.hint }}</p>
        </component>
      </div>
      <div class="grid gap-5 md:grid-cols-2 xl:grid-cols-4">
        <section class="rounded-xl border border-border p-4">
          <h2 class="mb-3 text-sm font-semibold">Open RMAs by step</h2>
          <BarList :items="rmaByStatus" :loading="loading" :color="rmaStepColor" empty="No open RMAs" />
        </section>
        <section class="rounded-xl border border-border p-4">
          <h2 class="mb-3 text-sm font-semibold">Units returned by product ({{ days }}d)</h2>
          <BarList :items="rmas.by_product || []" :loading="loading" empty="No RMAs in this period" />
        </section>
        <section class="rounded-xl border border-border p-4">
          <h2 class="mb-3 text-sm font-semibold">Units returned by supplier ({{ days }}d)</h2>
          <BarList :items="rmas.by_supplier || []" :loading="loading" empty="No RMAs in this period" />
        </section>
        <section class="rounded-xl border border-border p-4">
          <h2 class="mb-3 text-sm font-semibold">How RMAs closed ({{ days }}d)</h2>
          <BarList :items="rmaByOutcome" :loading="loading" empty="None closed in this period" />
        </section>
      </div>
    </div>
  </KdPage>
</template>

<script setup>
import { computed, defineComponent, h, ref } from 'vue'
import { useStorage, useNow } from '@vueuse/core'
import { format, formatDistanceToNowStrict } from 'date-fns'
import { RefreshCw } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@shared-ui/components/ui/tabs'
import { Avatar, AvatarFallback, AvatarImage } from '@shared-ui/components/ui/avatar'
import KdPage from '@main/components/layout/KdPage.vue'
import { useCachedResource } from '@main/composables/useCachedResource'
import { useSharedViewStore } from '@main/stores/sharedView'
import { initials } from '@/features/customer/initials'
import { statusClass } from '@main/constants/kdTickets'
import { RMA_STATUSES, outcomeLabel, rmaStatus } from '@main/constants/kdRmas'
import api from '@main/api'

defineOptions({ name: 'DashboardView' })

const RANGES = [7, 30, 90]
const days = useStorage('kd_dashboard_days', 30)
const loadedAt = ref(null)
const now = useNow({ interval: 30_000 })
const sharedViewStore = useSharedViewStore()

const { data, loading, refreshing, reload } = useCachedResource(
  computed(() => `dashboard:${days.value}`),
  async () => {
    const res = await api.getKdDashboard({ days: days.value })
    loadedAt.value = new Date()
    return res.data.data
  }
)

const updatedLabel = computed(() => {
  now.value // re-evaluate every 30s
  return loadedAt.value ? `Updated ${formatDistanceToNowStrict(loadedAt.value)} ago` : ''
})

const totals = computed(() => data.value?.totals || {})
const agents = computed(() => data.value?.agents || [])
const daily = computed(() => data.value?.daily || [])

const viewTo = (name) => {
  const v = (sharedViewStore.sharedViewList || []).find((x) => x.name === name)
  return v ? { name: 'view-inbox', params: { viewID: v.id } } : null
}

const duration = (secs) => {
  if (secs == null) return '—'
  const m = Math.round(secs / 60)
  if (m < 60) return `${m}m`
  const hrs = m / 60
  if (hrs < 48) return `${hrs.toFixed(hrs < 10 ? 1 : 0)}h`
  return `${Math.round(hrs / 24)}d`
}

const tiles = computed(() => [
  { label: 'Open tickets', value: totals.value.open ?? 0, hint: 'Not yet solved', to: viewTo('All unsolved tickets') },
  { label: 'Unassigned', value: totals.value.unassigned ?? 0, hint: 'Need an owner', to: viewTo('Unassigned tickets') },
  { label: 'Pending', value: totals.value.waiting ?? 0, hint: 'Waiting on customer', to: viewTo('Pending tickets') },
  { label: 'New', value: totals.value.created ?? 0, hint: `Last ${days.value} days`, to: null },
  { label: 'Solved', value: totals.value.solved ?? 0, hint: `Last ${days.value} days`, to: viewTo('Recently solved tickets') },
  { label: 'Avg first reply', value: duration(totals.value.avg_first_response_secs), hint: `Last ${days.value} days`, to: null },
  { label: 'Avg time to solve', value: duration(totals.value.avg_resolution_secs), hint: `Last ${days.value} days`, to: null }
])

const maxDaily = computed(() => Math.max(1, ...daily.value.map((d) => Math.max(d.created, d.solved))))
const barH = (n) => `${n ? Math.max(4, (n / maxDaily.value) * 100) : 0}%`
const dayLabel = (d) => format(new Date(d), 'd MMM')
const trendLabel = computed(
  () =>
    `${daily.value.reduce((s, d) => s + d.created, 0)} created and ${daily.value.reduce((s, d) => s + d.solved, 0)} solved in the last ${days.value} days`
)

// KD: returns (RMAs)
const rmas = computed(() => data.value?.rmas || {})
const rmaTotals = computed(() => rmas.value.totals || {})
const rmaList = (status) => ({ name: 'rmas', query: { status } })
const rmaTiles = computed(() => {
  const t = rmaTotals.value
  return [
    { label: 'Open RMAs', value: t.open ?? 0, hint: 'Not yet closed', to: rmaList('open') },
    { label: 'Needs checking', value: t.requested ?? 0, hint: 'Invoice and warranty', to: rmaList('requested') },
    { label: 'Awaiting goods', value: t.approved ?? 0, hint: 'Approved, not received', to: rmaList('approved') },
    { label: 'With supplier', value: t.with_supplier ?? 0, hint: 'Claimed from manufacturer', to: rmaList('with_supplier') },
    { label: 'Resolved', value: t.resolved ?? 0, hint: `Last ${days.value} days`, to: rmaList('resolved') },
    { label: 'Avg turnaround', value: duration(t.avg_turnaround_secs), hint: `Request to close, ${days.value}d`, to: null },
    {
      label: 'No fault found',
      value: t.tested ? `${Math.round((t.no_fault_found / t.tested) * 100)}%` : '—',
      hint: t.tested ? `${t.no_fault_found} of ${t.tested} tested` : 'Nothing tested yet',
      to: null
    }
  ]
})
const rmaByStatus = computed(() =>
  (rmas.value.by_status || []).map((s) => ({ name: rmaStatus(s.name).label, count: s.count }))
)
const rmaStepColor = (label) => RMA_STATUSES.find((s) => s.label === label)?.badge || 'bg-zinc-400'
const rmaByOutcome = computed(() =>
  (rmas.value.by_outcome || []).map((o) => ({
    name: o.name === 'rejected' ? 'Rejected' : outcomeLabel(o.name) || o.name,
    count: o.count
  }))
)

const PRIORITY_COLORS = { urgent: 'bg-red-600', high: 'bg-orange-500', medium: 'bg-amber-400', low: 'bg-sky-500' }
const priorityColor = (name) => PRIORITY_COLORS[String(name).toLowerCase()] || 'bg-zinc-400'

// Horizontal bar list used for the breakdowns
const BarList = defineComponent({
  props: {
    items: { type: Array, default: () => [] },
    loading: Boolean,
    color: { type: Function, default: null },
    empty: { type: String, default: 'Nothing here yet' }
  },
  setup(props) {
    return () => {
      if (props.loading) return h('div', { class: 'space-y-2' }, [1, 2, 3, 4].map((i) => h(Skeleton, { key: i, class: 'h-5 w-full' })))
      if (!props.items.length) return h('p', { class: 'py-4 text-sm text-muted-foreground' }, props.empty)
      const max = Math.max(1, ...props.items.map((i) => i.count))
      return h(
        'ul',
        { class: 'space-y-2' },
        props.items.map((i) =>
          h('li', { key: i.name, class: 'text-sm' }, [
            h('div', { class: 'mb-0.5 flex justify-between gap-2' }, [
              h('span', { class: 'truncate' }, i.name),
              h('span', { class: 'tabular-nums text-muted-foreground' }, i.count)
            ]),
            h('div', { class: 'h-1.5 overflow-hidden rounded-full bg-muted' }, [
              h('div', {
                class: ['h-full rounded-full transition-all duration-500', props.color ? props.color(i.name).split(' ')[0] : 'bg-foreground/70'],
                style: { width: `${(i.count / max) * 100}%` }
              })
            ])
          ])
        )
      )
    }
  }
})
</script>
