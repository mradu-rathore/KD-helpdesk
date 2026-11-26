<!--
  KD: one labelled desktop sidebar (Frappe Helpdesk style) instead of
  Libredesk's icon rail + inbox column: search, notifications, the main
  pages, then the ticket views with their counts.

  Collapses to an icon rail (toggle, ⌘B). It stays in the page flow rather
  than shadcn's fixed-position collapsible, so it keeps the app's rounded,
  padded layout; shadcn's own icon-mode styles apply through the
  data-collapsible="icon" attribute, and menu buttons show tooltips.
-->
<template>
  <SidebarProvider
    :style="{ '--sidebar-width': '15rem', '--sidebar-width-icon': '3rem' }"
    class="w-auto z-50"
    :open="!collapsed"
    @update:open="collapsed = !$event"
  >
    <ShadcnSidebar
      collapsible="none"
      :data-collapsible="collapsed ? 'icon' : undefined"
      :data-state="collapsed ? 'collapsed' : 'expanded'"
      :class="[
        'group overflow-hidden rounded-xl border border-sidebar-border transition-[width] duration-200 ease-out',
        collapsed && 'w-[--sidebar-width-icon]'
      ]"
    >
      <!-- Brand + collapse toggle (stacked when collapsed) -->
      <SidebarHeader class="px-3 pb-1 pt-3 group-data-[collapsible=icon]:px-2">
        <div class="flex items-center gap-1 group-data-[collapsible=icon]:flex-col">
          <router-link
            :to="{ name: 'home' }"
            class="flex min-w-0 flex-1 items-center gap-2 rounded-md px-1 py-1 group-data-[collapsible=icon]:flex-none group-data-[collapsible=icon]:p-0.5"
            :aria-label="siteName"
          >
            <span class="inline-flex size-7 shrink-0 items-center justify-center rounded-md bg-foreground text-[11px] font-bold text-background">
              KD
            </span>
            <span class="truncate text-sm font-semibold group-data-[collapsible=icon]:hidden">{{ siteName }}</span>
          </router-link>
          <KdSidebarToggle />
        </div>
      </SidebarHeader>

      <SidebarContent class="gap-0 group-data-[collapsible=icon]:overflow-y-auto group-data-[collapsible=icon]:[scrollbar-width:none]">
        <SidebarGroup class="pb-1">
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton :tooltip="t('globals.terms.search')" @click="palette.openPalette()">
                <Search />
                <span>{{ t('globals.terms.search') }}</span>
                <kbd class="ml-auto rounded border border-sidebar-border px-1 font-sans text-[10px] text-muted-foreground group-data-[collapsible=icon]:hidden">
                  {{ modKey }}K
                </kbd>
              </SidebarMenuButton>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <NotificationBell labelled />
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>

        <SidebarGroup class="py-1">
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton :tooltip="t('conversation.newConversation')" @click="emit('createConversation')">
                <SquarePen />
                <span>{{ t('conversation.newConversation') }}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
            <PrimaryNavItems variant="full" :include-admin="false" />
          </SidebarMenu>
        </SidebarGroup>

        <!-- Shared (public) views -->
        <SidebarGroup v-if="sharedViews.length" class="py-1 group-data-[collapsible=icon]:mx-2 group-data-[collapsible=icon]:border-t group-data-[collapsible=icon]:border-sidebar-border group-data-[collapsible=icon]:px-0">
          <Collapsible v-model:open="sharedOpen" class="group/collapsible">
            <CollapsibleTrigger as-child>
              <SidebarGroupLabel class="cursor-pointer select-none gap-1 hover:text-sidebar-foreground group-data-[collapsible=icon]:hidden">
                <ChevronRight class="!size-3.5 transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
                Public views
              </SidebarGroupLabel>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <SidebarMenu>
                <SidebarMenuItem v-for="view in sharedViews" :key="view.id">
                  <SidebarMenuButton :tooltip="view.name" :is-active="isViewActive(view.id)" @click="navigateToViewInbox(view.id)">
                    <component :is="viewIcon(view.name)" />
                    <span class="flex-1 truncate" :title="view.name">{{ view.name }}</span>
                    <SidebarCountBadge
                      :count="viewCount(view.id)"
                      :ariaLabel="t('conversation.sidebarCounts.view', viewCount(view.id))"
                    />
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    :tooltip="t('globals.terms.mention', 2)"
                    :is-active="route.path.startsWith('/inboxes/mentioned')"
                    @click="navigateToInbox('mentioned')"
                  >
                    <AtSign />
                    <span class="flex-1 truncate">{{ t('globals.terms.mention', 2) }}</span>
                    <SidebarCountBadge
                      :count="conversationStore.sidebarCounts.mentioned"
                      :ariaLabel="t('conversation.sidebarCounts.mentioned', conversationStore.sidebarCounts.mentioned)"
                    />
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </CollapsibleContent>
          </Collapsible>
        </SidebarGroup>

        <!-- Personal views -->
        <SidebarGroup v-if="userStore.can(permissions.VIEW_MANAGE)" class="py-1 group-data-[collapsible=icon]:mx-2 group-data-[collapsible=icon]:border-t group-data-[collapsible=icon]:border-sidebar-border group-data-[collapsible=icon]:px-0">
          <Collapsible v-model:open="myOpen" class="group/collapsible">
            <div class="group/label flex items-center">
              <CollapsibleTrigger as-child>
                <SidebarGroupLabel class="flex-1 cursor-pointer select-none gap-1 hover:text-sidebar-foreground group-data-[collapsible=icon]:hidden">
                  <ChevronRight class="!size-3.5 transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
                  My views
                </SidebarGroupLabel>
              </CollapsibleTrigger>
              <button
                type="button"
                class="mr-1 rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-sidebar-accent hover:text-sidebar-foreground focus-visible:opacity-100 group-hover/label:opacity-100 group-data-[collapsible=icon]:hidden"
                aria-label="Create view"
                @click="emit('createView')"
              >
                <Plus class="size-3.5" />
              </button>
            </div>
            <CollapsibleContent>
              <SidebarMenu>
                <SidebarMenuItem v-for="view in userViews" :key="view.id" class="group/view-item">
                  <SidebarMenuButton
                    :tooltip="view.name"
                    class="group-has-[[data-sidebar=menu-action]]/menu-item:pr-7"
                    :is-active="isViewActive(view.id)"
                    @click="navigateToViewInbox(view.id)"
                  >
                    <ListFilter />
                    <span class="flex-1 truncate" :title="view.name">{{ view.name }}</span>
                    <SidebarCountBadge
                      :count="viewCount(view.id)"
                      :ariaLabel="t('conversation.sidebarCounts.view', viewCount(view.id))"
                    />
                  </SidebarMenuButton>
                  <DropdownMenu>
                    <DropdownMenuTrigger as-child>
                      <SidebarMenuAction
                        class="can-hover:opacity-0 can-hover:group-hover/view-item:opacity-100 data-[state=open]:opacity-100"
                        @click.prevent
                      >
                        <EllipsisVertical />
                      </SidebarMenuAction>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent side="right" align="start">
                      <DropdownMenuItem @click="emit('editView', view)">{{ t('globals.messages.edit') }}</DropdownMenuItem>
                      <DropdownMenuItem @click="askDelete(view)">{{ t('globals.messages.delete') }}</DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </SidebarMenuItem>
                <p v-if="!userViews.length" class="px-2 py-1 text-xs text-muted-foreground group-data-[collapsible=icon]:hidden">
                  Save your own filters with +
                </p>
              </SidebarMenu>
            </CollapsibleContent>
          </Collapsible>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter class="gap-1">
        <SidebarMenu>
          <SidebarMenuItem v-if="userStore.hasAdminTabPermissions">
            <SidebarMenuButton as-child :tooltip="t('globals.terms.admin')" :is-active="route.path.startsWith('/admin')">
              <router-link :to="{ name: userStore.can('general_settings:manage') ? 'general' : 'admin' }">
                <Settings />
                <span>{{ t('globals.terms.admin') }}</span>
              </router-link>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem>
            <SidebarNavUser />
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </ShadcnSidebar>
  </SidebarProvider>

  <AlertDialog v-model:open="deleteOpen">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ t('globals.messages.areYouAbsolutelySure') }}</AlertDialogTitle>
        <AlertDialogDescription>{{ t('confirm.deleteView') }}</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ t('globals.messages.cancel') }}</AlertDialogCancel>
        <AlertDialogAction variant="destructive" @click="confirmDelete">{{ t('globals.messages.delete') }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useStorage } from '@vueuse/core'
import {
  AtSign,
  CheckCircle2,
  ChevronRight,
  Clock,
  EllipsisVertical,
  Inbox,
  ListFilter,
  ListTodo,
  PauseCircle,
  Plus,
  Search,
  Settings,
  SquarePen,
  UserRound,
  UserRoundX
} from 'lucide-vue-next'
import {
  Sidebar as ShadcnSidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider
} from '@shared-ui/components/ui/sidebar'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@shared-ui/components/ui/collapsible'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from '@shared-ui/components/ui/dropdown-menu'
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
import PrimaryNavItems from './PrimaryNavItems.vue'
import KdSidebarToggle from './KdSidebarToggle.vue'
import { useKdSidebar } from '@main/composables/useKdSidebar'
import NotificationBell from './NotificationBell.vue'
import SidebarNavUser from './SidebarNavUser.vue'
import SidebarCountBadge from './SidebarCountBadge.vue'
import { useCommandPalette } from '@/features/command/useCommandPalette'
import { useInboxNavigation } from '@main/composables/useInboxNavigation'
import { useUserStore } from '@main/stores/user'
import { useConversationStore } from '@main/stores/conversation'
import { useAppSettingsStore } from '@main/stores/appSettings'
import { permissions } from '@main/constants/permissions'

defineProps({
  userViews: { type: Array, default: () => [] },
  sharedViews: { type: Array, default: () => [] }
})
const emit = defineEmits(['createView', 'editView', 'deleteView', 'createConversation'])

const route = useRoute()
const { t } = useI18n()
const palette = useCommandPalette()
const userStore = useUserStore()
const conversationStore = useConversationStore()
const settingsStore = useAppSettingsStore()
const { navigateToInbox, navigateToViewInbox } = useInboxNavigation()

const { collapsed } = useKdSidebar()
const sharedOpen = useStorage('kd_sidebar_public_views_open', true)
const myOpen = useStorage('kd_sidebar_my_views_open', true)

const siteName = computed(() => {
  const name = settingsStore.settings?.['app.site_name']
  return name && name.toLowerCase() !== 'libredesk' ? name : 'KD Helpdesk'
})
const modKey = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘' : 'Ctrl '

const isViewActive = (id) => String(route.params.viewID) === String(id)
const viewCount = (id) => conversationStore.sidebarCounts.views?.[id] ?? 0

const VIEW_ICONS = {
  'your unsolved tickets': UserRound,
  'unassigned tickets': UserRoundX,
  'all unsolved tickets': ListTodo,
  'pending tickets': Clock,
  'on-hold tickets': PauseCircle,
  'recently solved tickets': CheckCircle2,
  'all tickets': Inbox
}
const viewIcon = (name) => VIEW_ICONS[String(name).toLowerCase()] || ListFilter

const deleteOpen = ref(false)
const pendingDelete = ref(null)
const askDelete = (view) => {
  pendingDelete.value = view
  deleteOpen.value = true
}
const confirmDelete = () => {
  if (pendingDelete.value) emit('deleteView', pendingDelete.value)
  pendingDelete.value = null
  deleteOpen.value = false
}

onMounted(() => conversationStore.fetchSidebarCounts({ force: true }))
</script>
