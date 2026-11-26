<!--
  KD: Zendesk-style strip of open tickets above the ticket list/page. Its own
  card like the panels below it; tabs look like the sidebar's items (active =
  accent background).
-->
<template>
  <div
    v-if="store.tabs.length"
    class="mb-1 flex h-10 shrink-0 items-center gap-1 overflow-x-auto rounded-xl border border-border bg-background px-1.5 [scrollbar-width:none]"
    role="tablist"
    aria-label="Open tickets"
  >
    <div
      v-for="tab in store.tabs"
      :key="tab.uuid"
      role="tab"
      :aria-selected="tab.uuid === activeUUID"
      :title="tab.title"
      tabindex="0"
      class="group flex h-7 min-w-[8rem] max-w-[15rem] shrink-0 cursor-pointer select-none items-center gap-2 rounded-md pl-2.5 pr-1.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
      :class="
        tab.uuid === activeUUID
          ? 'bg-accent font-medium text-foreground'
          : 'text-muted-foreground can-hover:hover:bg-accent/50 can-hover:hover:text-foreground'
      "
      @click="router.push(tab.route)"
      @keydown.enter="router.push(tab.route)"
      @auxclick.middle.prevent="closeTab(tab)"
    >
      <Ticket class="size-3.5 shrink-0 opacity-70" aria-hidden="true" />
      <span class="flex-1 truncate">{{ tab.title }}</span>
      <button
        type="button"
        class="flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground hover:bg-background hover:text-foreground"
        :class="tab.uuid === activeUUID ? 'visible' : 'invisible group-hover:visible group-focus-visible:visible'"
        aria-label="Close tab"
        @click.stop="closeTab(tab)"
      >
        <X class="size-3" />
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Ticket, X } from 'lucide-vue-next'
import { useConversationStore } from '@/stores/conversation'
import { useKdTicketTabsStore } from '@/stores/kdTicketTabs'

const route = useRoute()
const router = useRouter()
const store = useKdTicketTabsStore()
const conversationStore = useConversationStore()

const isTicketRoute = (r) => String(r.name).endsWith('-conversation')
const activeUUID = computed(() => (isTicketRoute(route) ? route.params.uuid : null))

// Opening a ticket (from a view, search, notification...) adds or refreshes its tab
watch(
  () => (isTicketRoute(route) ? route.fullPath : null),
  () => {
    if (!isTicketRoute(route)) return
    store.open(route.params.uuid, { name: route.name, params: { ...route.params } })
  },
  { immediate: true }
)

// Title the tab "#ref Subject" once the ticket has loaded
watch(
  () => conversationStore.current,
  (c) => {
    if (!c?.uuid) return
    const subject = c.subject || c.contact?.first_name || ''
    store.rename(c.uuid, `#${c.reference_number} ${subject}`.trim())
  },
  { deep: false, immediate: true }
)

function closeTab (tab) {
  const wasActive = tab.uuid === activeUUID.value
  const next = store.close(tab.uuid)
  if (!wasActive) return
  if (next) return router.push(next.route)
  // Back to the view/list the ticket was opened from
  const listName = String(tab.route.name).replace(/-conversation$/, '')
  const { uuid, ...params } = tab.route.params
  router.push({ name: listName, params })
}
</script>
