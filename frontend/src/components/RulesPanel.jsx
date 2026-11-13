import { useEffect, useState } from 'react'
import { fieldClass, formatWhen, ghostClass, labelClass, primaryClass } from '../format'
import PageBar from './PageBar'

const PAGE_SIZE = 6

export default function RulesPanel({ rules, geofences, vehicles, onCreate, onSetStatus, onDelete, ready }) {
  const [geofenceId, setGeofenceId] = useState(geofences[0]?.id || '')
  const [vehicleId, setVehicleId] = useState('')
  const [eventType, setEventType] = useState('entry')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [busyId, setBusyId] = useState('')
  const [fenceFilter, setFenceFilter] = useState('')
  const [vehicleFilter, setVehicleFilter] = useState('')
  const [page, setPage] = useState(1)

  const visible = rules.filter((rule) => {
    if (fenceFilter && rule.geofence_id !== fenceFilter) return false
    if (vehicleFilter && rule.vehicle_id !== vehicleFilter) return false
    return true
  })
  const pages = Math.max(1, Math.ceil(visible.length / PAGE_SIZE))
  const safePage = Math.min(page, pages)
  const start = (safePage - 1) * PAGE_SIZE
  const slice = visible.slice(start, start + PAGE_SIZE)

  useEffect(() => {
    setPage(1)
  }, [fenceFilter, vehicleFilter])

  async function submit(event) {
    event.preventDefault()
    setError('')
    if (!geofenceId) {
      setError('Choose a zone.')
      return
    }
    const body = { geofence_id: geofenceId, event_type: eventType }
    if (vehicleId) body.vehicle_id = vehicleId
    setSaving(true)
    try {
      await onCreate(body)
      setVehicleId('')
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h2 className="font-display text-lg uppercase tracking-wide">Alert rules</h2>
        <p className="mb-3 text-sm text-mute">A rule with no vehicle watches the whole fleet. Crossings are always stored; rules decide what is pushed live.</p>
        <div className="mb-3 grid grid-cols-2 gap-2">
          <select className={fieldClass} value={fenceFilter} onChange={(event) => setFenceFilter(event.target.value)}>
            <option value="">All zones</option>
            {geofences.map((fence) => <option key={fence.id} value={fence.id}>{fence.name}</option>)}
          </select>
          <select className={fieldClass} value={vehicleFilter} onChange={(event) => setVehicleFilter(event.target.value)}>
            <option value="">All rules</option>
            {vehicles.map((vehicle) => <option key={vehicle.id} value={vehicle.id}>{vehicle.vehicle_number}</option>)}
          </select>
        </div>
        <ul className="space-y-2">
          {!ready && <li className="text-sm text-white/70">Loading alert rules…</li>}
          {ready && visible.length === 0 && <li className="text-sm text-mute">No rules match. Save one to start watching a zone.</li>}
          {slice.map((rule) => (
            <li key={rule.alert_id} className="glass-inset px-3 py-2 text-sm">
              <p className="font-semibold">{rule.geofence_name}</p>
              <p className="text-mute">
                {rule.vehicle_number || 'All vehicles'} · {rule.event_type} · {rule.status}
              </p>
              <p className="font-mono text-xs text-mute">{formatWhen(rule.created_at)}</p>
              <div className="mt-2 flex flex-wrap gap-2">
                <button
                  className={ghostClass}
                  type="button"
                  disabled={busyId === rule.alert_id}
                  onClick={async () => {
                    setBusyId(rule.alert_id)
                    setError('')
                    try {
                      await onSetStatus(rule.alert_id, rule.status === 'active' ? 'inactive' : 'active')
                    } catch (err) {
                      setError(err.message)
                    } finally {
                      setBusyId('')
                    }
                  }}
                >
                  {busyId === rule.alert_id ? 'Saving…' : rule.status === 'active' ? 'Pause' : 'Resume'}
                </button>
                <button
                  className={ghostClass}
                  type="button"
                  disabled={busyId === rule.alert_id}
                  onClick={async () => {
                    setBusyId(rule.alert_id)
                    setError('')
                    try {
                      await onDelete(rule.alert_id)
                    } catch (err) {
                      setError(err.message)
                    } finally {
                      setBusyId('')
                    }
                  }}
                >
                  Remove
                </button>
              </div>
            </li>
          ))}
        </ul>
        <PageBar
          page={safePage}
          pages={pages}
          from={visible.length === 0 ? 0 : start + 1}
          to={Math.min(visible.length, start + slice.length)}
          total={visible.length}
          onPrev={() => setPage(safePage - 1)}
          onNext={() => setPage(safePage + 1)}
        />
      </div>
      <form onSubmit={submit} className="space-y-3">
        <h3 className="font-display text-xl uppercase tracking-wide">New rule</h3>
        <label className="block">
          <span className={labelClass}>Zone</span>
          <select className={fieldClass} value={geofenceId} onChange={(event) => setGeofenceId(event.target.value)}>
            {geofences.map((fence) => <option key={fence.id} value={fence.id}>{fence.name}</option>)}
          </select>
        </label>
        <label className="block">
          <span className={labelClass}>Vehicle</span>
          <select className={fieldClass} value={vehicleId} onChange={(event) => setVehicleId(event.target.value)}>
            <option value="">All vehicles</option>
            {vehicles.map((vehicle) => (
              <option key={vehicle.id} value={vehicle.id}>{vehicle.vehicle_number}</option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className={labelClass}>Event</span>
          <select className={fieldClass} value={eventType} onChange={(event) => setEventType(event.target.value)}>
            <option value="entry">Entry</option>
            <option value="exit">Exit</option>
            <option value="both">Entry and exit</option>
          </select>
        </label>
        {error && <p className="text-sm text-ember">{error}</p>}
        <button className={primaryClass} disabled={saving || geofences.length === 0}>
          {saving ? 'Saving…' : 'Save rule'}
        </button>
      </form>
    </div>
  )
}
