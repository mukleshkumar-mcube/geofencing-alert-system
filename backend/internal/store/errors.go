package store

import "errors"

var (
	// ErrVehicleNotFound is returned when a vehicle id does not exist.
	ErrVehicleNotFound = errors.New("vehicle not found")
	// ErrGeofenceNotFound is returned when a geofence id does not exist.
	ErrGeofenceNotFound = errors.New("geofence not found")
	// ErrAlertNotFound is returned when an alert rule id does not exist.
	ErrAlertNotFound = errors.New("alert not found")
	// ErrConflict is returned when a unique vehicle number or alert rule already exists.
	ErrConflict = errors.New("conflict")
)
