import { categoryMeta, formatWhen, ghostClass } from '../format'

export default function LiveFeed({ events, status, rules = [], onClear }) {
  const label = status === 'live' ? 'Live' : status === 'connecting' ? 'Connecting' : 'Reconnecting'
  const dot = status === 'live' ? 'bg-lime' : status === 'connecting' ? 'bg-brass' : 'bg-ember'

  return (
    <aside className="glass order-3 flex h-full min-h-0 flex-col overflow-hidden rounded-[24px]">
      <div className="flex items-center justify-between gap-3 border-b border-white/15 px-4 py-3">
        <div>
          <p className="font-display text-xl uppercase tracking-wide">Live feed</p>
          <p className="flex items-center gap-2 text-xs text-mute">
            <span className={`inline-block h-2 w-2 rounded-full ${dot}`} />
            {label}
          </p>
        </div>
        <button className={ghostClass} type="button" onClick={onClear}>Clear</button>
      </div>
      <ul className="flex-1 space-y-2 overflow-auto px-3 py-3">
        {events.length === 0 && (
          <li className="glass-inset border-dashed px-3 py-4 text-sm text-mute">
            <p>No live alert yet. A card appears only when a vehicle crosses a zone that has an active rule.</p>
            <ul className="mt-2 space-y-1">
              {rules.filter((rule) => rule.status === 'active').map((rule) => (
                <li key={rule.alert_id}>
                  {rule.vehicle_number || 'Every vehicle'} · {rule.geofence_name} · {rule.event_type}
                </li>
              ))}
            </ul>
          </li>
        )}
        {events.map((event) => {
          const meta = categoryMeta(event.geofence?.category)
          const hot = event.geofence?.category === 'restricted_zone'
          return (
            <li key={event.event_id} className={`glass-inset px-3 py-2 ${hot ? 'border-ember/70' : ''}`}>
              <div className="flex items-center justify-between gap-2">
                <span className={`rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase ${event.event_type === 'entry' ? 'chip-entry' : 'chip-exit'}`}>
                  {event.event_type}
                </span>
                <span className="font-mono text-[11px] text-mute">{formatWhen(event.timestamp)}</span>
              </div>
              <p className="mt-1 text-sm font-semibold">{event.vehicle?.vehicle_number}</p>
              <p className="text-sm text-mute">{event.vehicle?.driver_name}</p>
              <p className="mt-1 text-sm">
                {event.geofence?.geofence_name}
                <span className="ml-2 text-xs" style={{ color: meta.swatch }}>{meta.label}</span>
              </p>
              <p className="font-mono text-[11px] text-mute">
                {event.location?.latitude}, {event.location?.longitude}
              </p>
              {event.unwatched && (
                <p className="mt-1 text-xs text-white/70">Saved in History. No alert rule is watching this crossing, so it is not a live alert.</p>
              )}
            </li>
          )
        })}
      </ul>
    </aside>
  )
}
