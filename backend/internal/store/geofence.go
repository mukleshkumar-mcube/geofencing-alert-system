package store

import (
	"context"
	"encoding/json"
	"time"

	"geofence/internal/geo"
	"geofence/internal/model"
)

// CreateGeofence stores a polygon. Restricted zones also receive a fleet-wide
// entry/exit rule so those areas alert without a separate setup step.
func (s *Store) CreateGeofence(ctx context.Context, name, description, category string, coords [][]float64) (model.Geofence, error) {
	raw, err := json.Marshal(coords)
	if err != nil {
		return model.Geofence{}, err
	}
	box := geo.BoundsOf(coords)
	minLat, maxLat, minLng, maxLng := box.MinLat, box.MaxLat, box.MinLng, box.MaxLng
	now := formatTime(time.Now())
	id := newID("geo_")

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Geofence{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO geofences (
			id, name, description, category, coordinates,
			min_lat, max_lat, min_lng, max_lng, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)
	`, id, name, description, category, string(raw), minLat, maxLat, minLng, maxLng, now)
	if err != nil {
		return model.Geofence{}, err
	}
	if category == "restricted_zone" {
		if err := insertAlert(ctx, tx, newID("alert_"), id, nil, "both", now); err != nil {
			return model.Geofence{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Geofence{}, err
	}
	return model.Geofence{
		ID:          id,
		Name:        name,
		Description: description,
		Coordinates: coords,
		Category:    category,
		CreatedAt:   now,
	}, nil
}

// ListGeofences returns one page of geofences and the unpaged total.
func (s *Store) ListGeofences(ctx context.Context, category string, limit, offset int) ([]model.Geofence, int, error) {
	where := ` WHERE 1=1`
	args := make([]any, 0, 3)
	if category != "" {
		where += ` AND category = ?`
		args = append(args, category)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM geofences`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT id, name, description, coordinates, category, created_at FROM geofences` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]model.Geofence, 0)
	for rows.Next() {
		g, err := scanGeofence(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}

func (s *Store) geofenceExists(ctx context.Context, q queryer, id string) error {
	var found string
	err := q.QueryRowContext(ctx, `SELECT id FROM geofences WHERE id = ?`, id).Scan(&found)
	return notFound(err, ErrGeofenceNotFound)
}
