// KD: open a ticket from pages outside the inbox (Home, Customers, Contacts).
// It opens inside the "All tickets" view, so it gets a tab and closing it
// lands on a real ticket list.
import { useRouter } from 'vue-router'
import { useSharedViewStore } from '@main/stores/sharedView'

export function useOpenTicket() {
  const router = useRouter()
  const sharedViewStore = useSharedViewStore()

  return (uuid) => {
    const views = sharedViewStore.sharedViewList || []
    const all = views.find((v) => v.name === 'All tickets')
    if (all) return router.push({ name: 'view-inbox-conversation', params: { viewID: all.id, uuid } })
    return router.push({ name: 'inbox-conversation', params: { type: 'all', uuid } })
  }
}
