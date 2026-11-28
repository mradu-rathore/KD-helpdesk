<!--
  KD: one labelled desktop sidebar (Frappe Helpdesk style) instead of
  Libredesk's icon rail + inbox column: search, notifications, the main
  pages. Ticket views are switched from the ticket list's title.

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
            <!-- KD: most support starts with a phone call -->
            <SidebarMenuItem>
              <SidebarMenuButton tooltip="Log call" @click="emitter.emit(KD_EVENTS.LOG_CALL)">
                <PhoneCall />
                <span>Log call</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
            <PrimaryNavItems variant="full" :include-admin="false" />
          </SidebarMenu>
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
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { PhoneCall, Search, Settings, SquarePen } from 'lucide-vue-next'
import { useEmitter } from '@main/composables/useEmitter'
import { KD_EVENTS } from '@main/constants/emitterEvents'
import {
  Sidebar as ShadcnSidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider
} from '@shared-ui/components/ui/sidebar'
import PrimaryNavItems from './PrimaryNavItems.vue'
import KdSidebarToggle from './KdSidebarToggle.vue'
import { useKdSidebar } from '@main/composables/useKdSidebar'
import NotificationBell from './NotificationBell.vue'
import SidebarNavUser from './SidebarNavUser.vue'
import { useCommandPalette } from '@/features/command/useCommandPalette'
import { useUserStore } from '@main/stores/user'
import { useConversationStore } from '@main/stores/conversation'
import { useAppSettingsStore } from '@main/stores/appSettings'

const emit = defineEmits(['createConversation'])

const route = useRoute()
const { t } = useI18n()
const palette = useCommandPalette()
const userStore = useUserStore()
const conversationStore = useConversationStore()
const settingsStore = useAppSettingsStore()
const { collapsed } = useKdSidebar()
const emitter = useEmitter()

const siteName = computed(() => {
  const name = settingsStore.settings?.['app.site_name']
  return name && name.toLowerCase() !== 'libredesk' ? name : 'KD Helpdesk'
})
const modKey = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘' : 'Ctrl '

onMounted(() => conversationStore.fetchSidebarCounts({ force: true }))
</script>
