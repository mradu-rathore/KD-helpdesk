// KD: Zendesk-style tabs for open tickets. Each ticket an agent opens gets a
// tab; tabs remember which view the ticket was opened from and persist per agent.
import { defineStore } from 'pinia'
import { useStorage } from '@vueuse/core'

const MAX_TABS = 12

export const useKdTicketTabsStore = defineStore('kdTicketTabs', () => {
  const tabs = useStorage('kd_ticket_tabs', [])

  function open (uuid, route) {
    const existing = tabs.value.find((tab) => tab.uuid === uuid)
    if (existing) {
      existing.route = route
      return
    }
    tabs.value = [...tabs.value, { uuid, title: 'Ticket', route }].slice(-MAX_TABS)
  }

  function rename (uuid, title) {
    const tab = tabs.value.find((t) => t.uuid === uuid)
    if (tab && title && tab.title !== title) tab.title = title
  }

  /** Removes a tab; returns the tab to show instead (right neighbour, else left), or null. */
  function close (uuid) {
    const index = tabs.value.findIndex((tab) => tab.uuid === uuid)
    if (index === -1) return null
    const next = tabs.value[index + 1] ?? tabs.value[index - 1] ?? null
    tabs.value = tabs.value.filter((_, i) => i !== index)
    return next
  }

  return { tabs, open, rename, close }
})
