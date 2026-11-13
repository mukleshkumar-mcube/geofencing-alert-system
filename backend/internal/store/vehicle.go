package store

import (
	"context"
	"time"

	"geofence/internal/model"
)

// CreateVehicle registers a vehicle. Numbers are unique regardless of letter case.
func (s *Store) CreateVehicle(ctx context.Context, number, driver, kind, phone string) (model.Vehicle, error) {
	now := formatTime(time.Now())
	v := model.Vehicle{
		ID:            newID("veh_"),
		VehicleNumber: number,
		DriverName:    driver,
		VehicleType:   kind,
		Phone:         phone,
		Status:        "active",
		CreatedAt:     now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO vehicles (id, vehicle_number, driver_name, vehicle_type, phone, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'active', ?)
	`, v.ID, v.VehicleNumber, v.DriverName, v.VehicleType, v.Phone, v.CreatedAt)
	if isUnique(err) {
		return model.Vehicle{}, ErrConflict
	}
	if err != nil {
		return model.Vehicle{}, err
	}
	return v, nil
}

// ListVehicles returns one page of vehicles and the unpaged total, newest first.
func (s *Store) ListVehicles(ctx context.Context, limit, offset int) ([]model.Vehicle, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vehicles`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at
		FROM vehicles
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]model.Vehicle, 0)
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (s *Store) getVehicle(ctx context.Context, q queryer, id string) (model.Vehicle, error) {
	row := q.QueryRowContext(ctx, `
		SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at
		FROM vehicles WHERE id = ?
	`, id)
	v, err := scanVehicle(row)
	if err != nil {
		return model.Vehicle{}, notFound(err, ErrVehicleNotFound)
	}
	return v, nil
}

// VehicleSnapshot returns the vehicle, its latest fix, and the zones it is inside.
// The fix is nil when the vehicle has never reported a location.
func (s *Store) VehicleSnapshot(ctx context.Context, id string) (model.Vehicle, *model.LocationFix, []model.Geofence, error) {
	vehicle, err := s.getVehicle(ctx, s.db, id)
	if err != nil {
		return model.Vehicle{}, nil, nil, err
	}

	var fix *model.LocationFix
	var lat, lng float64
	var recorded string
	err = s.db.QueryRowContext(ctx, `
		SELECT latitude, longitude, recorded_at
		FROM locations
		WHERE vehicle_id = ?
		ORDER BY ts_ms DESC, id DESC
		LIMIT 1
	`, id).Scan(&lat, &lng, &recorded)
	if err == nil {
		fix = &model.LocationFix{Latitude: lat, Longitude: lng, Timestamp: recorded}
	} else if mapped := notFound(err, nil); mapped != nil {
		return model.Vehicle{}, nil, nil, mapped
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.description, g.coordinates, g.category, g.created_at
		FROM vehicle_geofence_state st
		JOIN geofences g ON g.id = st.geofence_id
		WHERE st.vehicle_id = ? AND st.inside = 1
		ORDER BY g.name
	`, id)
	if err != nil {
		return model.Vehicle{}, nil, nil, err
	}
	defer rows.Close()

	inside := make([]model.Geofence, 0)
	for rows.Next() {
		g, err := scanGeofence(rows)
		if err != nil {
			return model.Vehicle{}, nil, nil, err
		}
		inside = append(inside, g)
	}
	return vehicle, fix, inside, rows.Err()
}
