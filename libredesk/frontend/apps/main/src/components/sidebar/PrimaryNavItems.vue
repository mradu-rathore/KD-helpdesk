<template>
  <SidebarMenuItem v-for="item in items" :key="item.key">
    <Tooltip>
      <TooltipTrigger as-child>
        <SidebarMenuButton asChild :isActive="item.isActive">
          <router-link
            :to="item.to"
            :aria-label="item.label"
            :aria-current="item.isActive ? 'page' : undefined"
          >
            <component :is="item.icon" aria-hidden="true" />
            <span v-if="variant !== 'rail'">{{ item.label }}</span>
          </router-link>
        </SidebarMenuButton>
      </TooltipTrigger>
      <TooltipContent v-if="variant === 'rail'" side="right">
        <p>{{ item.label }}</p>
      </TooltipContent>
    </Tooltip>
  </SidebarMenuItem>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useStorage } from '@vueuse/core'
import { Building2, Home, LayoutGrid, PackageOpen, Shield, Ticket, Users } from 'lucide-vue-next'
import { SidebarMenuButton, SidebarMenuItem } from '@shared-ui/components/ui/sidebar'
import { Tooltip, TooltipContent, TooltipTrigger } from '@shared-ui/components/ui/tooltip'
import { useUserStore } from '@main/stores/user'

const props = defineProps({
  // 'rail' (icon only), 'drawer' (mobile nav drawer) or 'full' (KD desktop sidebar).
  variant: { type: String, default: 'rail' },
  includeAdmin: { type: Boolean, default: true }
})

const route = useRoute()
const { t } = useI18n()
const userStore = useUserStore()

const lastInboxPath = useStorage('lastInboxPath', '')

const inboxTarget = computed(() => {
  // KD: always the ticket list; open tickets stay in the tab strip
  if (!lastInboxPath.value) return { name: 'inboxes' }
  return lastInboxPath.value.replace(/\/conversation\/[^/]+$/, '') || { name: 'inboxes' }
})

// KD: Frappe-style main navigation
const items = computed(() =>
  [
    {
      key: 'home',
      icon: Home,
      label: 'Home',
      to: { name: 'home' },
      isActive: route.path.startsWith('/home'),
      show: true
    },
    {
      key: 'dashboard',
      icon: LayoutGrid,
      label: 'Dashboard',
      to: { name: 'dashboard' },
      isActive: route.path.startsWith('/dashboard') || route.path.startsWith('/reports'),
      show: userStore.hasReportTabPermissions
    },
    {
      key: 'tickets',
      icon: Ticket,
      label: 'Tickets',
      to: inboxTarget.value,
      isActive: route.path.startsWith('/inboxes'),
      show: true
    },
    {
      key: 'rmas',
      icon: PackageOpen,
      label: 'RMAs',
      to: { name: 'rmas' },
      isActive: route.path.startsWith('/rmas'),
      show: userStore.can('conversations:read')
    },
    {
      key: 'customers',
      icon: Building2,
      label: 'Customers',
      to: { name: 'customers' },
      isActive: route.path.startsWith('/customers'),
      show: userStore.can('contacts:read')
    },
    {
      key: 'contacts',
      icon: Users,
      label: t('globals.terms.contact', 2),
      to: { name: 'contacts' },
      isActive: route.path.startsWith('/contacts'),
      show: userStore.can('contacts:read_all')
    },
    {
      key: 'admin',
      icon: Shield,
      label: t('globals.terms.admin'),
      to: { name: userStore.can('general_settings:manage') ? 'general' : 'admin' },
      isActive: route.path.startsWith('/admin'),
      show: userStore.hasAdminTabPermissions && props.includeAdmin
    }
  ].filter((item) => item.show)
)
</script>