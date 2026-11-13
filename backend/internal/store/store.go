package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"geofence/internal/model"

	_ "modernc.org/sqlite"
)

// Store persists geofences, vehicles, fixes, rules, and crossings.
type Store struct {
	db *sql.DB
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Open creates the database file, enables foreign keys, and migrates the schema.
func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	slash := filepath.ToSlash(abs)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	db, err := sql.Open("sqlite", "file://"+slash)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, err
	}
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode = WAL`).Scan(&mode); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// Close releases the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

const schema = `
CREATE TABLE IF NOT EXISTS geofences (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL,
  coordinates TEXT NOT NULL,
  min_lat REAL NOT NULL,
  max_lat REAL NOT NULL,
  min_lng REAL NOT NULL,
  max_lng REAL NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS vehicles (
  id TEXT PRIMARY KEY,
  vehicle_number TEXT NOT NULL COLLATE NOCASE UNIQUE,
  driver_name TEXT NOT NULL,
  vehicle_type TEXT NOT NULL,
  phone TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS locations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  vehicle_id TEXT NOT NULL REFERENCES vehicles(id),
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  recorded_at TEXT NOT NULL,
  ts_ms INTEGER NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS vehicle_geofence_state (
  vehicle_id TEXT NOT NULL REFERENCES vehicles(id),
  geofence_id TEXT NOT NULL REFERENCES geofences(id),
  inside INTEGER NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (vehicle_id, geofence_id)
);

CREATE TABLE IF NOT EXISTS alert_rules (
  id TEXT PRIMARY KEY,
  geofence_id TEXT NOT NULL REFERENCES geofences(id),
  vehicle_id TEXT REFERENCES vehicles(id),
  event_type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS violations (
  id TEXT PRIMARY KEY,
  vehicle_id TEXT NOT NULL REFERENCES vehicles(id),
  geofence_id TEXT NOT NULL REFERENCES geofences(id),
  event_type TEXT NOT NULL,
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  timestamp TEXT NOT NULL,
  ts_ms INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_geofences_bbox ON geofences (min_lat, max_lat, min_lng, max_lng);
CREATE INDEX IF NOT EXISTS idx_locations_vehicle ON locations (vehicle_id, ts_ms DESC);
CREATE INDEX IF NOT EXISTS idx_state_vehicle ON vehicle_geofence_state (vehicle_id, inside);
CREATE INDEX IF NOT EXISTS idx_violations_ts ON violations (ts_ms DESC);
CREATE INDEX IF NOT EXISTS idx_violations_vehicle ON violations (vehicle_id, ts_ms DESC);
CREATE INDEX IF NOT EXISTS idx_violations_geofence ON violations (geofence_id, ts_ms DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_geofence ON alert_rules (geofence_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_identity
  ON alert_rules (geofence_id, ifnull(vehicle_id, ''), event_type);
`

func newID(prefix string) string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return prefix + hex.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return prefix + hex.EncodeToString(buf[:])
}

func formatTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func decodeCoords(raw string) ([][]float64, error) {
	var coords [][]float64
	if err := json.Unmarshal([]byte(raw), &coords); err != nil {
		return nil, fmt.Errorf("decode coordinates: %w", err)
	}
	return coords, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanGeofence(sc scanner) (model.Geofence, error) {
	var g model.Geofence
	var raw string
	if err := sc.Scan(&g.ID, &g.Name, &g.Description, &raw, &g.Category, &g.CreatedAt); err != nil {
		return g, err
	}
	coords, err := decodeCoords(raw)
	if err != nil {
		return g, err
	}
	g.Coordinates = coords
	return g, nil
}

func scanVehicle(sc scanner) (model.Vehicle, error) {
	var v model.Vehicle
	err := sc.Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Status, &v.CreatedAt)
	return v, err
}

func notFound(err, kind error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return kind
	}
	return err
}
