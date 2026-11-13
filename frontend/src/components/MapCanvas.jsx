import { useEffect, useRef } from 'react'
import L from 'leaflet'
import { categoryMeta } from '../format'

export default function MapCanvas({
  geofences,
  vehicles,
  positions,
  selectedId,
  draft,
  fitNonce,
  mode,
  followNonce,
  onMapClick,
  onSelectVehicle,
}) {
  const elRef = useRef(null)
  const mapRef = useRef(null)
  const fenceLayer = useRef(null)
  const vehicleLayer = useRef(null)
  const draftLayer = useRef(null)
  const clickRef = useRef(onMapClick)
  const selectRef = useRef(onSelectVehicle)
  const modeRef = useRef(mode)
  const fitted = useRef(false)
  const followRef = useRef(true)
  const courseRef = useRef({})

  clickRef.current = onMapClick
  selectRef.current = onSelectVehicle
  modeRef.current = mode

  useEffect(() => {
    const map = L.map(elRef.current, { zoomControl: false }).setView([37.7799, -122.4194], 13)
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '&copy; OpenStreetMap',
      maxZoom: 19,
    }).addTo(map)
    L.control.zoom({ position: 'bottomright' }).addTo(map)
    L.control.scale({ position: 'bottomleft', imperial: false }).addTo(map)
    fenceLayer.current = L.layerGroup().addTo(map)
    vehicleLayer.current = L.layerGroup().addTo(map)
    draftLayer.current = L.layerGroup().addTo(map)
    map.on('click', (event) => {
      if (modeRef.current === 'view') return
      clickRef.current(event.latlng.lat, event.latlng.lng)
    })
    map.on('dragstart', () => {
      followRef.current = false
    })
    mapRef.current = map
    const observer = new ResizeObserver(() => map.invalidateSize())
    observer.observe(elRef.current)
    return () => {
      observer.disconnect()
      map.remove()
    }
  }, [])

  useEffect(() => {
    const map = mapRef.current
    if (!map) return
    map.getContainer().style.cursor = mode === 'view' ? '' : 'crosshair'
  }, [mode])

  useEffect(() => {
    const group = fenceLayer.current
    const map = mapRef.current
    if (!group || !map) return
    group.clearLayers()
    const bounds = []
    geofences.forEach((fence) => {
      const meta = categoryMeta(fence.category)
      const latlngs = fence.coordinates.map(([lat, lng]) => [lat, lng])
      const polygon = L.polygon(latlngs, {
        color: meta.swatch,
        weight: 2,
        fillColor: meta.swatch,
        fillOpacity: 0.22,
      })
      polygon.bindPopup(
        `<strong>${escapeHtml(fence.name)}</strong><br/>${escapeHtml(meta.label)}`,
      )
      polygon.addTo(group)
      latlngs.forEach((point) => bounds.push(point))
    })
    if (!fitted.current && bounds.length > 0) {
      map.fitBounds(bounds, { padding: [28, 28] })
      fitted.current = true
    }
  }, [geofences])

  useEffect(() => {
    const group = vehicleLayer.current
    if (!group) return
    group.clearLayers()
    vehicles.forEach((vehicle) => {
      const snap = positions[vehicle.id]
      const fix = snap?.current_location
      if (!fix) return
      const restricted = (snap.current_geofences || []).some((fence) => fence.category === 'restricted_zone')
      const selected = vehicle.id === selectedId
      const course = courseFor(courseRef.current, vehicle.id, fix.latitude, fix.longitude)
      const icon = L.divIcon({
        className: 'nav-icon',
        html: vehicleMarkerHtml({
          kind: vehicle.vehicle_type,
          plate: plateTail(vehicle.vehicle_number),
          selected,
          restricted,
          course,
        }),
        iconSize: [56, 78],
        iconAnchor: [28, 26],
      })
      const marker = L.marker([fix.latitude, fix.longitude], { icon, zIndexOffset: selected ? 600 : 0 })
      const zones = (snap.current_geofences || []).map((fence) => escapeHtml(fence.geofence_name)).join(', ')
      marker.bindPopup(
        `<strong>${escapeHtml(vehicle.vehicle_number)}</strong><br/>${escapeHtml(vehicle.driver_name)} · ${escapeHtml(vehicle.vehicle_type)}<br/>${zones || 'Outside every zone'}`,
      )
      marker.on('click', (event) => {
        L.DomEvent.stopPropagation(event)
        followRef.current = true
        selectRef.current(vehicle.id)
        const map = mapRef.current
        if (map && modeRef.current === 'view') {
          const zoom = map.getZoom() < 16 ? 17 : map.getZoom()
          map.flyTo([fix.latitude, fix.longitude], zoom, { duration: 0.6 })
        }
      })
      marker.addTo(group)
    })
  }, [vehicles, positions, selectedId])

  useEffect(() => {
    const group = draftLayer.current
    if (!group) return
    group.clearLayers()
    if (!draft.length) return
    const latlngs = draft.map(([lat, lng]) => [lat, lng])
    latlngs.forEach((point) => {
      L.circleMarker(point, {
        radius: 5,
        color: '#14181a',
        weight: 2,
        fillColor: '#d6ff4a',
        fillOpacity: 1,
      }).addTo(group)
    })
    if (latlngs.length >= 2) {
      const closed = samePoint(latlngs[0], latlngs[latlngs.length - 1]) && latlngs.length >= 4
      const ring = closed ? latlngs : latlngs.length >= 3 ? [...latlngs, latlngs[0]] : latlngs
      L.polyline(ring, { color: '#14181a', weight: 2, dashArray: closed ? null : '6 6' }).addTo(group)
    }
  }, [draft])

  useEffect(() => {
    if (mode === 'view') followRef.current = true
  }, [mode, followNonce, selectedId])

  useEffect(() => {
    const map = mapRef.current
    if (!map || mode !== 'view' || !followRef.current) return
    const fix = positions[selectedId]?.current_location
    if (!isFix(fix)) return
    const zoom = map.getZoom() < 16 ? 17 : map.getZoom()
    map.flyTo([fix.latitude, fix.longitude], zoom, { duration: 0.6 })
  }, [selectedId, positions, mode, followNonce])

  useEffect(() => {
    const map = mapRef.current
    if (!map || !fitNonce) return
    followRef.current = false
    const bounds = []
    geofences.forEach((fence) => fence.coordinates.forEach((point) => bounds.push(point)))
    draft.forEach((point) => bounds.push(point))
    if (bounds.length) map.fitBounds(bounds, { padding: [28, 28] })
  }, [fitNonce])

  return <div ref={elRef} className="h-full w-full" />
}

function plateTail(number) {
  const compact = String(number).replace(/\s+/g, '')
  return compact.slice(-4)
}

function isFix(fix) {
  return fix && Number.isFinite(fix.latitude) && Number.isFinite(fix.longitude)
}

function courseFor(store, id, lat, lng) {
  const prev = store[id]
  let course = prev?.course ?? 0
  if (prev && movedEnough(prev.lat, prev.lng, lat, lng)) {
    course = bearing(prev.lat, prev.lng, lat, lng)
  }
  store[id] = { lat, lng, course }
  return course
}

function movedEnough(lat1, lng1, lat2, lng2) {
  const north = (lat2 - lat1) * 111320
  const east = (lng2 - lng1) * 111320 * Math.cos((lat1 * Math.PI) / 180)
  return north * north + east * east > 144
}

function bearing(lat1, lng1, lat2, lng2) {
  const p1 = (lat1 * Math.PI) / 180
  const p2 = (lat2 * Math.PI) / 180
  const dLng = ((lng2 - lng1) * Math.PI) / 180
  const y = Math.sin(dLng) * Math.cos(p2)
  const x = Math.cos(p1) * Math.sin(p2) - Math.sin(p1) * Math.cos(p2) * Math.cos(dLng)
  return (Math.atan2(y, x) * 180) / Math.PI
}

function vehicleMarkerHtml({ kind, plate, selected, restricted, course }) {
  const fill = restricted ? '#e23d1a' : '#1a73e8'
  const classes = `nav-veh${selected ? ' is-selected' : ''}${restricted ? ' is-restricted' : ''}`
  const wedge = selected ? '<span class="nav-wedge"></span>' : ''
  const pulse = selected ? '<span class="nav-pulse"></span>' : ''
  return `<div class="${classes}">${pulse}<div class="nav-rot" style="transform: rotate(${course}deg)">${wedge}${vehicleSvg(kind, fill)}</div><span class="nav-plate">${escapeHtml(plate)}</span></div>`
}

function vehicleSvg(kind, fill) {
  const glass = '#eef6ff'
  const stroke = '#ffffff'
  if (kind === 'motorcycle') {
    return `<svg viewBox="0 0 64 64" width="42" height="42"><rect x="28" y="8" width="8" height="14" rx="3" fill="${fill}" stroke="${stroke}" stroke-width="2"/><rect x="26" y="22" width="12" height="24" rx="5" fill="${fill}" stroke="${stroke}" stroke-width="2"/><circle cx="32" cy="16" r="4" fill="#1b2430" stroke="${stroke}" stroke-width="2"/><circle cx="32" cy="50" r="5" fill="#1b2430" stroke="${stroke}" stroke-width="2"/></svg>`
  }
  if (kind === 'truck') {
    return `<svg viewBox="0 0 64 64" width="42" height="42"><path d="M20 24h24l4 8v22a4 4 0 0 1-4 4H20a4 4 0 0 1-4-4V28a4 4 0 0 1 4-4z" fill="${fill}" stroke="${stroke}" stroke-width="2"/><path d="M22 16h20l4 8H18z" fill="${fill}" stroke="${stroke}" stroke-width="2"/><path d="M26 18h12l2 6H24z" fill="${glass}"/><rect x="20" y="34" width="24" height="3" rx="1" fill="${glass}" opacity="0.75"/><rect x="20" y="42" width="24" height="3" rx="1" fill="${glass}" opacity="0.55"/></svg>`
  }
  if (kind === 'bus') {
    return `<svg viewBox="0 0 64 64" width="42" height="42"><rect x="16" y="8" width="32" height="48" rx="7" fill="${fill}" stroke="${stroke}" stroke-width="2"/><rect x="21" y="13" width="22" height="8" rx="2" fill="${glass}"/><rect x="21" y="26" width="9" height="7" rx="1" fill="${glass}"/><rect x="34" y="26" width="9" height="7" rx="1" fill="${glass}"/><rect x="21" y="37" width="9" height="7" rx="1" fill="${glass}"/><rect x="34" y="37" width="9" height="7" rx="1" fill="${glass}"/></svg>`
  }
  if (kind === 'van') {
    return `<svg viewBox="0 0 64 64" width="42" height="42"><path d="M18 28 22 14h20l4 14v24a4 4 0 0 1-4 4H22a4 4 0 0 1-4-4z" fill="${fill}" stroke="${stroke}" stroke-width="2"/><path d="M24 16h16l2 10H22z" fill="${glass}"/><rect x="22" y="36" width="20" height="8" rx="2" fill="${glass}" opacity="0.7"/></svg>`
  }
  return `<svg viewBox="0 0 64 64" width="42" height="42"><path d="M22 30c0-12 4-18 10-18s10 6 10 18l2 18c0 6-4 8-12 8s-12-2-12-8z" fill="${fill}" stroke="${stroke}" stroke-width="2"/><path d="M26 18h12c2 4 2 8 0 12H26c-2-4-2-8 0-12z" fill="${glass}"/><path d="M25 40h14c1 3 1 5 0 8H25c-1-3-1-5 0-8z" fill="${glass}" opacity="0.9"/></svg>`
}

function samePoint(a, b) {
  return a[0] === b[0] && a[1] === b[1]
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }[char]))
}
