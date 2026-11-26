// KD: whether the main sidebar is collapsed to its icon rail. Shared by the
// sidebar itself and the toggle buttons in page headers; remembered per browser.
import { useStorage } from '@vueuse/core'

const collapsed = useStorage('kd_sidebar_collapsed', false)

export function useKdSidebar () {
  const toggle = () => (collapsed.value = !collapsed.value)
  return { collapsed, toggle }
}
