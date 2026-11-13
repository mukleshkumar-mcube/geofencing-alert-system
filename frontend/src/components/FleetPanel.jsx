import { useState } from 'react'
import { VEHICLE_TYPES, categoryMeta, fieldClass, ghostClass, labelClass, primaryClass } from '../format'
import PageBar from './PageBar'

const PAGE_SIZE = 6

export default function FleetPanel({ vehicles, positions, selectedId, onSelect, onCreate, ready }) {
  const [number, setNumber] = useState('')
  const [driver, setDriver] = useState('')
  const [kind, setKind] = useState('truck')
  const [phone, setPhone] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [page, setPage] = useState(1)
  const pages = Math.max(1, Math.ceil(vehicles.length / PAGE_SIZE))
  const safePage = Math.min(page, pages)
  const start = (safePage - 1) * PAGE_SIZE
  const slice = vehicles.slice(start, start + PAGE_SIZE)

  async function submit(event) {
    event.preventDefault()
    setError('')
    setSaving(true)
    try {
      await onCreate({
        vehicle_number: number.trim(),
        driver_name: driver.trim(),
        vehicle_type: kind,
        phone: phone.trim(),
      })
      setNumber('')
      setDriver('')
      setPhone('')
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h2 className="font-display text-lg uppercase tracking-wide">Fleet</h2>
        <p className="mb-3 text-sm text-mute">Registered vehicles and the zones they currently occupy.</p>
        <ul className="space-y-2">
          {!ready && <li className="text-sm text-white/70">Loading vehicles…</li>}
          {ready && vehicles.length === 0 && <li className="text-sm text-mute">No vehicles yet. Register one on the right.</li>}
          {slice.map((vehicle) => {
            const snap = positions[vehicle.id]
            const zones = snap?.current_geofences || []
            const active = vehicle.id === selectedId
            return (
              <li key={vehicle.id} className={`glass-inset px-3 py-2 ${active ? 'ring-1 ring-white/70' : ''}`}>
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="font-mono text-sm">{vehicle.vehicle_number}</p>
                    <p className="text-sm">{vehicle.driver_name} · {vehicle.vehicle_type}</p>
                    <p className="text-xs text-mute">{vehicle.phone}</p>
                  </div>
                  <button className={ghostClass} type="button" onClick={() => onSelect(vehicle.id)}>
                    {active ? 'Tracking' : 'Track'}
                  </button>
                </div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {zones.length === 0 && <span className="text-xs text-mute">{snap?.current_location ? 'Outside every zone' : 'No fix yet'}</span>}
                  {zones.map((fence) => (
                    <span key={fence.geofence_id} className="rounded-full px-2 py-0.5 text-[11px] font-semibold text-ink" style={{ background: categoryMeta(fence.category).swatch }}>
                      {fence.geofence_name}
                    </span>
                  ))}
                </div>
              </li>
            )
          })}
        </ul>
        <PageBar
          page={safePage}
          pages={pages}
          from={vehicles.length === 0 ? 0 : start + 1}
          to={Math.min(vehicles.length, start + slice.length)}
          total={vehicles.length}
          onPrev={() => setPage(safePage - 1)}
          onNext={() => setPage(safePage + 1)}
        />
      </div>
      <form onSubmit={submit} className="space-y-3">
        <h3 className="font-display text-xl uppercase tracking-wide">Register</h3>
        <label className="block">
          <span className={labelClass}>Vehicle number</span>
          <input className={fieldClass} value={number} onChange={(event) => setNumber(event.target.value)} placeholder="KA-01-AB-1234" />
        </label>
        <label className="block">
          <span className={labelClass}>Driver</span>
          <input className={fieldClass} value={driver} onChange={(event) => setDriver(event.target.value)} placeholder="John Doe" />
        </label>
        <label className="block">
          <span className={labelClass}>Type</span>
          <select className={fieldClass} value={kind} onChange={(event) => setKind(event.target.value)}>
            {VEHICLE_TYPES.map((item) => (
              <option key={item} value={item}>{item}</option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className={labelClass}>Phone</span>
          <input className={fieldClass} value={phone} onChange={(event) => setPhone(event.target.value)} placeholder="+14155550100" />
        </label>
        {error && <p className="text-sm text-ember">{error}</p>}
        <button className={primaryClass} disabled={saving}>{saving ? 'Saving…' : 'Add vehicle'}</button>
      </form>
    </div>
  )
}
