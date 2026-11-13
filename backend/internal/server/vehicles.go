package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"

	"geofence/internal/model"
	"geofence/internal/store"
)

func (s *Server) createVehicle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req struct {
		VehicleNumber string `json:"vehicle_number"`
		DriverName    string `json:"driver_name"`
		VehicleType   string `json:"vehicle_type"`
		Phone         string `json:"phone"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	number := strings.TrimSpace(req.VehicleNumber)
	driver := strings.TrimSpace(req.DriverName)
	kind := strings.TrimSpace(req.VehicleType)
	phone := strings.TrimSpace(req.Phone)
	switch {
	case !validVehicleNumber(number):
		fail(w, start, http.StatusBadRequest, "vehicle_number must be 3–32 characters and use letters, numbers, spaces, or hyphens")
		return
	case driver == "" || len([]rune(driver)) > 120:
		fail(w, start, http.StatusBadRequest, "driver_name is required and must be at most 120 characters")
		return
	case kind == "" || len([]rune(kind)) > 40:
		fail(w, start, http.StatusBadRequest, "vehicle_type is required and must be at most 40 characters")
		return
	case !validPhone(phone):
		fail(w, start, http.StatusBadRequest, "phone must include at least 7 digits and may contain +, spaces, and dashes")
		return
	}

	v, err := s.store.CreateVehicle(r.Context(), number, driver, kind, phone)
	if errors.Is(err, store.ErrConflict) {
		fail(w, start, http.StatusConflict, "a vehicle with this number is already registered")
		return
	}
	if err != nil {
		log.Printf("create vehicle: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not register vehicle")
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		ID            string `json:"id"`
		VehicleNumber string `json:"vehicle_number"`
		Status        string `json:"status"`
		TimeNS        string `json:"time_ns"`
	}{ID: v.ID, VehicleNumber: v.VehicleNumber, Status: v.Status, TimeNS: ns(start)})
}

func (s *Server) listVehicles(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	limit, offset, err := pageParams(r, 500, 500)
	if err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := s.store.ListVehicles(r.Context(), limit, offset)
	if err != nil {
		log.Printf("list vehicles: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not list vehicles")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Vehicles   []model.Vehicle `json:"vehicles"`
		TotalCount int             `json:"total_count"`
		TimeNS     string          `json:"time_ns"`
	}{Vehicles: items, TotalCount: total, TimeNS: ns(start)})
}

func (s *Server) updateLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !s.limiter.Allow(clientIP(r)) {
		fail(w, start, http.StatusTooManyRequests, "too many location updates, slow down and try again")
		return
	}
	var req struct {
		VehicleID string   `json:"vehicle_id"`
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
		Timestamp string   `json:"timestamp"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	vehicleID := strings.TrimSpace(req.VehicleID)
	if vehicleID == "" {
		fail(w, start, http.StatusBadRequest, "vehicle_id is required")
		return
	}
	if req.Latitude == nil || req.Longitude == nil {
		fail(w, start, http.StatusBadRequest, "latitude and longitude are required")
		return
	}
	if *req.Latitude < -90 || *req.Latitude > 90 {
		fail(w, start, http.StatusBadRequest, "latitude must be between -90 and 90")
		return
	}
	if *req.Longitude < -180 || *req.Longitude > 180 {
		fail(w, start, http.StatusBadRequest, "longitude must be between -180 and 180")
		return
	}
	ts, err := parseTimestamp(req.Timestamp)
	if err != nil {
		fail(w, start, http.StatusBadRequest, "timestamp must be ISO 8601, for example 2025-01-15T10:35:00Z")
		return
	}

	outcome, err := s.store.ApplyLocation(r.Context(), vehicleID, *req.Latitude, *req.Longitude, ts)
	if errors.Is(err, store.ErrVehicleNotFound) {
		fail(w, start, http.StatusNotFound, "vehicle not found")
		return
	}
	if err != nil {
		log.Printf("update location: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not update location")
		return
	}

	payloads := make([][]byte, 0, len(outcome.Events))
	for _, ev := range outcome.Events {
		match, err := s.store.HasMatchingRule(r.Context(), ev.Geofence.ID, ev.Vehicle.ID, ev.EventType)
		if err != nil {
			log.Printf("match alert rule: %v", err)
			fail(w, start, http.StatusInternalServerError, "could not evaluate alert rules")
			return
		}
		if !match {
			continue
		}
		body, err := json.Marshal(model.AlertEvent{
			EventID:   eventID(),
			EventType: ev.EventType,
			Timestamp: ev.Timestamp.UTC().Truncate(time.Second).Format(time.RFC3339),
			Vehicle: model.AlertVehicle{
				VehicleID:     ev.Vehicle.ID,
				VehicleNumber: ev.Vehicle.VehicleNumber,
				DriverName:    ev.Vehicle.DriverName,
			},
			Geofence: model.AlertFence{
				GeofenceID:   ev.Geofence.ID,
				GeofenceName: ev.Geofence.Name,
				Category:     ev.Geofence.Category,
			},
			Location: model.AlertPoint{Latitude: ev.Latitude, Longitude: ev.Longitude},
		})
		if err != nil {
			log.Printf("encode alert: %v", err)
			continue
		}
		payloads = append(payloads, body)
	}
	if len(payloads) > 0 {
		go func(msgs [][]byte) {
			for _, msg := range msgs {
				s.hub.Broadcast(msg)
			}
		}(payloads)
	}

	hits := make([]fenceHit, 0, len(outcome.Inside))
	for _, g := range outcome.Inside {
		hits = append(hits, fenceHit{GeofenceID: g.ID, GeofenceName: g.Name, Status: "inside"})
	}
	writeJSON(w, http.StatusOK, struct {
		VehicleID         string     `json:"vehicle_id"`
		LocationUpdated   bool       `json:"location_updated"`
		CurrentGeofences  []fenceHit `json:"current_geofences"`
		TimeNS            string     `json:"time_ns"`
	}{
		VehicleID:        vehicleID,
		LocationUpdated:  true,
		CurrentGeofences: hits,
		TimeNS:           ns(start),
	})
}

type fenceHit struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Status       string `json:"status"`
}

func (s *Server) getLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := strings.TrimSpace(r.PathValue("vehicle_id"))
	if id == "" {
		fail(w, start, http.StatusBadRequest, "vehicle_id is required")
		return
	}
	vehicle, fix, inside, err := s.store.VehicleSnapshot(r.Context(), id)
	if errors.Is(err, store.ErrVehicleNotFound) {
		fail(w, start, http.StatusNotFound, "vehicle not found")
		return
	}
	if err != nil {
		log.Printf("get location: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not load vehicle location")
		return
	}
	current := make([]currentFence, 0, len(inside))
	for _, g := range inside {
		current = append(current, currentFence{
			GeofenceID:   g.ID,
			GeofenceName: g.Name,
			Category:     g.Category,
		})
	}
	writeJSON(w, http.StatusOK, struct {
		VehicleID        string         `json:"vehicle_id"`
		VehicleNumber    string         `json:"vehicle_number"`
		CurrentLocation  *model.LocationFix `json:"current_location"`
		CurrentGeofences []currentFence `json:"current_geofences"`
		TimeNS           string         `json:"time_ns"`
	}{
		VehicleID:        vehicle.ID,
		VehicleNumber:    vehicle.VehicleNumber,
		CurrentLocation:  fix,
		CurrentGeofences: current,
		TimeNS:           ns(start),
	})
}

type currentFence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

func validVehicleNumber(s string) bool {
	n := len([]rune(s))
	if n < 3 || n > 32 {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == ' ' {
			continue
		}
		return false
	}
	return true
}

func validPhone(s string) bool {
	if len(s) < 7 || len(s) > 20 {
		return false
	}
	digits := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' || r == '-' || r == ' ' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return digits >= 7
}
