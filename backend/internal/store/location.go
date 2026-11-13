package store

import (
	"context"
	"sort"
	"time"

	"geofence/internal/geo"
	"geofence/internal/model"
)

// Transition is an entry or exit discovered by comparing the new fix
// with the vehicle's previous inside-set.
type Transition struct {
	EventType string
	Geofence  model.Geofence
	Vehicle   model.Vehicle
	Latitude  float64
	Longitude float64
	Timestamp time.Time
}

// LocationOutcome is the result of saving a fix.
type LocationOutcome struct {
	Inside []model.Geofence
	Events []Transition
}

// ApplyLocation stores the fix, diffs geofence membership, and records every
// entry and exit. Callers decide which of those events should be broadcast.
func (s *Store) ApplyLocation(ctx context.Context, vehicleID string, lat, lng float64, ts time.Time) (LocationOutcome, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LocationOutcome{}, err
	}
	defer tx.Rollback()

	vehicle, err := s.getVehicle(ctx, tx, vehicleID)
	if err != nil {
		return LocationOutcome{}, err
	}

	recorded := formatTime(ts)
	now := formatTime(time.Now())
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO locations (vehicle_id, latitude, longitude, recorded_at, ts_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, vehicleID, lat, lng, recorded, ts.UTC().UnixMilli(), now); err != nil {
		return LocationOutcome{}, err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, name, description, coordinates, category, created_at
		FROM geofences
		WHERE min_lat <= ? AND max_lat >= ? AND min_lng <= ? AND max_lng >= ?
	`, lat, lat, lng, lng)
	if err != nil {
		return LocationOutcome{}, err
	}

	inside := make([]model.Geofence, 0)
	insideSet := make(map[string]model.Geofence)
	for rows.Next() {
		g, err := scanGeofence(rows)
		if err != nil {
			rows.Close()
			return LocationOutcome{}, err
		}
		if geo.PointInPolygon(lat, lng, g.Coordinates) {
			inside = append(inside, g)
			insideSet[g.ID] = g
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return LocationOutcome{}, err
	}
	rows.Close()

	prevRows, err := tx.QueryContext(ctx, `
		SELECT geofence_id FROM vehicle_geofence_state
		WHERE vehicle_id = ? AND inside = 1
	`, vehicleID)
	if err != nil {
		return LocationOutcome{}, err
	}
	previous := make(map[string]struct{})
	for prevRows.Next() {
		var id string
		if err := prevRows.Scan(&id); err != nil {
			prevRows.Close()
			return LocationOutcome{}, err
		}
		previous[id] = struct{}{}
	}
	if err := prevRows.Err(); err != nil {
		prevRows.Close()
		return LocationOutcome{}, err
	}
	prevRows.Close()

	events := make([]Transition, 0)
	for id, g := range insideSet {
		if _, wasInside := previous[id]; wasInside {
			continue
		}
		if err := s.writeCrossing(ctx, tx, vehicle, g, "entry", lat, lng, ts, recorded); err != nil {
			return LocationOutcome{}, err
		}
		events = append(events, Transition{
			EventType: "entry",
			Geofence:  g,
			Vehicle:   vehicle,
			Latitude:  lat,
			Longitude: lng,
			Timestamp: ts.UTC(),
		})
	}
	for id := range previous {
		if _, still := insideSet[id]; still {
			continue
		}
		g, err := loadGeofence(ctx, tx, id)
		if err != nil {
			return LocationOutcome{}, err
		}
		if err := s.writeCrossing(ctx, tx, vehicle, g, "exit", lat, lng, ts, recorded); err != nil {
			return LocationOutcome{}, err
		}
		events = append(events, Transition{
			EventType: "exit",
			Geofence:  g,
			Vehicle:   vehicle,
			Latitude:  lat,
			Longitude: lng,
			Timestamp: ts.UTC(),
		})
	}

	sort.Slice(inside, func(i, j int) bool { return inside[i].Name < inside[j].Name })
	sort.Slice(events, func(i, j int) bool {
		if events[i].EventType != events[j].EventType {
			return events[i].EventType < events[j].EventType
		}
		return events[i].Geofence.Name < events[j].Geofence.Name
	})

	if err := tx.Commit(); err != nil {
		return LocationOutcome{}, err
	}
	return LocationOutcome{Inside: inside, Events: events}, nil
}

func (s *Store) writeCrossing(ctx context.Context, tx queryer, vehicle model.Vehicle, g model.Geofence, eventType string, lat, lng float64, ts time.Time, recorded string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO vehicle_geofence_state (vehicle_id, geofence_id, inside, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(vehicle_id, geofence_id) DO UPDATE SET
			inside = excluded.inside,
			updated_at = excluded.updated_at
	`, vehicle.ID, g.ID, boolToInt(eventType == "entry"), recorded); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO violations (
			id, vehicle_id, geofence_id, event_type, latitude, longitude, timestamp, ts_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, newID("viol_"), vehicle.ID, g.ID, eventType, lat, lng, recorded, ts.UTC().UnixMilli())
	return err
}

func loadGeofence(ctx context.Context, q queryer, id string) (model.Geofence, error) {
	row := q.QueryRowContext(ctx, `
		SELECT id, name, description, coordinates, category, created_at
		FROM geofences WHERE id = ?
	`, id)
	g, err := scanGeofence(row)
	if err != nil {
		return model.Geofence{}, notFound(err, ErrGeofenceNotFound)
	}
	return g, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
