export function pointInPolygon(lat, lng, ring) {
  let n = ring.length
  if (n === 0) return false
  if (n > 1 && ring[0][0] === ring[n - 1][0] && ring[0][1] === ring[n - 1][1]) n -= 1
  if (n < 3) return false
  let inside = false
  let j = n - 1
  for (let i = 0; i < n; i += 1) {
    const yi = ring[i][0]
    const xi = ring[i][1]
    const yj = ring[j][0]
    const xj = ring[j][1]
    if ((yi > lat) !== (yj > lat)) {
      const xIntersect = ((xj - xi) * (lat - yi)) / (yj - yi) + xi
      if (lng < xIntersect) inside = !inside
    }
    j = i
  }
  return inside
}

export function polygonCentroid(ring) {
  const pts = openRing(ring)
  if (pts.length === 0) return [0, 0]
  let area = 0
  let cx = 0
  let cy = 0
  for (let i = 0; i < pts.length; i += 1) {
    const j = (i + 1) % pts.length
    const cross = pts[i][1] * pts[j][0] - pts[j][1] * pts[i][0]
    area += cross
    cy += (pts[i][0] + pts[j][0]) * cross
    cx += (pts[i][1] + pts[j][1]) * cross
  }
  area *= 0.5
  if (Math.abs(area) < 1e-12) {
    const lat = pts.reduce((sum, p) => sum + p[0], 0) / pts.length
    const lng = pts.reduce((sum, p) => sum + p[1], 0) / pts.length
    return [lat, lng]
  }
  return [cy / (6 * area), cx / (6 * area)]
}

export function parseCoordinateText(text) {
  const trimmed = text.trim()
  if (!trimmed) return []
  if (trimmed.startsWith('[')) {
    const data = JSON.parse(trimmed)
    if (!Array.isArray(data)) throw new Error('Coordinates must be an array of [lat, lng] pairs.')
    return data.map((pair, index) => readPair(pair, index))
  }
  const lines = trimmed.split(/\n+/).map((line) => line.trim()).filter(Boolean)
  const points = []
  lines.forEach((line, index) => {
    const parts = line.split(/[,\s]+/).filter(Boolean)
    if (parts.length < 2) {
      if (index === lines.length - 1) return
      throw new Error(`Could not read a latitude and longitude from "${line}".`)
    }
    points.push(readPair([Number(parts[0]), Number(parts[1])], index))
  })
  return points
}

export function closeRing(points) {
  if (points.length === 0) return points
  const first = points[0]
  const last = points[points.length - 1]
  if (first[0] === last[0] && first[1] === last[1]) return points
  return [...points, [first[0], first[1]]]
}

function openRing(ring) {
  if (ring.length > 1 && ring[0][0] === ring[ring.length - 1][0] && ring[0][1] === ring[ring.length - 1][1]) {
    return ring.slice(0, -1)
  }
  return ring
}

function readPair(pair, index) {
  if (!Array.isArray(pair) || pair.length < 2) {
    throw new Error(`Point ${index + 1} needs a latitude and a longitude.`)
  }
  const lat = Number(pair[0])
  const lng = Number(pair[1])
  if (!Number.isFinite(lat) || !Number.isFinite(lng)) {
    throw new Error(`Point ${index + 1} must be numeric.`)
  }
  return [lat, lng]
}
