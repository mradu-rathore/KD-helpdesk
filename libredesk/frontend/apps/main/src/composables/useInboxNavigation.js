import { useRouter } from 'vue-router'

export function useInboxNavigation() {
  const router = useRouter()

  // KD: Zendesk flow - clicking a view (or inbox) always shows its ticket
  // list. Open tickets stay reachable from the tab strip, so we no longer
  // keep the current ticket on screen like the old split layout did.
  const navigate = (listName, params) => router.push({ name: listName, params })

  const navigateToInbox = (type) => navigate('inbox', { type })
  const navigateToTeamInbox = (teamID) => navigate('team-inbox', { teamID })
  const navigateToViewInbox = (viewID) => navigate('view-inbox', { viewID })

  return { navigateToInbox, navigateToTeamInbox, navigateToViewInbox }
}
