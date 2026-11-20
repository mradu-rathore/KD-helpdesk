<!-- KD: Zendesk-style tab strip for open tickets, shown above the ticket list/page. -->
<template>
  <div
    v-if="store.tabs.length"
    class="flex h-10 shrink-0 items-end gap-1 overflow-x-auto px-1"
    role="tablist"
    aria-label="Open tickets"
  >
    <div
      v-for="tab in store.tabs"
      :key="tab.uuid"
      role="tab"
      :aria-selected="tab.uuid === activeUUID"
      :title="tab.title"
      class="group flex h-9 min-w-[8rem] max-w-[15rem] shrink-0 cursor-pointer select-none items-center gap-2 rounded-t-lg border border-b-0 px-3 text-sm"
      :class="
        tab.uuid === activeUUID
          ? 'border-border bg-background text-foreground'
          : 'border-transparent text-muted-foreground can-hover:hover:bg-accent/50 can-hover:hover:text-foreground'
      "
      @click="router.push(tab.route)"
      @auxclick.middle.prevent="closeTab(tab)"
    >
      <Ticket class="size-3.5 shrink-0" aria-hidden="true" />
      <span class="flex-1 truncate">{{ tab.title }}</span>
      <button
        type="button"
        class="flex size-4 shrink-0 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
        :class="tab.uuid === activeUUID ? 'visible' : 'invisible group-hover:visible'"
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
