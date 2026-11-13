import { useCallback, useEffect, useRef, useState } from 'react'
import { api, websocketURL } from './api'
import { closeRing, pointInPolygon, polygonCentroid } from './geo'
import { CATEGORIES, fieldClass, ghostClass } from './format'
import MapCanvas from './components/MapCanvas'
import WatchPanel from './components/WatchPanel'
import ZonePanel from './components/ZonePanel'
import FleetPanel from './components/FleetPanel'
import RulesPanel from './components/RulesPanel'
import LogPanel from './components/LogPanel'
import LiveFeed from './components/LiveFeed'

const NAV = [
  { id: 'watch', label: 'Watch' },
  { id: 'zones', label: 'Zones' },
  { id: 'fleet', label: 'Fleet' },
  { id: 'rules', label: 'Rules' },
  { id: 'history', label: 'History' },
]

const EMPTY_FILTERS = { vehicle_id: '', geofence_id: '', start_date: '', end_date: '', limit: 25, offset: 0 }

export default function App() {
  const [section, setSection] = useState('watch')
  const [geofences, setGeofences] = useState([])
  const [vehicles, setVehicles] = useState([])
  const [rules, setRules] = useState([])
  const [positions, setPositions] = useState({})
  const [events, setEvents] = useState([])
  const [toasts, setToasts] = useState([])
  const [wsState, setWsState] = useState('connecting')
  const [mode, setMode] = useState('view')
  const [draft, setDraft] = useState([])
  const [drawSerial, setDrawSerial] = useState(0)
  const [selectedId, setSelectedId] = useState('')
  const [flash, setFlash] = useState(null)
  const [lastFix, setLastFix] = useState(null)
  const [locBusy, setLocBusy] = useState(false)
  const [simNote, setSimNote] = useState('')
  const [filters, setFilters] = useState(EMPTY_FILTERS)
  const [rows, setRows] = useState([])
  const [total, setTotal] = useState(0)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [apiKey, setApiKey] = useState(() => localStorage.getItem('harbor.apiKey') || '')
  const [fitNonce, setFitNonce] = useState(0)
  const [followNonce, setFollowNonce] = useState(0)
  const [connEpoch, setConnEpoch] = useState(0)
  const [ready, setReady] = useState(false)
  const [clock, setClock] = useState(() => new Date())

  const selectedRef = useRef('')
  const insideRef = useRef({})
  const filtersRef = useRef(filters)
  const refreshCoreRef = useRef(async () => {})
  const refreshHistoryRef = useRef(async () => {})
  selectedRef.current = selectedId
  filtersRef.current = filters

  const loadPositions = useCallback(async (list) => {
    const pairs = await Promise.all(list.map(async (vehicle) => {
      try {
        const snap = await api(`/vehicles/location/${encodeURIComponent(vehicle.id)}`)
        return [vehicle.id, snap]
      } catch {
        return [vehicle.id, null]
      }
    }))
    setPositions(Object.fromEntries(pairs))
  }, [])

  const refreshCore = useCallback(async () => {
    try {
      const [fenceRes, vehicleRes, ruleRes] = await Promise.all([
        api('/geofences'),
        api('/vehicles'),
        api('/alerts'),
      ])
      const fenceList = fenceRes.geofences || []
      const vehicleList = vehicleRes.vehicles || []
      setGeofences(fenceList)
      setVehicles(vehicleList)
      setRules(ruleRes.alerts || [])
      if (!selectedRef.current && vehicleList[0]) setSelectedId(vehicleList[0].id)
      await loadPositions(vehicleList)
    } finally {
      setReady(true)
    }
  }, [loadPositions])

  const refreshHistory = useCallback(async () => {
    const current = filtersRef.current
    const params = new URLSearchParams()
    if (current.vehicle_id) params.set('vehicle_id', current.vehicle_id)
    if (current.geofence_id) params.set('geofence_id', current.geofence_id)
    if (current.start_date) params.set('start_date', new Date(current.start_date).toISOString())
    if (current.end_date) params.set('end_date', new Date(current.end_date).toISOString())
    params.set('limit', String(current.limit || 25))
    params.set('offset', String(current.offset || 0))
    setHistoryLoading(true)
    try {
      const data = await api(`/violations/history?${params.toString()}`)
      setRows(data.violations || [])
      setTotal(data.total_count || 0)
    } finally {
      setHistoryLoading(false)
    }
  }, [])

  refreshCoreRef.current = refreshCore
  refreshHistoryRef.current = refreshHistory

  useEffect(() => {
    const next = {}
    Object.entries(positions).forEach(([id, snap]) => {
      next[id] = snap?.current_geofences || []
    })
    insideRef.current = next
  }, [positions])

  useEffect(() => {
    const timer = setInterval(() => setClock(new Date()), 1000)
    return () => clearInterval(timer)
  }, [])

  useEffect(() => {
    refreshCore().catch((err) => setFlash({ tone: 'bad', text: err.message }))
    refreshHistory().catch((err) => setFlash({ tone: 'bad', text: err.message }))
  }, [refreshCore, refreshHistory])

  useEffect(() => {
    refreshHistory().catch((err) => setFlash({ tone: 'bad', text: err.message }))
  }, [filters, refreshHistory])

  useEffect(() => {
    let stop = false
    let socket
    let timer
    let attempt = 0
    const connect = () => {
      if (stop) return
      setWsState('connecting')
      socket = new WebSocket(websocketURL())
      socket.onopen = () => {
        attempt = 0
        if (!stop) setWsState('live')
      }
      socket.onmessage = (message) => {
        let data
        try {
          data = JSON.parse(message.data)
        } catch {
          return
        }
        setEvents((curr) => [data, ...curr].slice(0, 80))
        const toastId = data.event_id || `${Date.now()}`
        setToasts((curr) => [...curr, { ...data, toastId }])
        setTimeout(() => {
          setToasts((curr) => curr.filter((item) => item.toastId !== toastId))
        }, 7000)
        refreshHistoryRef.current().catch(() => {})
        refreshCoreRef.current().catch(() => {})
      }
      socket.onerror = () => socket.close()
      socket.onclose = () => {
        if (stop) return
        setWsState('reconnecting')
        const delay = Math.min(8000, 600 * 2 ** attempt)
        attempt += 1
        timer = setTimeout(connect, delay)
      }
    }
    connect()
    return () => {
      stop = true
      clearTimeout(timer)
      socket?.close()
    }
  }, [connEpoch])

  async function sendLocation(vehicleId, lat, lng, timestamp) {
    setLocBusy(true)
    try {
      const res = await api('/vehicles/location', {
        method: 'POST',
        body: {
          vehicle_id: vehicleId,
          latitude: Number(lat),
          longitude: Number(lng),
          timestamp: timestamp || new Date().toISOString(),
        },
      })
      setLastFix(res)
      const before = insideRef.current[vehicleId] || []
      const after = res.current_geofences || []
      insideRef.current = { ...insideRef.current, [vehicleId]: after }
      const quiet = unwatchedCrossings(before, after, geofences, rules, vehicles.find((item) => item.id === vehicleId), Number(lat), Number(lng))
      if (quiet.length) setEvents((curr) => [...quiet, ...curr].slice(0, 80))
      const names = (res.current_geofences || []).map((fence) => fence.geofence_name)
      setFlash({
        tone: 'ok',
        text: names.length ? `Inside ${names.join(', ')}.` : 'Location saved. Outside every zone.',
      })
      await refreshCore()
      await refreshHistory()
      return res
    } catch (err) {
      setFlash({ tone: 'bad', text: err.message })
      throw err
    } finally {
      setLocBusy(false)
    }
  }

  async function simulate(vehicleId, geofenceId) {
    const fence = geofences.find((item) => item.id === geofenceId)
    if (!fence) return
    const center = polygonCentroid(fence.coordinates)
    let outside = null
    for (let step = 1; step <= 25; step += 1) {
      const candidate = [round6(center[0] + 0.008 * step), round6(center[1] + 0.008 * step)]
      const hit = geofences.some((item) => pointInPolygon(candidate[0], candidate[1], item.coordinates))
      if (!hit) {
        outside = candidate
        break
      }
    }
    if (!outside) outside = [round6(center[0] + 0.25), round6(center[1] + 0.25)]
    try {
      setSimNote('Moving outside')
      await sendLocation(vehicleId, outside[0], outside[1])
      await wait(700)
      setSimNote(`Entering ${fence.name}`)
      await sendLocation(vehicleId, round6(center[0]), round6(center[1]))
      await wait(700)
      setSimNote(`Leaving ${fence.name}`)
      await sendLocation(vehicleId, outside[0], outside[1])
      setFlash({ tone: 'ok', text: `Crossed ${fence.name}. Watch the live feed for the alert.` })
    } finally {
      setSimNote('')
    }
  }

  function onMapClick(lat, lng) {
    const point = [round6(lat), round6(lng)]
    if (mode === 'draw') {
      setDraft((curr) => [...curr, point])
      setDrawSerial((n) => n + 1)
      setSection('zones')
      return
    }
    if (mode === 'pin') {
      if (!selectedId) {
        setFlash({ tone: 'bad', text: 'Select a vehicle before pinning a location.' })
        return
      }
      if (!locBusy) sendLocation(selectedId, point[0], point[1]).catch(() => {})
    }
  }

  function undoPoint() {
    setDraft((curr) => curr.slice(0, -1))
    setDrawSerial((n) => n + 1)
  }

  function clearDraft() {
    setDraft([])
    setDrawSerial((n) => n + 1)
  }

  function closeDraft() {
    if (draft.length < 3) {
      setFlash({ tone: 'bad', text: 'Add at least 3 corners before closing the zone.' })
      return
    }
    const ring = closeRing(draft)
    setDraft(ring)
    setDrawSerial((n) => n + 1)
    setMode('view')
    setSection('zones')
  }

  async function createZone(payload) {
    await api('/geofences', { method: 'POST', body: payload })
    setDraft([])
    setMode('view')
    await refreshCore()
    setFlash({
      tone: 'ok',
      text: payload.category === 'restricted_zone'
        ? 'Restricted zone saved. Every vehicle is now watched for entry and exit.'
        : 'Zone saved.',
    })
  }

  async function createVehicle(payload) {
    const created = await api('/vehicles', { method: 'POST', body: payload })
    await refreshCore()
    setSelectedId(created.id)
    setFlash({ tone: 'ok', text: `${created.vehicle_number} is on the board.` })
  }

  async function createRule(payload) {
    await api('/alerts/configure', { method: 'POST', body: payload })
    await refreshCore()
    setFlash({ tone: 'ok', text: 'Alert rule saved.' })
  }

  async function setRuleStatus(alertId, status) {
    await api(`/alerts/${encodeURIComponent(alertId)}/status`, { method: 'POST', body: { status } })
    await refreshCore()
    setFlash({ tone: 'ok', text: status === 'active' ? 'Alert rule resumed.' : 'Alert rule paused. Crossings are still saved.' })
  }

  async function deleteRule(alertId) {
    await api(`/alerts/${encodeURIComponent(alertId)}`, { method: 'DELETE' })
    await refreshCore()
    setFlash({ tone: 'ok', text: 'Alert rule removed. Past crossings stay in history.' })
  }

  function saveKey() {
    const next = apiKey.trim()
    if (next) localStorage.setItem('harbor.apiKey', next)
    else localStorage.removeItem('harbor.apiKey')
    setConnEpoch((n) => n + 1)
    refreshCore().catch((err) => setFlash({ tone: 'bad', text: err.message }))
  }

  return (
    <div className="flex h-dvh flex-col gap-2 overflow-hidden p-2 lg:p-3">
      <header className="glass flex shrink-0 flex-wrap items-center gap-2 rounded-[24px] px-3 py-2 lg:flex-nowrap">
        <div className="glass-badge flex shrink-0 items-center gap-2 py-1 pl-1 pr-3">
          <span className="grid h-8 w-8 place-items-center rounded-full bg-sky-400 text-sm font-bold text-white">H</span>
          <div className="pr-1">
            <p className="text-sm font-semibold leading-none tracking-wide">Harbor</p>
            <p className="mt-1 text-[10px] uppercase tracking-[0.18em] text-white/60">Geofence watch</p>
          </div>
        </div>
        <nav className="flex min-w-0 flex-1 gap-1 overflow-x-auto">
          {NAV.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => setSection(item.id)}
              title={countFor(item.id, geofences, vehicles, rules, total)}
              className={`shrink-0 px-3 py-1.5 text-sm ${section === item.id ? 'glass-button-on' : 'glass-button glass-button-quiet text-white/80'}`}
            >
              {item.label}
            </button>
          ))}
        </nav>
        <div className="flex shrink-0 items-center gap-2">
          <input
            className={`${fieldClass} w-32 sm:w-40`}
            value={apiKey}
            onChange={(event) => setApiKey(event.target.value)}
            placeholder="API key"
            aria-label="API key"
          />
          <button className={ghostClass} type="button" onClick={saveKey}>Apply key</button>
          <p className="hidden font-mono text-sm text-mute sm:block">{clock.toLocaleTimeString()}</p>
        </div>
      </header>

      {!ready && (
        <div className="glass rounded-2xl px-4 py-2 text-sm" role="status">Loading zones, vehicles, and alert rules…</div>
      )}

      {flash && (
        <div className={`glass flex items-center justify-between gap-3 rounded-2xl px-4 py-2 text-sm ${flash.tone === 'bad' ? 'border-ember/70' : ''}`}>
          <p>{flash.text}</p>
          <button type="button" className="font-semibold" onClick={() => setFlash(null)}>Dismiss</button>
        </div>
      )}

      <div className="grid min-h-0 flex-1 grid-rows-[minmax(220px,42vh)_minmax(0,1fr)_168px] gap-2 lg:grid-cols-[minmax(300px,380px)_minmax(0,1fr)_300px] lg:grid-rows-1">
        <section className="glass order-2 min-h-0 overflow-auto rounded-[24px] px-3 py-3 lg:order-1">
            {section === 'watch' && (
              <WatchPanel
                vehicles={vehicles}
                geofences={geofences}
                positions={positions}
                selectedId={selectedId}
                onSelect={setSelectedId}
                mode={mode}
                onMode={(next) => { setMode(next); if (next === 'pin') setSection('watch') }}
                lastFix={lastFix}
                busy={locBusy}
                ready={ready}
                simNote={simNote}
                onSubmit={sendLocation}
                onSimulate={simulate}
                rules={rules}
              />
            )}
            {section === 'zones' && (
              <ZonePanel
                geofences={geofences}
                draft={draft}
                drawSerial={drawSerial}
                onCreate={createZone}
                onPreview={setDraft}
                ready={ready}
              />
            )}
            {section === 'fleet' && (
              <FleetPanel
                vehicles={vehicles}
                positions={positions}
                selectedId={selectedId}
                onSelect={(id) => { setSelectedId(id); setSection('watch') }}
                onCreate={createVehicle}
                ready={ready}
              />
            )}
            {section === 'rules' && (
              <RulesPanel
                rules={rules}
                geofences={geofences}
                vehicles={vehicles}
                onCreate={createRule}
                onSetStatus={setRuleStatus}
                onDelete={deleteRule}
                ready={ready}
              />
            )}
            {section === 'history' && (
              <LogPanel
                rows={rows}
                total={total}
                loading={historyLoading}
                vehicles={vehicles}
                geofences={geofences}
                filters={filters}
                onChange={setFilters}
                onSearch={() => refreshHistory().catch((err) => setFlash({ tone: 'bad', text: err.message }))}
              />
            )}
        </section>

        <div className="glass relative order-1 min-h-0 overflow-hidden rounded-[24px] p-1 lg:order-2">
          <div className="h-full min-h-[200px] overflow-hidden rounded-[20px]">
            <MapCanvas
              geofences={geofences}
              vehicles={vehicles}
              positions={positions}
              selectedId={selectedId}
              draft={draft}
              fitNonce={fitNonce}
              mode={mode}
              followNonce={followNonce}
              onMapClick={onMapClick}
              onSelectVehicle={(id) => {
                setSelectedId(id)
                setSection('watch')
              }}
            />
          </div>
          <div className="pointer-events-none absolute inset-x-3 top-3 z-[500] flex items-start justify-between gap-2">
            <div className="pointer-events-auto flex flex-wrap gap-2">
              <ModeButton active={mode === 'view'} onClick={() => { setMode('view'); setFollowNonce((n) => n + 1) }}>Navigate</ModeButton>
              <ModeButton active={mode === 'draw'} onClick={() => { setMode('draw'); setSection('zones') }}>Draw zone</ModeButton>
              <ModeButton active={mode === 'pin'} onClick={() => { setMode('pin'); setSection('watch') }}>Pin vehicle</ModeButton>
              {mode === 'draw' && (
                <>
                  <ModeButton onClick={undoPoint}>Undo</ModeButton>
                  <ModeButton onClick={clearDraft}>Clear</ModeButton>
                  <ModeButton onClick={closeDraft}>Use {draft.length} points</ModeButton>
                </>
              )}
            </div>
            <button className={`${ghostClass} pointer-events-auto`} type="button" onClick={() => setFitNonce((n) => n + 1)}>
              Fit
            </button>
          </div>
          <div className="pointer-events-none absolute bottom-8 left-3 z-[500] flex flex-wrap gap-2">
            {CATEGORIES.map((item) => (
              <span key={item.id} className="glass-button glass-button-quiet pointer-events-none px-2.5 py-1 text-[11px] text-white">
                <i className="mr-1 inline-block h-2 w-2 rounded-full" style={{ background: item.swatch }} />
                {item.label}
              </span>
            ))}
          </div>
        </div>

        <LiveFeed events={events} status={wsState} rules={rules} onClear={() => setEvents([])} />
      </div>

      <div className="pointer-events-none fixed right-4 top-20 z-[800] flex w-80 max-w-[calc(100%-2rem)] flex-col gap-2" aria-live="assertive">
        {toasts.map((toast) => (
          <div key={toast.toastId} className="glass pointer-events-auto rounded-2xl px-3 py-2">
            <p className="text-xs font-semibold uppercase tracking-wide text-white">{toast.event_type} · {toast.geofence?.geofence_name}</p>
            <p className="text-sm">{toast.vehicle?.vehicle_number} · {toast.vehicle?.driver_name}</p>
          </div>
        ))}
      </div>
    </div>
  )
}

function ModeButton({ active, onClick, children }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`px-3 py-1.5 text-xs font-semibold ${active ? 'glass-button-on' : 'glass-button glass-button-quiet'}`}
    >
      {children}
    </button>
  )
}

function countFor(id, geofences, vehicles, rules, total) {
  if (id === 'zones') return `${geofences.length} on the map`
  if (id === 'fleet') return `${vehicles.length} registered`
  if (id === 'rules') return `${rules.length} active`
  if (id === 'history') return `${total} crossings`
  return 'Live positions'
}

function round6(value) {
  return Math.round(value * 1e6) / 1e6
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function unwatchedCrossings(before, after, geofences, rules, vehicle, latitude, longitude) {
  const prev = new Map((before || []).map((fence) => [fence.geofence_id, fence]))
  const next = new Map((after || []).map((fence) => [fence.geofence_id, fence]))
  const found = []
  const push = (eventType, id, known) => {
    const fence = geofences.find((item) => item.id === id)
    const watched = rules.some((rule) => (
      rule.status === 'active'
      && rule.geofence_id === id
      && (!rule.vehicle_id || rule.vehicle_id === vehicle?.id)
      && (rule.event_type === eventType || rule.event_type === 'both')
    ))
    if (watched) return
    found.push({
      event_id: `local_${id}_${eventType}_${Date.now()}`,
      event_type: eventType,
      timestamp: new Date().toISOString(),
      unwatched: true,
      vehicle: vehicle ? { vehicle_number: vehicle.vehicle_number, driver_name: vehicle.driver_name } : {},
      geofence: {
        geofence_id: id,
        geofence_name: known?.geofence_name || fence?.name || id,
        category: fence?.category,
      },
      location: { latitude, longitude },
    })
  }
  next.forEach((fence, id) => { if (!prev.has(id)) push('entry', id, fence) })
  prev.forEach((fence, id) => { if (!next.has(id)) push('exit', id, fence) })
  return found
}
