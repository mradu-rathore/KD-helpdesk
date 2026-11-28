<!-- KD: agent home - greeting, your queues at a glance, your tickets, and tickets needing an owner. -->
<template>
  <KdPage :title="greeting" :subtitle="today">
    <template #actions>
      <Button size="sm" variant="outline" class="h-8" @click="openSearch">
        <Search class="mr-1 size-3.5" /> Search
      </Button>
      <Button size="sm" variant="outline" class="h-8" @click="logCall">
        <PhoneCall class="mr-1 size-3.5" /> Log call
      </Button>
      <Button size="sm" class="h-8" @click="newTicket">
        <Plus class="mr-1 size-3.5" /> New ticket
      </Button>
    </template>

    <div class="space-y-5 p-5">
      <!-- Queue cards -->
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <router-link
          v-for="q in queues"
          :key="q.label"
          :to="q.to || ''"
          class="group rounded-xl border border-border p-4 transition-colors hover:border-foreground/30 hover:bg-accent/40"
        >
          <div class="flex items-center justify-between">
            <component :is="q.icon" class="size-4 text-muted-foreground" />
            <ArrowRight class="size-3.5 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
          </div>
          <p class="mt-3 text-2xl font-semibold tabular-nums">{{ q.count }}</p>
          <p class="text-sm text-muted-foreground">{{ q.label }}</p>
        </router-link>
      </div>

      <div class="grid gap-5 xl:grid-cols-2">
        <TicketPanel
          title="Your unsolved tickets"
          :tickets="mine.data.value || []"
          :loading="mine.loading.value"
          :to="viewTo('Your unsolved tickets')"
          empty="Nothing assigned to you. Nice."
          @open="openTicket"
        />
        <TicketPanel
          title="Unassigned tickets"
          :tickets="unassigned.data.value || []"
          :loading="unassigned.loading.value"
          :to="viewTo('Unassigned tickets')"
          empty="Every ticket has an owner."
          @open="openTicket"
        />
      </div>

      <!-- KD: call-backs promised on the phone and not made yet (the daily check) -->
      <section class="overflow-hidden rounded-xl border border-border">
        <div class="flex items-center justify-between border-b border-border px-4 py-3">
          <h2 class="flex items-center gap-2 text-sm font-semibold">
            Call backs waiting
            <span v-if="callbacks.data.value?.length" :class="[BADGE, 'bg-amber-400 text-black']">{{ callbacks.data.value.length }}</span>
          </h2>
          <button type="button" class="text-xs text-muted-foreground hover:text-foreground" @click="logCall">Log a call</button>
        </div>
        <div v-if="callbacks.loading.value" class="space-y-3 p-4">
          <Skeleton v-for="i in 2" :key="i" class="h-9 w-full" />
        </div>
        <ul v-else-if="callbacks.data.value?.length">
          <li
            v-for="c in callbacks.data.value"
            :key="c.id"
            class="flex cursor-pointer items-center gap-3 border-b border-border/60 px-4 py-2.5 transition-colors last:border-0 hover:bg-accent/50"
            tabindex="0"
            @click="openTicket(c.ticket_uuid)"
            @keydown.enter="openTicket(c.ticket_uuid)"
          >
            <PhoneIncoming class="size-4 shrink-0 text-muted-foreground" />
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">
                {{ c.contact_name || 'Caller' }}<span v-if="c.customer_name" class="font-normal text-muted-foreground"> · {{ c.customer_name }}</span>
              </p>
              <p class="truncate text-xs text-muted-foreground">#{{ c.ticket_ref }} · {{ c.notes }}</p>
            </div>
            <span class="shrink-0 text-xs text-muted-foreground" :title="c.agent_name ? `Logged by ${c.agent_name}` : ''">
              {{ formatDistanceToNowStrict(new Date(c.called_at), { addSuffix: true }) }}
            </span>
            <Button size="sm" variant="outline" class="h-7 shrink-0" @click.stop="callbackDone(c)">Done</Button>
          </li>
        </ul>
        <p v-else class="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
          <PhoneCall class="size-4" /> No call backs waiting.
        </p>
      </section>
    </div>
  </KdPage>
</template>

<script setup>
import { computed, defineComponent, h, resolveComponent } from 'vue'
import { format, formatDistanceToNowStrict } from 'date-fns'
import { ArrowRight, AtSign, Clock, Inbox, PhoneCall, PhoneIncoming, Plus, Search, UserRound, UserRoundX } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import KdPage from '@main/components/layout/KdPage.vue'
import { useCachedResource } from '@main/composables/useCachedResource'
import { useOpenTicket } from '@main/composables/useOpenTicket'
import { useCommandPalette } from '@/features/command/useCommandPalette'
import { useEmitter } from '@main/composables/useEmitter'
import { EMITTER_EVENTS, KD_EVENTS } from '@main/constants/emitterEvents'
import { BADGE } from '@main/constants/kdRmas'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { statusClass } from '@main/constants/kdTickets'
import { useUserStore } from '@main/stores/user'
import { useSharedViewStore } from '@main/stores/sharedView'
import { useConversationStore } from '@main/stores/conversation'
import api from '@main/api'

defineOptions({ name: 'HomeView' })

const userStore = useUserStore()
const sharedViewStore = useSharedViewStore()
const conversationStore = useConversationStore()
const emitter = useEmitter()
const palette = useCommandPalette()
const openTicket = useOpenTicket()

const greeting = computed(() => {
  const hr = new Date().getHours()
  const part = hr < 12 ? 'Good morning' : hr < 18 ? 'Good afternoon' : 'Good evening'
  const name = userStore.firstName && userStore.firstName !== 'System' ? `, ${userStore.firstName}` : ''
  return `${part}${name}`
})
const today = format(new Date(), 'EEEE d MMMM')

const view = (name) => (sharedViewStore.sharedViewList || []).find((v) => v.name === name)
const viewTo = (name) => {
  const v = view(name)
  return v ? { name: 'view-inbox', params: { viewID: v.id } } : null
}
const viewCount = (name) => conversationStore.sidebarCounts.views?.[view(name)?.id] ?? 0

const queues = computed(() => [
  { label: 'Your unsolved tickets', icon: UserRound, count: viewCount('Your unsolved tickets'), to: viewTo('Your unsolved tickets') },
  { label: 'Unassigned', icon: UserRoundX, count: viewCount('Unassigned tickets'), to: viewTo('Unassigned tickets') },
  { label: 'Pending', icon: Clock, count: viewCount('Pending tickets'), to: viewTo('Pending tickets') },
  { label: 'Mentions', icon: AtSign, count: conversationStore.sidebarCounts.mentioned ?? 0, to: { name: 'inbox', params: { type: 'mentioned' } } }
])

const viewTickets = (name) => {
  const id = computed(() => view(name)?.id)
  return useCachedResource(
    computed(() => (id.value ? `home:view:${id.value}` : null)),
    async () => {
      const res = await api.getViewConversations(id.value, {
        page: 1,
        page_size: 8,
        order_by: 'conversations.last_message_at',
        order: 'desc'
      })
      return res.data.data.results || []
    }
  )
}
const mine = viewTickets('Your unsolved tickets')
const unassigned = viewTickets('Unassigned tickets')

const openSearch = () => palette.openPalette()
const newTicket = () => emitter.emit(EMITTER_EVENTS.OPEN_CREATE_CONVERSATION)

// KD: phone calls - log one, and the call-backs still to make
const logCall = () => emitter.emit(KD_EVENTS.LOG_CALL)
const callbacks = useCachedResource('home:callbacks', async () => {
  const res = await api.getCalls({ callback: 'open', limit: 20 })
  return res.data.data || []
})
emitter.on(KD_EVENTS.CALL_LOGGED, () => callbacks.reload())
async function callbackDone (call) {
  try {
    await api.markCallbackDone(call.id)
    callbacks.mutate((callbacks.data.value || []).filter((c) => c.id !== call.id))
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: `Call back to ${call.contact_name || 'the caller'} marked done` })
  } catch (err) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { variant: 'destructive', description: handleHTTPError(err).message })
  }
}

const TicketPanel = defineComponent({
  props: { title: String, tickets: Array, loading: Boolean, to: Object, empty: String },
  emits: ['open'],
  setup(props, { emit }) {
    const RouterLink = resolveComponent('RouterLink')
    const name = (c) => [c.contact?.first_name, c.contact?.last_name].filter(Boolean).join(' ')
    return () =>
      h('section', { class: 'overflow-hidden rounded-xl border border-border' }, [
        h('div', { class: 'flex items-center justify-between border-b border-border px-4 py-3' }, [
          h('h2', { class: 'text-sm font-semibold' }, props.title),
          props.to && h(RouterLink, { to: props.to, class: 'text-xs text-muted-foreground hover:text-foreground' }, () => 'View all')
        ]),
        props.loading
          ? h('div', { class: 'space-y-3 p-4' }, [1, 2, 3].map((i) => h(Skeleton, { key: i, class: 'h-9 w-full' })))
          : props.tickets.length
            ? h(
                'ul',
                props.tickets.map((c) =>
                  h(
                    'li',
                    {
                      key: c.uuid,
                      class: 'flex cursor-pointer items-center gap-3 border-b border-border/60 px-4 py-2.5 transition-colors last:border-0 hover:bg-accent/50',
                      tabindex: 0,
                      onClick: () => emit('open', c.uuid),
                      onKeydown: (e) => e.key === 'Enter' && emit('open', c.uuid)
                    },
                    [
                      h('span', { class: ['shrink-0 rounded px-1.5 py-0.5 text-[11px] font-semibold', statusClass(c.status)] }, c.status),
                      h('div', { class: 'min-w-0 flex-1' }, [
                        h('p', { class: 'truncate text-sm font-medium' }, c.subject || '(no subject)'),
                        h('p', { class: 'truncate text-xs text-muted-foreground' }, `#${c.reference_number} · ${name(c)}`)
                      ]),
                      h(
                        'span',
                        { class: 'shrink-0 text-xs text-muted-foreground' },
                        c.last_message_at ? formatDistanceToNowStrict(new Date(c.last_message_at)) : ''
                      )
                    ]
                  )
                )
              )
            : h('p', { class: 'flex items-center gap-2 px-4 py-8 text-sm text-muted-foreground' }, [h(Inbox, { class: 'size-4' }), props.empty])
      ])
  }
})
</script>
