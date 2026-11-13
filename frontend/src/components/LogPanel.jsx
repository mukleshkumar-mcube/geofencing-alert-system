import { fieldClass, formatWhen, ghostClass, labelClass, primaryClass } from '../format'

export default function LogPanel({ rows, total, loading, vehicles, geofences, filters, onChange, onSearch }) {
  const pageSize = filters.limit || 25
  const page = Math.floor((filters.offset || 0) / pageSize) + 1
  const pages = Math.max(1, Math.ceil(total / pageSize))
  const from = total === 0 ? 0 : filters.offset + 1
  const to = Math.min(total, filters.offset + rows.length)

  function update(patch) {
    onChange({ ...filters, offset: 0, ...patch })
  }

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="font-display text-lg uppercase tracking-wide">History</h2>
          <p className="text-sm text-mute">Every entry and exit, whether or not a live alert was sent.</p>
        </div>
        <p className="text-xs text-mute">{loading ? 'Loading…' : `${from}–${to} of ${total}`}</p>
      </div>
      <form
        className="mb-3 grid grid-cols-2 gap-2"
        onSubmit={(event) => {
          event.preventDefault()
          onSearch()
        }}
      >
        <label>
          <span className={labelClass}>Vehicle</span>
          <select className={fieldClass} value={filters.vehicle_id} onChange={(event) => update({ vehicle_id: event.target.value })}>
            <option value="">All</option>
            {vehicles.map((vehicle) => <option key={vehicle.id} value={vehicle.id}>{vehicle.vehicle_number}</option>)}
          </select>
        </label>
        <label>
          <span className={labelClass}>Zone</span>
          <select className={fieldClass} value={filters.geofence_id} onChange={(event) => update({ geofence_id: event.target.value })}>
            <option value="">All</option>
            {geofences.map((fence) => <option key={fence.id} value={fence.id}>{fence.name}</option>)}
          </select>
        </label>
        <label>
          <span className={labelClass}>From</span>
          <input className={fieldClass} type="datetime-local" value={filters.start_date} onChange={(event) => update({ start_date: event.target.value })} />
        </label>
        <label>
          <span className={labelClass}>To</span>
          <input className={fieldClass} type="datetime-local" value={filters.end_date} onChange={(event) => update({ end_date: event.target.value })} />
        </label>
        <div className="flex items-end gap-2">
          <button className={primaryClass} type="submit">Apply</button>
          <button
            className={ghostClass}
            type="button"
            onClick={() => onChange({ vehicle_id: '', geofence_id: '', start_date: '', end_date: '', limit: 25, offset: 0 })}
          >
            Reset
          </button>
        </div>
      </form>
      <ul className="space-y-2">
        {loading && rows.length === 0 && <li className="text-sm text-white/70">Loading crossings…</li>}
        {!loading && rows.length === 0 && (
          <li className="text-sm text-mute">No crossings yet. Move a vehicle into a zone to write the first one.</li>
        )}
        {rows.map((row) => (
          <li key={row.id} className="glass-inset px-3 py-2 text-sm">
            <div className="flex items-center justify-between gap-2">
              <span className={`rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase ${row.event_type === 'entry' ? 'chip-entry' : 'chip-exit'}`}>
                {row.event_type}
              </span>
              <span className="font-mono text-[11px] text-mute">{formatWhen(row.timestamp)}</span>
            </div>
            <p className="mt-1 font-semibold">{row.vehicle_number}</p>
            <p className="text-white/75">{row.geofence_name}</p>
            <p className="font-mono text-xs text-mute">{row.latitude}, {row.longitude}</p>
          </li>
        ))}
      </ul>
      <div className="mt-3 flex items-center justify-end gap-2">
        <button className={ghostClass} type="button" disabled={page <= 1} onClick={() => onChange({ ...filters, offset: Math.max(0, filters.offset - pageSize) })}>
          Previous
        </button>
        <span className="text-xs text-mute">Page {page} of {pages}</span>
        <button className={ghostClass} type="button" disabled={page >= pages} onClick={() => onChange({ ...filters, offset: filters.offset + pageSize })}>
          Next
        </button>
      </div>
    </div>
  )
}
