// KD: RMA steps, outcomes and field options, shared by the RMA pages, the
// ticket panel and the dashboard. Keys match the backend (cmd/kd_rmas.go).

export const RMA_STEPS = [
  { key: 'requested', label: 'Requested', hint: 'Check the invoice and warranty', badge: 'bg-amber-400 text-black', dateField: 'created_at' },
  { key: 'approved', label: 'Approved', hint: 'Waiting for the goods', badge: 'bg-sky-600 text-white', dateField: 'approved_at' },
  { key: 'received', label: 'Received', hint: 'Goods are in', badge: 'bg-violet-600 text-white', dateField: 'received_at' },
  { key: 'testing', label: 'Testing', hint: 'Being checked', badge: 'bg-indigo-600 text-white', dateField: null },
  { key: 'with_supplier', label: 'With supplier', hint: 'Sent back to the manufacturer', badge: 'bg-orange-500 text-white', dateField: 'sent_to_supplier_at', optional: true },
  { key: 'resolved', label: 'Resolved', hint: 'Credited, replaced, repaired or returned', badge: 'bg-emerald-600 text-white', dateField: 'resolved_at' }
]
export const RMA_REJECTED = { key: 'rejected', label: 'Rejected', hint: 'Not accepted', badge: 'bg-zinc-600 text-white', dateField: 'resolved_at' }
export const RMA_STATUSES = [...RMA_STEPS, RMA_REJECTED]
export const OPEN_STATUSES = ['requested', 'approved', 'received', 'testing', 'with_supplier']

export const rmaStatus = (key) =>
  RMA_STATUSES.find((s) => s.key === key) || { key, label: key, badge: 'bg-zinc-500 text-white' }

// The usual next step from each status - the main button on an RMA.
export const RMA_NEXT = {
  requested: { to: 'approved', label: 'Approve' },
  approved: { to: 'received', label: 'Mark received' },
  received: { to: 'testing', label: 'Start testing' },
  testing: { to: 'resolved', label: 'Resolve' },
  with_supplier: { to: 'resolved', label: 'Resolve' }
}

export const RMA_OUTCOMES = [
  { key: 'replaced', label: 'Replaced', hint: 'A replacement went out', field: 'replacement_order_number', fieldLabel: 'Replacement order no.' },
  { key: 'credited', label: 'Credited', hint: 'A credit note was raised', field: 'credit_note_number', fieldLabel: 'Credit note no.' },
  { key: 'repaired', label: 'Repaired', hint: 'Repaired and sent back' },
  { key: 'returned_nff', label: 'Returned – no fault found', hint: 'Sent back as it was' }
]
export const outcomeLabel = (key) => RMA_OUTCOMES.find((o) => o.key === key)?.label || ''

export const RMA_WARRANTY = [
  { key: 'doa', label: 'Dead on arrival' },
  { key: 'in_warranty', label: 'In warranty' },
  { key: 'out_of_warranty', label: 'Out of warranty' }
]
export const RMA_TEST_RESULTS = [
  { key: 'fault_confirmed', label: 'Fault confirmed' },
  { key: 'no_fault_found', label: 'No fault found' },
  { key: 'damaged', label: 'Customer damage' }
]
export const optionLabel = (options, key) => options.find((o) => o.key === key)?.label || ''

// Brands KD carries; anything else can be typed in.
export const RMA_BRANDS = ['Dahua', 'Ajax', 'Alta Labs']
const BRAND_SUPPLIERS = { dahua: 'Dahua Technology', ajax: 'Ajax Systems', 'alta labs': 'Alta Labs' }
export const defaultSupplier = (brand) => BRAND_SUPPLIERS[String(brand || '').trim().toLowerCase()] || ''

// "3 × Ajax FireProtect 2"
export const rmaLine = (r) => {
  const product = r?.product || ''
  const brand = r?.brand || ''
  const name = brand && !product.toLowerCase().startsWith(brand.toLowerCase()) ? `${brand} ${product}` : product
  return `${r?.quantity || 1} × ${name}`
}

export const serialList = (s) => String(s || '').split('\n').filter(Boolean)

// Same badge as the ticket list's status badges.
export const BADGE = 'inline-block rounded px-1.5 py-0.5 text-[11px] font-semibold leading-4'

// Native selects styled like the app's inputs (give them a width where used).
export const SELECT_CLASS =
  'h-9 rounded-md border border-input bg-transparent px-2.5 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50'

// Same as the ticket list: "Today 14:05" or "24 Nov".
export function shortDate (value) {
  if (!value) return ''
  const d = new Date(value)
  const now = new Date()
  const time = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (d.toDateString() === now.toDateString()) return `Today ${time}`
  return d.toLocaleDateString([], { month: 'short', day: '2-digit' })
}
