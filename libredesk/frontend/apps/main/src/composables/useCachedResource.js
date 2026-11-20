// KD: stale-while-revalidate loader. Revisiting a page shows the last data
// instantly and refreshes it in the background, so navigation feels smooth
// instead of flashing a spinner every time.
import { ref, watch, unref, onActivated } from 'vue'

const cache = new Map()

export function invalidateCached(prefix) {
  for (const key of cache.keys()) if (key.startsWith(prefix)) cache.delete(key)
}

/**
 * @param {import('vue').MaybeRef<string|null>} key cache key; null skips loading
 * @param {() => Promise<any>} fetcher resolves to the data
 */
export function useCachedResource(key, fetcher) {
  const data = ref(null)
  const error = ref(null)
  const loading = ref(false) // true only when there is nothing to show yet
  const refreshing = ref(false)
  let seq = 0

  const load = async () => {
    const k = unref(key)
    if (!k) return
    const mine = ++seq
    const cached = cache.get(k)
    if (cached !== undefined) data.value = cached
    else {
      data.value = null
      loading.value = true
    }
    refreshing.value = true
    try {
      const fresh = await fetcher()
      if (mine !== seq) return
      cache.set(k, fresh)
      data.value = fresh
      error.value = null
    } catch (err) {
      if (mine === seq) error.value = err
    } finally {
      if (mine === seq) {
        loading.value = false
        refreshing.value = false
      }
    }
  }

  watch(() => unref(key), load, { immediate: true })
  // Kept-alive pages refresh quietly when shown again
  let first = true
  onActivated(() => {
    if (first) return (first = false)
    load()
  })

  const mutate = (next) => {
    data.value = next
    const k = unref(key)
    if (k) cache.set(k, next)
  }

  return { data, error, loading, refreshing, reload: load, mutate }
}
