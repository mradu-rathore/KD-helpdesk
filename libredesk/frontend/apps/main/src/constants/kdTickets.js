// KD: Zendesk-style status badge colours, shared by every ticket list.
export const STATUS_CLASSES = {
  open: 'bg-red-600 text-white',
  new: 'bg-amber-400 text-black',
  replied: 'bg-blue-600 text-white',
  pending: 'bg-blue-600 text-white',
  'on-hold': 'bg-zinc-900 text-white dark:bg-zinc-200 dark:text-black',
  snoozed: 'bg-violet-600 text-white',
  resolved: 'bg-zinc-500 text-white',
  closed: 'bg-zinc-700 text-white'
}

export const statusClass = (status) =>
  STATUS_CLASSES[String(status || '').toLowerCase()] || 'bg-zinc-500 text-white'
