export const CATEGORIES = [
  { id: 'delivery_zone', label: 'Delivery', swatch: '#65a30d' },
  { id: 'restricted_zone', label: 'Restricted', swatch: '#e23d1a' },
  { id: 'toll_zone', label: 'Toll', swatch: '#d97706' },
  { id: 'customer_area', label: 'Customer', swatch: '#1d6fbf' },
]

export const VEHICLE_TYPES = ['truck', 'van', 'car', 'motorcycle', 'bus']

export function categoryMeta(id) {
  return CATEGORIES.find((item) => item.id === id) || { id, label: id, swatch: '#314046' }
}

export function formatWhen(iso) {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export const fieldClass =
  'glass-field w-full px-4 py-2.5 text-sm outline-none placeholder:text-white/45'

export const labelClass = 'mb-1.5 block text-[11px] font-semibold tracking-wide text-white/70'

export const primaryClass =
  'glass-button px-4 py-2.5 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50'

export const ghostClass =
  'glass-button glass-button-quiet px-4 py-2.5 text-sm disabled:cursor-not-allowed disabled:opacity-50'
