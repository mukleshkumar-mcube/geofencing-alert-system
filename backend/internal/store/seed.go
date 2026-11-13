package store

import (
	"context"
	"encoding/json"
	"time"
)

// Seed inserts a San Francisco sample fleet the first time the database is empty.
// Identifiers are stable so the setup guide and curl examples can refer to them.
// The sample fix for Priya is already inside the toll zone, and history stays
// empty so the first live crossing is one the operator triggers.
func (s *Store) Seed(ctx context.Context) (bool, error) {
	var fences, vehicles int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM geofences`).Scan(&fences); err != nil {
		return false, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vehicles`).Scan(&vehicles); err != nil {
		return false, err
	}
	if fences > 0 || vehicles > 0 {
		return false, nil
	}

	type fence struct {
		id, name, description, category string
		coords                           [][]float64
	}
	samples := []fence{
		{
			id: "geo_downtown", name: "Downtown Delivery Zone",
			description: "Main delivery area for downtown customers",
			category:    "delivery_zone",
			coords: [][]float64{
				{37.7749, -122.4194},
				{37.7849, -122.4194},
				{37.7849, -122.4094},
				{37.7749, -122.4094},
				{37.7749, -122.4194},
			},
		},
		{
			id: "geo_port", name: "Port Restricted Zone",
			description: "Active terminal. Entry and exit are raised immediately.",
			category:    "restricted_zone",
			coords: [][]float64{
				{37.7950, -122.4100},
				{37.8040, -122.4100},
				{37.8040, -122.3960},
				{37.7950, -122.3960},
				{37.7950, -122.4100},
			},
		},
		{
			id: "geo_toll", name: "Bay Toll Corridor",
			description: "Toll approach running along the south of downtown.",
			category:    "toll_zone",
			coords: [][]float64{
				{37.7690, -122.4300},
				{37.7730, -122.4300},
				{37.7730, -122.4000},
				{37.7690, -122.4000},
				{37.7690, -122.4300},
			},
		},
		{
			id: "geo_customer", name: "North Dock Customers",
			description: "Priority customer docks north-west of downtown.",
			category:    "customer_area",
			coords: [][]float64{
				{37.7860, -122.4300},
				{37.7940, -122.4300},
				{37.7940, -122.4160},
				{37.7860, -122.4160},
				{37.7860, -122.4300},
			},
		},
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	base := time.Now().UTC().Truncate(time.Second)
	for i, sample := range samples {
		raw, err := json.Marshal(sample.coords)
		if err != nil {
			return false, err
		}
		minLat, maxLat, minLng, maxLng := boundsOf(sample.coords)
		created := formatTime(base.Add(time.Duration(i) * time.Second))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO geofences (
				id, name, description, category, coordinates,
				min_lat, max_lat, min_lng, max_lng, status, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)
		`, sample.id, sample.name, sample.description, sample.category, string(raw), minLat, maxLat, minLng, maxLng, created); err != nil {
			return false, err
		}
	}

	createdVeh := formatTime(base)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO vehicles (id, vehicle_number, driver_name, vehicle_type, phone, status, created_at)
		VALUES
		('veh_john', 'KA-01-AB-1234', 'John Doe', 'truck', '+14155550100', 'active', ?),
		('veh_priya', 'MH-12-CD-7781', 'Priya Nair', 'van', '+14155550198', 'active', ?)
	`, createdVeh, createdVeh); err != nil {
		return false, err
	}

	if err := insertAlert(ctx, tx, "alert_downtown", "geo_downtown", strPtr("veh_john"), "entry", createdVeh); err != nil {
		return false, err
	}
	if err := insertAlert(ctx, tx, "alert_port", "geo_port", nil, "both", createdVeh); err != nil {
		return false, err
	}

	johnAt := formatTime(base)
	priyaAt := formatTime(base)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO locations (vehicle_id, latitude, longitude, recorded_at, ts_ms, created_at)
		VALUES
		('veh_john', 37.7799, -122.4280, ?, ?, ?),
		('veh_priya', 37.7708, -122.4150, ?, ?, ?)
	`, johnAt, base.UnixMilli(), johnAt, priyaAt, base.UnixMilli(), priyaAt); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO vehicle_geofence_state (vehicle_id, geofence_id, inside, updated_at)
		VALUES ('veh_priya', 'geo_toll', 1, ?)
	`, priyaAt); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func strPtr(v string) *string { return &v }

func boundsOf(coords [][]float64) (minLat, maxLat, minLng, maxLng float64) {
	minLat, maxLat = coords[0][0], coords[0][0]
	minLng, maxLng = coords[0][1], coords[0][1]
	for _, p := range coords[1:] {
		if p[0] < minLat {
			minLat = p[0]
		}
		if p[0] > maxLat {
			maxLat = p[0]
		}
		if p[1] < minLng {
			minLng = p[1]
		}
		if p[1] > maxLng {
			maxLng = p[1]
		}
	}
	return minLat, maxLat, minLng, maxLng
}
