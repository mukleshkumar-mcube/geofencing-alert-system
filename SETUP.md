# Setup

Harbor runs as a Go API plus a React console. The API stores data in a SQLite file and evaluates geofences in process, so a database server is not required.

## Prerequisites

- Go 1.23 or newer: https://go.dev/dl/
- Node.js 20 or newer (the console was built with Node 24)
- Optional: Docker with Compose, for the container path

On this machine Go was unpacked to `C:\Users\ADMIN\sdk\go`. If `go version` is not recognized, add `C:\Users\ADMIN\sdk\go\bin` to your user PATH, or call `C:\Users\ADMIN\sdk\go\bin\go.exe` directly.

## Local setup

From `C:\Users\ADMIN\Downloads\geofencing-alert-system`:

```powershell
cd backend
go test ./...
go run ./cmd/server
```

The API listens on port 8080. The database file is `backend/data/geofence.db`. The first launch inserts the sample zones and vehicles described in the README. Set `SEED_DATA=false` to start empty.

In another terminal:

```powershell
cd frontend
npm install
npm run dev
```

Open http://localhost:5173. Vite proxies API and WebSocket calls to port 8080.

### Environment

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | API port |
| `DATABASE_PATH` | `data/geofence.db` | SQLite file, relative to the backend working directory |
| `SEED_DATA` | `true` | Insert the sample fleet when the database is empty |
| `API_KEY` | empty | When set, every route except `/health` requires header `X-API-Key`. The WebSocket requires `?api_key=` |
| `RATE_LIMIT_PER_MIN` | `120` | Cap for `POST /vehicles/location`, per client IP. `0` disables it |

To require a key:

```powershell
$env:API_KEY = "harbor-secret"
go run ./cmd/server
```

Paste the same value into the API key field in the console and choose Apply key.

## Docker Compose

From the project root, with Docker running:

```powershell
docker compose up --build
```

- API: http://localhost:8080
- Console: http://localhost:3000

The console image is built with `VITE_API_URL=http://localhost:8080`, so the browser talks to the published API port. SQLite lives in the `geodata` volume.

Stop it with `docker compose down`. Add `-v` to delete the database volume.

## API testing

Every JSON response includes `time_ns`, the handler duration in nanoseconds.

Health:

```powershell
curl.exe http://localhost:8080/health
```

Create a geofence. The first and last points must match, with at least three distinct corners:

```powershell
curl.exe -X POST http://localhost:8080/geofences -H "Content-Type: application/json" -d "{\"name\":\"Yard\",\"description\":\"Loading yard\",\"coordinates\":[[37.77,-122.42],[37.78,-122.42],[37.78,-122.41],[37.77,-122.41],[37.77,-122.42]],\"category\":\"delivery_zone\"}"
```

List, with an optional category filter:

```powershell
curl.exe "http://localhost:8080/geofences?category=delivery_zone"
```

Register a vehicle:

```powershell
curl.exe -X POST http://localhost:8080/vehicles -H "Content-Type: application/json" -d "{\"vehicle_number\":\"KA-05-ZZ-9999\",\"driver_name\":\"Asha Menon\",\"vehicle_type\":\"van\",\"phone\":\"+14155550999\"}"
```

Update a location. This is the call that detects entry and exit. Use a real id from the create response, or the seeded `veh_john`:

```powershell
$ts = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
curl.exe -X POST http://localhost:8080/vehicles/location -H "Content-Type: application/json" -d "{\"vehicle_id\":\"veh_john\",\"latitude\":37.7799,\"longitude\":-122.4144,\"timestamp\":\"$ts\"}"
```

Current fix:

```powershell
curl.exe http://localhost:8080/vehicles/location/veh_john
```

Alert rule. Omit `vehicle_id` to watch every vehicle. `event_type` is `entry`, `exit`, or `both`.

```powershell
curl.exe -X POST http://localhost:8080/alerts/configure -H "Content-Type: application/json" -d "{\"geofence_id\":\"geo_downtown\",\"vehicle_id\":\"veh_john\",\"event_type\":\"entry\"}"
```

History. `limit` defaults to 50 and cannot exceed 500. `offset`, `vehicle_id`, `geofence_id`, `start_date`, and `end_date` are optional.

```powershell
curl.exe "http://localhost:8080/violations/history?vehicle_id=veh_john&limit=20"
```

`scripts/demo.ps1` posts John Doe into downtown and prints the latest crossings.

WebSocket clients connect to `ws://localhost:8080/ws/alerts`. A matching rule produces a message like:

```json
{
  "event_id": "evt_ab12cd34",
  "event_type": "entry",
  "timestamp": "2025-01-15T10:35:00Z",
  "vehicle": { "vehicle_id": "veh_john", "vehicle_number": "KA-01-AB-1234", "driver_name": "John Doe" },
  "geofence": { "geofence_id": "geo_downtown", "geofence_name": "Downtown Delivery Zone", "category": "delivery_zone" },
  "location": { "latitude": 37.7799, "longitude": -122.4144 }
}
```

## Using the console

1. **Watch** — pick John Doe. Choose Pin vehicle and click downtown, or choose Downtown Delivery Zone under Run a crossing. His entry rule fires in the live feed.
2. **Zones** — choose Draw zone, click at least three corners, then Use N points. Restricted zones start watching the whole fleet on their own.
3. **Fleet** — register a vehicle. Track jumps back to Watch with that vehicle selected.
4. **Rules** — attach entry, exit, or both to a zone. Leave the vehicle blank to watch everyone. Pause a rule to keep it without live alerts, or remove it. Past crossings stay in history.
5. **History** — filter by vehicle, zone, and time. Previous and Next walk the pages.

The map legend colors match the four categories. A vehicle marker turns red while it sits inside a restricted zone. The live feed shows Live, Connecting, or Reconnecting and reconnects on its own.

## Architecture

The browser talks to one Go process.

- `cmd/server` loads config, opens SQLite, seeds when asked, and serves HTTP.
- `internal/server` is the assessment API. Each handler times itself and writes `time_ns`.
- `internal/geo` validates rings and runs the even-odd ray cast.
- `internal/store` keeps polygons, fixes, the previous inside-set, rules, and history. Candidate polygons are limited with a bounding-box index before the ray cast.
- `internal/ws` is a small hub. Location updates enqueue alert JSON and return; slow browsers drop a message instead of stalling the request.
- The React console is a single map with Watch, Zones, Fleet, Rules, and History. Leaflet draws the polygons and the vehicle markers. A WebSocket fills the live feed.

Polygons are simple rings in degree space. They do not support holes, and a ring that crosses the antimeridian will not query correctly. That is the right scope for city-sized zones like the sample set. A larger deployment would move the same API onto PostGIS, keep this ray cast as a unit-tested reference, and shard the WebSocket hub behind a queue.

## Bonus behavior included

- Rate limit on `POST /vehicles/location`
- Optional API key
- `limit` and `offset` on the list endpoints, with `total_count` on history
- Unit tests for the ray cast, validation, the HTTP flow, and WebSocket delivery
