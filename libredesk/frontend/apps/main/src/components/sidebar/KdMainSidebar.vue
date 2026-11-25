<!--
  KD: one labelled desktop sidebar (Frappe Helpdesk style) instead of
  Libredesk's icon rail + inbox column: search, notifications, the main
  pages. Ticket views are switched from the ticket list's title.
-->
<template>
  <SidebarProvider style="--sidebar-width: 15rem" class="w-auto z-50">
    <ShadcnSidebar collapsible="none" class="overflow-hidden rounded-xl border border-sidebar-border">
      <!-- Brand -->
      <SidebarHeader class="px-3 pb-1 pt-3">
        <router-link :to="{ name: 'home' }" class="flex items-center gap-2 rounded-md px-1 py-1">
          <span class="inline-flex size-7 items-center justify-center rounded-md bg-foreground text-[11px] font-bold text-background">
            KD
          </span>
          <span class="truncate text-sm font-semibold">{{ siteName }}</span>
        </router-link>
      </SidebarHeader>

      <SidebarContent class="gap-0">
        <SidebarGroup class="pb-1">
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton @click="palette.openPalette()">
                <Search />
                <span>{{ t('globals.terms.search') }}</span>
                <kbd class="ml-auto rounded border border-sidebar-border px-1 font-sans text-[10px] text-muted-foreground">
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
              <SidebarMenuButton @click="emit('createConversation')">
                <SquarePen />
                <span>{{ t('conversation.newConversation') }}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
            <PrimaryNavItems variant="full" :include-admin="false" />
          </SidebarMenu>
        </SidebarGroup>

      </SidebarContent>

      <SidebarFooter class="gap-1">
        <SidebarMenu>
          <SidebarMenuItem v-if="userStore.hasAdminTabPermissions">
            <SidebarMenuButton as-child :is-active="route.path.startsWith('/admin')">
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
import { Search, Settings, SquarePen } from 'lucide-vue-next'
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

const siteName = computed(() => {
  const name = settingsStore.settings?.['app.site_name']
  return name && name.toLowerCase() !== 'libredesk' ? name : 'KD Helpdesk'
})
const modKey = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘' : 'Ctrl '

onMounted(() => conversationStore.fetchSidebarCounts({ force: true }))
</script>
