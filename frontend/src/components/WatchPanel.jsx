import { useEffect, useRef, useState } from 'react'
import { fieldClass, formatWhen, ghostClass, labelClass, primaryClass } from '../format'

export default function WatchPanel({
  vehicles,
  geofences,
  positions,
  selectedId,
  onSelect,
  mode,
  onMode,
  lastFix,
  busy,
  ready,
  simNote,
  onSubmit,
  onSimulate,
  rules = [],
}) {
  const selected = vehicles.find((vehicle) => vehicle.id === selectedId) || null
  const snap = selected ? positions[selected.id] : null
  const [lat, setLat] = useState('')
  const [lng, setLng] = useState('')
  const [timestamp, setTimestamp] = useState('')
  const [fenceId, setFenceId] = useState('')
  const [error, setError] = useState('')
  const picked = useRef(false)

  useEffect(() => {
    if (picked.current || !geofences.length || !rules.length) return
    const watched = geofences.find((fence) => rules.some((rule) => rule.status === 'active' && rule.geofence_id === fence.id))
    setFenceId((watched || geofences[0]).id)
    picked.current = true
  }, [geofences, rules])

  useEffect(() => {
    const fix = snap?.current_location
    if (!fix) return
    setLat(String(fix.latitude))
    setLng(String(fix.longitude))
  }, [selectedId, snap?.current_location?.latitude, snap?.current_location?.longitude, snap?.current_location?.timestamp])

  async function submit(event) {
    event.preventDefault()
    setError('')
    if (!selected) {
      setError('Choose a vehicle first.')
      return
    }
    const latitude = Number(lat)
    const longitude = Number(lng)
    if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
      setError('Enter a numeric latitude and longitude, or pin the map.')
      return
    }
    try {
      await onSubmit(selected.id, latitude, longitude, timestamp ? new Date(timestamp).toISOString() : '')
    } catch (err) {
      setError(err.message)
    }
  }

  async function simulate() {
    setError('')
    if (!selected || !fenceId) {
      setError('Choose a vehicle and a zone to cross.')
      return
    }
    try {
      await onSimulate(selected.id, fenceId)
    } catch (err) {
      setError(err.message)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <form onSubmit={submit} className="space-y-3">
        <div>
          <h2 className="font-display text-lg uppercase tracking-wide">Watch a vehicle</h2>
          <p className="text-sm text-mute">
          {ready ? 'Update a fix from the form, or turn on pin mode and click the map.' : 'Loading vehicles…'}
        </p>
        </div>
        <label className="block">
          <span className={labelClass}>Vehicle</span>
          <select className={fieldClass} value={selectedId} onChange={(event) => onSelect(event.target.value)}>
            <option value="">Select a vehicle</option>
            {vehicles.map((vehicle) => (
              <option key={vehicle.id} value={vehicle.id}>
                {vehicle.vehicle_number} · {vehicle.driver_name}
              </option>
            ))}
          </select>
        </label>
        <div className="grid grid-cols-2 gap-3">
          <label>
            <span className={labelClass}>Latitude</span>
            <input className={fieldClass} value={lat} onChange={(event) => setLat(event.target.value)} placeholder="37.7799" />
          </label>
          <label>
            <span className={labelClass}>Longitude</span>
            <input className={fieldClass} value={lng} onChange={(event) => setLng(event.target.value)} placeholder="-122.4144" />
          </label>
        </div>
        <label className="block">
          <span className={labelClass}>Timestamp</span>
          <input className={fieldClass} type="datetime-local" value={timestamp} onChange={(event) => setTimestamp(event.target.value)} />
          <span className="mt-1 block text-xs text-mute">Leave blank to send the current time.</span>
        </label>
        <div className="flex flex-wrap gap-2">
          <button className={primaryClass} type="submit" disabled={busy || !selected}>
            {busy ? 'Saving…' : 'Update location'}
          </button>
          <button
            className={mode === 'pin' ? primaryClass : ghostClass}
            type="button"
            onClick={() => onMode(mode === 'pin' ? 'view' : 'pin')}
          >
            {mode === 'pin' ? 'Pinning — click the map' : 'Pin on map'}
          </button>
        </div>
        {error && <p className="text-sm text-ember">{error}</p>}
        {snap?.current_location && (
          <div className="glass-inset p-3 text-sm">
            <p className="text-mute">Last fix · {formatWhen(snap.current_location.timestamp)}</p>
            <p className="font-mono text-mist">
              {snap.current_location.latitude}, {snap.current_location.longitude}
            </p>
            <div className="mt-2 flex flex-wrap gap-2">
              {(snap.current_geofences || []).length === 0 && <span className="text-mute">Outside every zone</span>}
              {(snap.current_geofences || []).map((fence) => (
                <span key={fence.geofence_id} className="glass-button glass-button-quiet px-2 py-0.5 text-xs">
                  {fence.geofence_name}
                </span>
              ))}
            </div>
          </div>
        )}
        {lastFix && (
          <p className="text-sm text-mist">
            {(lastFix.current_geofences || []).length === 0
              ? 'Server confirms the vehicle is outside every zone.'
              : `Server confirms ${(lastFix.current_geofences || []).map((fence) => fence.geofence_name).join(', ')}.`}
          </p>
        )}
      </form>
      <div className="glass-inset space-y-3 p-3">
        <h3 className="font-display text-lg uppercase tracking-wide">Run a crossing</h3>
        <p className="text-sm text-mute">Steps the vehicle outside the zone, through the middle, then back out.</p>
        <label className="block">
          <span className={labelClass}>Zone</span>
          <select className={fieldClass} value={fenceId} onChange={(event) => { picked.current = true; setFenceId(event.target.value) }}>
            {geofences.map((fence) => (
              <option key={fence.id} value={fence.id}>{zoneOptionLabel(fence, rules)}</option>
            ))}
          </select>
        </label>
        <p className="text-sm text-mute">{zoneWatchCopy(fenceId, rules)}</p>
        <button className={ghostClass} type="button" disabled={busy || !selected || !fenceId} onClick={simulate}>
          {simNote || 'Cross this zone'}
        </button>
      </div>
    </div>
  )
}

function zoneOptionLabel(fence, rules) {
  const active = rules.filter((rule) => rule.status === 'active' && rule.geofence_id === fence.id)
  if (!active.length) return `${fence.name} · history only`
  return `${fence.name} · live alert`
}

function zoneWatchCopy(fenceId, rules) {
  const active = rules.filter((rule) => rule.status === 'active' && rule.geofence_id === fenceId)
  if (!fenceId) return 'Choose a zone.'
  if (!active.length) return 'This zone has no alert rule. Crossing it is saved in History, and the Live feed stays empty.'
  return active.map((rule) => {
    const who = rule.vehicle_number || 'Every vehicle'
    const when = rule.event_type === 'both' ? 'enters or leaves' : rule.event_type === 'entry' ? 'enters' : 'leaves'
    return `${who} ${when} this zone, and the Live feed shows it.`
  }).join(' ')
}
