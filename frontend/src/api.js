const BASE = import.meta.env.VITE_API_URL || ''

export async function api(path, { method = 'GET', body } = {}) {
  const headers = {}
  const key = localStorage.getItem('harbor.apiKey') || ''
  if (key) headers['X-API-Key'] = key
  let payload
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }

  let response
  try {
    response = await fetch(BASE + path, { method, headers, body: payload })
  } catch {
    throw new Error('Cannot reach the API. Start the backend on port 8080.')
  }

  const text = await response.text()
  let data = {}
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { error: text.slice(0, 180) }
    }
  }
  if (!response.ok) {
    throw new Error(data.error || `Request failed (${response.status})`)
  }
  return data
}

export function websocketURL() {
  const key = localStorage.getItem('harbor.apiKey') || ''
  if (BASE) {
    const url = new URL(BASE)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    url.pathname = '/ws/alerts'
    url.search = key ? `api_key=${encodeURIComponent(key)}` : ''
    return url.toString()
  }
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const query = key ? `?api_key=${encodeURIComponent(key)}` : ''
  return `${proto}://${window.location.host}/ws/alerts${query}`
}
