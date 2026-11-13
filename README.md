# Harbor — Geofence Watch

Harbor is a vehicle geofencing desk. Operators draw zones on a map, register a fleet, post live positions, and receive entry and exit alerts the moment a watched vehicle crosses a boundary.

The project is built from the MapUp full-stack assessment use cases:

- Create and list polygonal geofences (`delivery_zone`, `restricted_zone`, `toll_zone`, `customer_area`)
- Register and list vehicles
- Update a vehicle fix and learn which zones contain it
- Read a vehicle's current fix and zone membership
- Configure alert rules for entry, exit, or both, for one vehicle or the whole fleet
- Store every crossing and browse it with filters
- Push matching alerts to the browser over a WebSocket

## Layout

```
backend/     Go API, SQLite, WebSocket hub
frontend/    React console (Vite, Tailwind, Leaflet)
docker-compose.yml
SETUP.md     install, curl examples, and how to use the console
```

## Quick start

Go 1.23+ and Node 20+ are required. From this folder:

```powershell
cd backend
go run ./cmd/server
```

In a second terminal:

```powershell
cd frontend
npm install
npm run dev
```

Open http://localhost:5173. The API listens on http://localhost:8080 and loads a San Francisco sample fleet the first time the database is empty.

Docker Compose is the other path. See [SETUP.md](SETUP.md).

## How a crossing is decided

1. `POST /vehicles/location` stores the fix.
2. A bounding-box index selects candidate polygons.
3. `internal/geo.PointInPolygon` ray-casts the point.
4. The result is compared with the vehicle's previous inside-set.
5. Every entry and exit is written to history.
6. If an alert rule matches, the event is broadcast on `/ws/alerts` without blocking the HTTP response.

Restricted zones are special: creating one also creates a fleet-wide entry-and-exit rule, so those areas alert immediately.

## Sample story

| Id | What it is |
| --- | --- |
| `geo_downtown` | Downtown Delivery Zone |
| `geo_port` | Port Restricted Zone, watched for every vehicle |
| `geo_toll` | Bay Toll Corridor |
| `geo_customer` | North Dock Customers |
| `veh_john` | KA-01-AB-1234, John Doe, parked west of downtown |
| `veh_priya` | MH-12-CD-7781, Priya Nair, already inside the toll corridor |
| `alert_downtown` | John Doe entering downtown |
| `alert_port` | Any vehicle entering or leaving the port |

## Tests

```powershell
cd backend
go test ./...
```

## Before you submit the assessment

- Demo video link: add your unlisted URL here
- Repository: https://github.com/mukleshkumar-mcube/geofencing-alert-system (private)
- Backend image: https://hub.docker.com/r/muklesh/geofencing-alert-backend (`docker pull muklesh/geofencing-alert-backend`)
