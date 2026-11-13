package store

import (
	"context"
	"database/sql"
	"time"

	"geofence/internal/model"
)

// CreateAlert stores a rule. A nil vehicleID watches every vehicle.
func (s *Store) CreateAlert(ctx context.Context, geofenceID string, vehicleID *string, eventType string) (model.AlertRule, error) {
	if err := s.geofenceExists(ctx, s.db, geofenceID); err != nil {
		return model.AlertRule{}, err
	}
	if vehicleID != nil {
		if _, err := s.getVehicle(ctx, s.db, *vehicleID); err != nil {
			return model.AlertRule{}, err
		}
	}
	now := formatTime(time.Now())
	id := newID("alert_")
	if err := insertAlert(ctx, s.db, id, geofenceID, vehicleID, eventType, now); err != nil {
		return model.AlertRule{}, err
	}
	return s.getAlert(ctx, id)
}

func insertAlert(ctx context.Context, q queryer, id, geofenceID string, vehicleID *string, eventType, now string) error {
	var vehicle any
	if vehicleID != nil && *vehicleID != "" {
		vehicle = *vehicleID
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO alert_rules (id, geofence_id, vehicle_id, event_type, status, created_at)
		VALUES (?, ?, ?, ?, 'active', ?)
	`, id, geofenceID, vehicle, eventType, now)
	if isUnique(err) {
		return ErrConflict
	}
	return err
}

func (s *Store) getAlert(ctx context.Context, id string) (model.AlertRule, error) {
	row := s.db.QueryRowContext(ctx, alertSelect+` WHERE a.id = ?`, id)
	return scanAlert(row)
}

// ListAlerts returns one page of rules and the unpaged total.
func (s *Store) ListAlerts(ctx context.Context, geofenceID, vehicleID string, limit, offset int) ([]model.AlertRule, int, error) {
	where := ` WHERE 1=1`
	args := make([]any, 0, 4)
	if geofenceID != "" {
		where += ` AND a.geofence_id = ?`
		args = append(args, geofenceID)
	}
	if vehicleID != "" {
		where += ` AND a.vehicle_id = ?`
		args = append(args, vehicleID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alert_rules a`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := alertSelect + where + ` ORDER BY a.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]model.AlertRule, 0)
	for rows.Next() {
		rule, err := scanAlert(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, rule)
	}
	return out, total, rows.Err()
}

// SetAlertStatus marks a rule active or inactive. Inactive rules stay in the list and do not broadcast.
func (s *Store) SetAlertStatus(ctx context.Context, id, status string) (model.AlertRule, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE alert_rules SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return model.AlertRule{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return model.AlertRule{}, err
	}
	if n == 0 {
		return model.AlertRule{}, ErrAlertNotFound
	}
	return s.getAlert(ctx, id)
}

// DeleteAlert removes a rule. Historical crossings stay in the violations table.
func (s *Store) DeleteAlert(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAlertNotFound
	}
	return nil
}

// HasMatchingRule reports whether any active rule wants this crossing broadcast.
func (s *Store) HasMatchingRule(ctx context.Context, geofenceID, vehicleID, eventType string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM alert_rules
		WHERE status = 'active'
		  AND geofence_id = ?
		  AND (vehicle_id IS NULL OR vehicle_id = ?)
		  AND (event_type = ? OR event_type = 'both')
	`, geofenceID, vehicleID, eventType).Scan(&n)
	return n > 0, err
}

const alertSelect = `
SELECT a.id, a.geofence_id, g.name, a.vehicle_id, v.vehicle_number, a.event_type, a.status, a.created_at
FROM alert_rules a
JOIN geofences g ON g.id = a.geofence_id
LEFT JOIN vehicles v ON v.id = a.vehicle_id
`

func scanAlert(sc scanner) (model.AlertRule, error) {
	var rule model.AlertRule
	var vehicleID, vehicleNumber sql.NullString
	err := sc.Scan(
		&rule.AlertID,
		&rule.GeofenceID,
		&rule.GeofenceName,
		&vehicleID,
		&vehicleNumber,
		&rule.EventType,
		&rule.Status,
		&rule.CreatedAt,
	)
	if err != nil {
		return rule, err
	}
	if vehicleID.Valid {
		rule.VehicleID = &vehicleID.String
	}
	if vehicleNumber.Valid {
		rule.VehicleNumber = &vehicleNumber.String
	}
	return rule, nil
}

// ViolationFilter narrows the crossing history.
type ViolationFilter struct {
	VehicleID  string
	GeofenceID string
	StartMS    *int64
	EndMS      *int64
	Limit      int
	Offset     int
}

// ListViolations returns a page of crossings and the unpaged total.
func (s *Store) ListViolations(ctx context.Context, f ViolationFilter) ([]model.Violation, int, error) {
	where := ` WHERE 1=1`
	args := make([]any, 0, 6)
	if f.VehicleID != "" {
		where += ` AND v.vehicle_id = ?`
		args = append(args, f.VehicleID)
	}
	if f.GeofenceID != "" {
		where += ` AND v.geofence_id = ?`
		args = append(args, f.GeofenceID)
	}
	if f.StartMS != nil {
		where += ` AND v.ts_ms >= ?`
		args = append(args, *f.StartMS)
	}
	if f.EndMS != nil {
		where += ` AND v.ts_ms <= ?`
		args = append(args, *f.EndMS)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM violations v`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `
		SELECT v.id, v.vehicle_id, ve.vehicle_number, v.geofence_id, g.name,
		       v.event_type, v.latitude, v.longitude, v.timestamp
		FROM violations v
		JOIN vehicles ve ON ve.id = v.vehicle_id
		JOIN geofences g ON g.id = v.geofence_id
	` + where + ` ORDER BY v.ts_ms DESC, v.id DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]model.Violation, 0)
	for rows.Next() {
		var item model.Violation
		if err := rows.Scan(
			&item.ID, &item.VehicleID, &item.VehicleNumber, &item.GeofenceID, &item.GeofenceName,
			&item.EventType, &item.Latitude, &item.Longitude, &item.Timestamp,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}
