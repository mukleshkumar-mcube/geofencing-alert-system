package model

// Geofence is a named polygonal zone. Bounds are stored for the SQL prefilter
// and are not part of the API payload.
type Geofence struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
	CreatedAt   string      `json:"created_at"`
	MinLat      float64     `json:"-"`
	MaxLat      float64     `json:"-"`
	MinLng      float64     `json:"-"`
	MaxLng      float64     `json:"-"`
}

// Vehicle is a registered fleet unit.
type Vehicle struct {
	ID            string `json:"id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

// LocationFix is the latest recorded position of a vehicle.
type LocationFix struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp string  `json:"timestamp"`
}

// AlertRule is a saved watch on entry, exit, or both.
type AlertRule struct {
	AlertID       string  `json:"alert_id"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	VehicleID     *string `json:"vehicle_id"`
	VehicleNumber *string `json:"vehicle_number"`
	EventType     string  `json:"event_type"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
}

// Violation is one recorded entry or exit. The API name is historical;
// every crossing is stored, including ordinary delivery zones.
type Violation struct {
	ID            string  `json:"id"`
	VehicleID     string  `json:"vehicle_id"`
	VehicleNumber string  `json:"vehicle_number"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	EventType     string  `json:"event_type"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Timestamp     string  `json:"timestamp"`
}

// Transition is an entry or exit detected while applying a location fix.
type Transition struct {
	EventType string
	Geofence  Geofence
	Latitude  float64
	Longitude float64
	Timestamp string
}

// ValidCategory reports whether category is one of the four supported kinds.
func ValidCategory(category string) bool {
	switch category {
	case "delivery_zone", "restricted_zone", "toll_zone", "customer_area":
		return true
	default:
		return false
	}
}

// ValidEventType reports whether eventType is entry, exit, or both.
func ValidEventType(eventType string) bool {
	switch eventType {
	case "entry", "exit", "both":
		return true
	default:
		return false
	}
}

// AlertEvent is the payload pushed to connected WebSocket clients.
type AlertEvent struct {
	EventID   string       `json:"event_id"`
	EventType string       `json:"event_type"`
	Timestamp string       `json:"timestamp"`
	Vehicle   AlertVehicle `json:"vehicle"`
	Geofence  AlertFence   `json:"geofence"`
	Location  AlertPoint   `json:"location"`
}

// AlertVehicle identifies the unit that crossed a boundary.
type AlertVehicle struct {
	VehicleID     string `json:"vehicle_id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
}

// AlertFence identifies the zone that was crossed.
type AlertFence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

// AlertPoint is the fix that caused the alert.
type AlertPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
