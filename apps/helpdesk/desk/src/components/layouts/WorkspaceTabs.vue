<!-- KD: Zendesk-style tab strip for open tickets, contacts and customers. -->
<template>
  <div
    v-if="store.tabs.length"
    class="flex h-9 shrink-0 items-end gap-1 overflow-x-auto border-b bg-surface-gray-1 px-2 pt-1"
    role="tablist"
    :aria-label="__('Open tabs')"
  >
    <div
      v-for="tab in store.tabs"
      :key="tab.kind + ':' + tab.id"
      role="tab"
      :aria-selected="isActive(tab)"
      :title="tab.title"
      class="group flex h-8 max-w-[14rem] min-w-[7rem] shrink-0 cursor-pointer select-none items-center gap-1.5 rounded-t-md border border-b-0 px-2.5 text-sm"
      :class="
        isActive(tab)
          ? 'relative -mb-px border-outline-gray-2 bg-surface-white text-ink-gray-9'
          : 'border-transparent text-ink-gray-6 hover:bg-surface-gray-2 hover:text-ink-gray-8'
      "
      @click="router.push(tabRoute(tab))"
      @auxclick.middle.prevent="closeTab(tab)"
    >
      <component :is="ICONS[tab.kind]" class="h-3.5 w-3.5 shrink-0" />
      <span class="flex-1 truncate">{{ tab.title }}</span>
      <button
        class="flex h-4 w-4 shrink-0 items-center justify-center rounded text-ink-gray-5 hover:bg-surface-gray-3 hover:text-ink-gray-8"
        :class="isActive(tab) ? 'visible' : 'invisible group-hover:visible'"
        :aria-label="__('Close tab')"
        @click.stop="closeTab(tab)"
      >
        <LucideX class="h-3 w-3" />
      </button>
    </div>

    <button
      class="mb-1 ms-1 flex h-6 w-6 shrink-0 items-center justify-center self-center rounded text-ink-gray-6 hover:bg-surface-gray-3 hover:text-ink-gray-9"
      :title="__('New ticket')"
      :aria-label="__('New ticket')"
      @click="router.push({ name: 'TicketAgentNew' })"
    >
      <LucidePlus class="h-4 w-4" />
    </button>
  </div>
</template>

<script setup lang="ts">
import {
  tabFromRoute,
  tabRoute,
  useWorkspaceTabsStore,
  type WorkspaceTab,
} from "@/stores/workspaceTabs";
import { watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import LucideBuilding2 from "~icons/lucide/building-2";
import LucidePlus from "~icons/lucide/plus";
import LucideTicket from "~icons/lucide/ticket";
import LucideUser from "~icons/lucide/user";
import LucideX from "~icons/lucide/x";

const ICONS = {
  ticket: LucideTicket,
  contact: LucideUser,
  customer: LucideBuilding2,
};

const route = useRoute();
const router = useRouter();
const store = useWorkspaceTabsStore();

// Opening a ticket/contact/customer page (from a list, search, link, URL...)
// adds its tab; revisiting an existing one just activates it.
watch(
  () => tabFromRoute(route),
  (current) => current && store.open(current.kind, current.id),
  { immediate: true }
);

function isActive(tab: WorkspaceTab) {
  const current = tabFromRoute(route);
  return !!current && current.kind === tab.kind && current.id === tab.id;
}

function closeTab(tab: WorkspaceTab) {
  const wasActive = isActive(tab);
  const next = store.close(tab.kind, tab.id);
  if (!wasActive) return;
  router.push(next ? tabRoute(next) : { name: "TicketsAgent" });
}
</script>
