import { useEffect, useRef, useState } from 'react'
import { CATEGORIES, categoryMeta, fieldClass, labelClass, primaryClass } from '../format'
import { closeRing, parseCoordinateText } from '../geo'
import PageBar from './PageBar'

const PAGE_SIZE = 6

export default function ZonePanel({ geofences, draft, drawSerial, onCreate, onPreview, ready }) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState('delivery_zone')
  const [filter, setFilter] = useState('all')
  const [text, setText] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [page, setPage] = useState(1)
  const seen = useRef(0)

  useEffect(() => {
    if (!drawSerial || drawSerial === seen.current) return
    seen.current = drawSerial
    setText(draft.map((point) => `${point[0]}, ${point[1]}`).join('\n'))
    setError('')
  }, [drawSerial, draft])

  const visible = geofences.filter((fence) => filter === 'all' || fence.category === filter)
  const pages = Math.max(1, Math.ceil(visible.length / PAGE_SIZE))
  const safePage = Math.min(page, pages)
  const start = (safePage - 1) * PAGE_SIZE
  const slice = visible.slice(start, start + PAGE_SIZE)

  useEffect(() => {
    setPage(1)
  }, [filter])

  async function submit(event) {
    event.preventDefault()
    setError('')
    if (!name.trim()) {
      setError('Give the zone a name.')
      return
    }
    let coordinates = []
    try {
      coordinates = closeRing(parseCoordinateText(text))
    } catch (err) {
      setError(err.message)
      return
    }
    if (coordinates.length < 4) {
      setError('A zone needs at least 3 corners. Draw them on the map or paste one pair per line.')
      return
    }
    setSaving(true)
    try {
      await onCreate({
        name: name.trim(),
        description: description.trim(),
        category,
        coordinates,
      })
      setName('')
      setDescription('')
      setText('')
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 className="font-display text-lg uppercase tracking-wide">Zones</h2>
            <p className="text-sm text-mute">Filter the board, or draw a new boundary on the map.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <FilterChip active={filter === 'all'} onClick={() => setFilter('all')}>All</FilterChip>
            {CATEGORIES.map((item) => (
              <FilterChip key={item.id} active={filter === item.id} onClick={() => setFilter(item.id)}>
                {item.label}
              </FilterChip>
            ))}
          </div>
        </div>
        <ul className="space-y-2">
          {!ready && <li className="text-sm text-white/70">Loading zones…</li>}
          {ready && visible.length === 0 && <li className="text-sm text-mute">No zones in this category yet.</li>}
          {slice.map((fence) => {
            const meta = categoryMeta(fence.category)
            return (
              <li key={fence.id} className="glass-inset flex items-start justify-between gap-3 px-3 py-2">
                <div>
                  <p className="font-semibold">{fence.name}</p>
                  <p className="text-sm text-mute">{fence.description || 'No description'}</p>
                </div>
                <span className="shrink-0 rounded-full px-2 py-0.5 text-xs font-semibold text-ink" style={{ background: meta.swatch }}>
                  {meta.label}
                </span>
              </li>
            )
          })}
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
        <h3 className="font-display text-xl uppercase tracking-wide">New zone</h3>
        <label className="block">
          <span className={labelClass}>Name</span>
          <input className={fieldClass} value={name} onChange={(event) => setName(event.target.value)} placeholder="Downtown Delivery Zone" />
        </label>
        <label className="block">
          <span className={labelClass}>Description</span>
          <input className={fieldClass} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Optional" />
        </label>
        <label className="block">
          <span className={labelClass}>Category</span>
          <select className={fieldClass} value={category} onChange={(event) => setCategory(event.target.value)}>
            {CATEGORIES.map((item) => (
              <option key={item.id} value={item.id}>{item.label}</option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className={labelClass}>Coordinates</span>
          <textarea
            className={`${fieldClass} min-h-28 font-mono text-xs`}
            value={text}
            onChange={(event) => {
              const next = event.target.value
              setText(next)
              try {
                onPreview(parseCoordinateText(next))
              } catch {
                if (!next.trim()) onPreview([])
              }
            }}
            placeholder={'37.7749, -122.4194\n37.7849, -122.4194\n37.7849, -122.4094'}
          />
        </label>
        {category === 'restricted_zone' && (
          <p className="text-xs text-ember">Restricted zones automatically watch every vehicle for entry and exit.</p>
        )}
        {error && <p className="text-sm text-ember">{error}</p>}
        <button className={primaryClass} disabled={saving}>
          {saving ? 'Saving…' : 'Save zone'}
        </button>
        <p className="text-xs text-mute">Use Draw on the map to drop corners. One pair per line also works, and JSON arrays are accepted.</p>
      </form>
    </div>
  )
}

function FilterChip({ active, onClick, children }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`px-3 py-1 text-xs font-semibold ${active ? 'glass-button-on' : 'glass-button glass-button-quiet text-white/70'}`}
    >
      {children}
    </button>
  )
}
