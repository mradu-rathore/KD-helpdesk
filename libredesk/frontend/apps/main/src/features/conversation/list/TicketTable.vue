<!--
  KD: Zendesk-style ticket list. A full-width table (status badge, ID, subject,
  requester, requested, priority, updated, assignee) with bulk selection,
  sortable columns and, outside "My inbox", rows grouped by assignee.
-->
<template>
  <div class="flex h-full min-h-0 flex-col">
    <!-- Header: view title + count -->
    <div class="flex shrink-0 items-center gap-3 border-b border-border px-4 pb-3 pt-4">
      <!-- KD: desktop collapses the main sidebar; phones open the menu -->
      <KdSidebarToggle v-if="!isMobile" />
      <SidebarTrigger class="cursor-pointer md:hidden" />
      <div class="min-w-0 flex-1">
        <!-- KD: the title switches between ticket views (they're no longer in the sidebar) -->
        <DropdownMenu>
          <DropdownMenuTrigger
            class="-ml-1.5 inline-flex max-w-full items-center gap-1 rounded-md px-1.5 py-0.5 text-left transition-colors hover:bg-accent"
          >
            <h1 class="truncate text-xl font-semibold" :title="title">{{ title }}</h1>
            <ChevronDown class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" class="w-64">
            <DropdownMenuItem
              v-for="v in sharedViewStore.sharedViewList"
              :key="'s' + v.id"
              :class="{ 'bg-accent': isCurrentView(v.id) }"
              @click="router.push({ name: 'view-inbox', params: { viewID: v.id } })"
            >
              <span class="flex-1 truncate">{{ v.name }}</span>
              <span class="text-xs tabular-nums text-muted-foreground">{{ store.sidebarCounts.views?.[v.id] ?? '' }}</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              :class="{ 'bg-accent': route.params.type === 'mentioned' }"
              @click="router.push({ name: 'inbox', params: { type: 'mentioned' } })"
            >
              <span class="flex-1 truncate">{{ t('globals.terms.mention', 2) }}</span>
              <span class="text-xs tabular-nums text-muted-foreground">{{ store.sidebarCounts.mentioned || '' }}</span>
            </DropdownMenuItem>
            <template v-if="viewStore.views.length">
              <DropdownMenuSeparator />
              <DropdownMenuItem
                v-for="v in viewStore.views"
                :key="'u' + v.id"
                :class="{ 'bg-accent': isCurrentView(v.id) }"
                @click="router.push({ name: 'view-inbox', params: { viewID: v.id } })"
              >
                <span class="flex-1 truncate">{{ v.name }}</span>
                <span class="text-xs tabular-nums text-muted-foreground">{{ store.sidebarCounts.views?.[v.id] ?? '' }}</span>
              </DropdownMenuItem>
            </template>
          </DropdownMenuContent>
        </DropdownMenu>
        <p class="text-xs text-muted-foreground tabular-nums">
          {{ store.conversations.total }} {{ t('globals.terms.conversation', store.conversations.total).toLowerCase() }}
        </p>
      </div>
    </div>

    <!-- Bulk actions replace the filter bar while tickets are selected -->
    <ConversationBulkActionToolbar v-if="hasSelection && canBulkAct" />
    <div v-else class="flex shrink-0 items-center gap-2 border-b border-border px-4 py-2">
      <DropdownMenu v-if="!route.params.viewID">
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="outline" size="sm" class="gap-1.5 text-xs">
            <span class="text-muted-foreground">{{ t('globals.terms.status', 1) }}:</span>
            <span>{{ store.getListStatus }}</span>
            <ChevronDown class="size-3.5 opacity-50" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem
            v-for="status in store.statusOptions"
            :key="status.value"
            @click="store.setListStatus(status.label)"
          >
            {{ status.label }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>

    <!-- Table -->
    <div class="min-h-0 flex-1 overflow-auto">
      <table class="w-full min-w-[56rem] border-separate border-spacing-0 text-sm">
        <thead class="sticky top-0 z-10 bg-background">
          <tr class="text-left text-xs font-medium text-muted-foreground">
            <th class="w-10 border-b border-border py-2 pl-4">
              <Checkbox
                v-if="canBulkAct && rows.length"
                :checked="allSelected"
                :aria-label="t('conversation.bulkActions.selectConversation')"
                @update:checked="toggleAll"
              />
            </th>
            <th class="w-28 border-b border-border py-2 pr-3">{{ t('globals.terms.status', 1) }}</th>
            <th class="w-20 border-b border-border py-2 pr-3">ID</th>
            <th class="min-w-[16rem] border-b border-border py-2 pr-3">{{ t('globals.terms.subject', 1) }}</th>
            <th class="w-44 border-b border-border py-2 pr-3">Requester</th>
            <SortHeader label="Requested" :fields="['started_last', 'started_first']" class="w-32" />
            <SortHeader label="Priority" :fields="['priority_first']" class="w-24" />
            <SortHeader label="Updated" :fields="['newest', 'oldest']" class="w-32" />
            <th v-if="!grouped" class="w-40 border-b border-border py-2 pr-4">{{ t('globals.terms.assignee') }}</th>
          </tr>
        </thead>

        <tbody v-if="!store.conversations.loading && !hasErrored">
          <template v-for="group in groups" :key="group.key">
            <tr v-if="grouped">
              <td colspan="8" class="border-b border-border bg-muted/40 px-4 py-1.5 text-xs text-muted-foreground">
                {{ t('globals.terms.assignee') }}: <span class="font-medium text-foreground">{{ group.label }}</span>
              </td>
            </tr>
            <tr
              v-for="c in group.rows"
              :key="c.uuid"
              class="cursor-pointer transition-colors can-hover:hover:bg-accent/50"
              :class="{ 'bg-primary/5': store.isSelected(c.uuid), 'font-semibold': c.unread_message_count > 0 }"
              @click="open(c)"
            >
              <td class="border-b border-border py-2.5 pl-4" @click.stop>
                <Checkbox
                  v-if="canBulkAct"
                  :checked="store.isSelected(c.uuid)"
                  :aria-label="t('conversation.bulkActions.selectConversation')"
                  @click="(e) => store.toggleSelect(c.uuid, e.shiftKey)"
                />
              </td>
              <td class="border-b border-border py-2.5 pr-3">
                <span
                  class="inline-block rounded px-1.5 py-0.5 text-[11px] font-semibold leading-4"
                  :class="statusClass(c.status)"
                >{{ c.status }}</span>
              </td>
              <td class="border-b border-border py-2.5 pr-3 tabular-nums text-muted-foreground">#{{ c.reference_number }}</td>
              <td class="max-w-0 truncate border-b border-border py-2.5 pr-3" :title="subjectOf(c)">
                {{ subjectOf(c) }}
              </td>
              <td class="max-w-0 truncate border-b border-border py-2.5 pr-3">{{ store.getContactFullName(c.uuid) }}</td>
              <td class="border-b border-border py-2.5 pr-3 text-muted-foreground" :title="formatFullTimestamp(c.created_at)">
                {{ shortDate(c.created_at) }}
              </td>
              <td class="border-b border-border py-2.5 pr-3">{{ c.priority || '' }}</td>
              <td class="border-b border-border py-2.5 pr-3 text-muted-foreground" :title="formatFullTimestamp(updatedAt(c))">
                {{ shortDate(updatedAt(c)) }}
              </td>
              <td v-if="!grouped" class="max-w-0 truncate border-b border-border py-2.5 pr-4">{{ assigneeLabel(c) }}</td>
            </tr>
          </template>
        </tbody>
      </table>

      <div v-if="store.conversations.loading" class="space-y-2 p-4">
        <Skeleton v-for="i in 8" :key="i" class="h-8 w-full" />
      </div>
      <EmptyList
        v-if="showEmpty"
        class="px-4 py-12"
        :title="t('conversation.noConversationsFound')"
        :message="t('conversation.tryAdjustingFilters')"
        :icon="Inbox"
      />
      <EmptyList
        v-if="hasErrored"
        class="px-4 py-12"
        :title="t('conversation.couldNotFetch')"
        :message="store.conversations.errorMessage"
        :icon="MessageCircleWarning"
      />

      <div v-if="store.conversations.hasMore" class="flex justify-center p-4">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          :disabled="store.conversations.fetching"
          @click="store.fetchNextConversations"
        >
          <Loader2 v-if="store.conversations.fetching" class="animate-spin" aria-hidden="true" />
          {{ store.conversations.fetching ? t('globals.terms.loading') : t('globals.terms.loadMore') }}
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { statusClass } from '@main/constants/kdTickets'
import { computed, defineComponent, h, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowDown, ArrowUp, ChevronDown, Inbox, Loader2, MessageCircleWarning } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Checkbox } from '@shared-ui/components/ui/checkbox'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@shared-ui/components/ui/dropdown-menu'
import { SidebarTrigger } from '@shared-ui/components/ui/sidebar'
import KdSidebarToggle from '@main/components/sidebar/KdSidebarToggle.vue'
import { useIsMobile } from '@shared-ui/composables'
import { formatFullTimestamp } from '@shared-ui/utils/datetime.js'
import { useConversationStore } from '@/stores/conversation'
import { useUsersStore } from '@/stores/users'
import { useTeamStore } from '@/stores/team'
import { useSharedViewStore } from '@/stores/sharedView'
import { useViewStore } from '@/stores/view'
import { useBulkActionPermissions } from '@/composables/useBulkActionPermissions'
import EmptyList from '@/features/conversation/list/ConversationEmptyList.vue'
import ConversationBulkActionToolbar from '@/features/conversation/list/ConversationBulkActionToolbar.vue'

const store = useConversationStore()
const usersStore = useUsersStore()
const teamStore = useTeamStore()
const sharedViewStore = useSharedViewStore()
const viewStore = useViewStore()
const { canBulkAct } = useBulkActionPermissions()
const route = useRoute()
const isMobile = useIsMobile()
const router = useRouter()
const { t } = useI18n()

// Team names for unassigned-to-agent tickets
watch(
  () => store.conversationsList.filter((c) => !c.assigned_user_id).map((c) => c.assigned_team_id),
  teamStore.ensureTeamIDs,
  { immediate: true }
)

const title = computed(() => {
  if (route.params.viewID) {
    const all = [...sharedViewStore.sharedViewList, ...viewStore.views]
    const view = all.find((v) => String(v.id) === String(route.params.viewID))
    if (view) return view.name
  }
  const typeKey = route.meta?.typeKey?.(route)
  if (typeKey) return t(typeKey)
  const key = route.meta?.titleKey
  return key ? t(key, route.meta?.titleCount || 1) : ''
})

const isCurrentView = (id) => String(route.params.viewID) === String(id)

const rows = computed(() => store.conversationsList)
const hasSelection = computed(() => store.selectedCount > 0)
const allSelected = computed(() => rows.value.length > 0 && store.selectedCount === rows.value.length)
const hasErrored = computed(() => !!store.conversations.errorMessage)
const showEmpty = computed(
  () => !rows.value.length && !hasErrored.value && !store.conversations.loading && store.conversations.initialized
)

function toggleAll () {
  allSelected.value ? store.clearSelection() : store.selectAll()
}

// Zendesk groups shared lists by assignee; skip it when everything has one owner
const grouped = computed(() => new Set(rows.value.map(assigneeLabel)).size > 1)
const groups = computed(() => {
  if (!grouped.value) return [{ key: 'all', rows: rows.value }]
  const map = new Map()
  for (const c of rows.value) {
    const label = assigneeLabel(c)
    if (!map.has(label)) map.set(label, { key: label, label, rows: [] })
    map.get(label).rows.push(c)
  }
  return [...map.values()]
})

function assigneeLabel (c) {
  const agent = usersStore.users.find((u) => String(u.id) === String(c.assigned_user_id))
  if (agent) return [agent.first_name, agent.last_name].filter(Boolean).join(' ')
  const team = teamStore.teams.find((tm) => String(tm.id) === String(c.assigned_team_id))
  if (team) return team.name
  return t('globals.terms.unassigned')
}

const subjectOf = (c) => c.subject || c.last_message || '(no subject)'
const updatedAt = (c) => c.last_interaction_at || c.last_message_at || c.updated_at

function shortDate (value) {
  if (!value) return ''
  const d = new Date(value)
  const now = new Date()
  const time = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (d.toDateString() === now.toDateString()) return `Today ${time}`
  return d.toLocaleDateString([], { month: 'short', day: '2-digit' })
}

// Zendesk badge colours: open = red, pending/replied = blue, solved = grey

function open (c) {
  const name = route.params.teamID
    ? 'team-inbox-conversation'
    : route.params.viewID
      ? 'view-inbox-conversation'
      : 'inbox-conversation'
  router.push({
    name,
    params: {
      uuid: c.uuid,
      ...(route.params.teamID && { teamID: route.params.teamID }),
      ...(route.params.viewID && { viewID: route.params.viewID })
    }
  })
}

// Clickable column header that cycles through the store's sort options
const SortHeader = defineComponent({
  props: { label: String, fields: Array },
  setup (props) {
    return () => {
      const current = store.conversations.sortField
      const activeIndex = props.fields.indexOf(current)
      const next = props.fields[(activeIndex + 1) % props.fields.length]
      const icon = activeIndex === 0 ? ArrowDown : activeIndex === 1 ? ArrowUp : null
      return h('th', { class: 'border-b border-border py-2 pr-3' }, [
        h(
          'button',
          {
            type: 'button',
            class: 'inline-flex items-center gap-1 hover:text-foreground' + (activeIndex >= 0 ? ' text-foreground' : ''),
            onClick: () => store.setListSortField(next)
          },
          [props.label, icon ? h(icon, { class: 'size-3' }) : null]
        )
      ])
    }
  }
})
</script>
