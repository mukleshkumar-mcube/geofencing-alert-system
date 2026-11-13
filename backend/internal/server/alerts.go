package server

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"geofence/internal/model"
	"geofence/internal/store"
)

func (s *Server) configureAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req struct {
		GeofenceID string  `json:"geofence_id"`
		VehicleID  *string `json:"vehicle_id"`
		EventType  string  `json:"event_type"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	geofenceID := strings.TrimSpace(req.GeofenceID)
	if geofenceID == "" {
		fail(w, start, http.StatusBadRequest, "geofence_id is required")
		return
	}
	if !model.ValidEventType(req.EventType) {
		fail(w, start, http.StatusBadRequest, "event_type must be one of entry, exit, both")
		return
	}
	var vehicleID *string
	if req.VehicleID != nil {
		trimmed := strings.TrimSpace(*req.VehicleID)
		if trimmed != "" {
			vehicleID = &trimmed
		}
	}

	rule, err := s.store.CreateAlert(r.Context(), geofenceID, vehicleID, req.EventType)
	switch {
	case errors.Is(err, store.ErrGeofenceNotFound):
		fail(w, start, http.StatusNotFound, "geofence not found")
		return
	case errors.Is(err, store.ErrVehicleNotFound):
		fail(w, start, http.StatusNotFound, "vehicle not found")
		return
	case errors.Is(err, store.ErrConflict):
		fail(w, start, http.StatusConflict, "this alert rule already exists")
		return
	case err != nil:
		log.Printf("create alert: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not save alert rule")
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		AlertID    string  `json:"alert_id"`
		GeofenceID string  `json:"geofence_id"`
		VehicleID  *string `json:"vehicle_id"`
		EventType  string  `json:"event_type"`
		Status     string  `json:"status"`
		TimeNS     string  `json:"time_ns"`
	}{
		AlertID:    rule.AlertID,
		GeofenceID: rule.GeofenceID,
		VehicleID:  rule.VehicleID,
		EventType:  rule.EventType,
		Status:     rule.Status,
		TimeNS:     ns(start),
	})
}

func (s *Server) listAlerts(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	limit, offset, err := pageParams(r, 500, 500)
	if err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := s.store.ListAlerts(
		r.Context(),
		strings.TrimSpace(r.URL.Query().Get("geofence_id")),
		strings.TrimSpace(r.URL.Query().Get("vehicle_id")),
		limit,
		offset,
	)
	if err != nil {
		log.Printf("list alerts: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not list alert rules")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Alerts     []model.AlertRule `json:"alerts"`
		TotalCount int               `json:"total_count"`
		TimeNS     string            `json:"time_ns"`
	}{Alerts: items, TotalCount: total, TimeNS: ns(start)})
}

func (s *Server) setAlertStatus(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req struct {
		Status string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	status := strings.TrimSpace(req.Status)
	if status != "active" && status != "inactive" {
		fail(w, start, http.StatusBadRequest, "status must be active or inactive")
		return
	}
	rule, err := s.store.SetAlertStatus(r.Context(), r.PathValue("alert_id"), status)
	if errors.Is(err, store.ErrAlertNotFound) {
		fail(w, start, http.StatusNotFound, "alert rule not found")
		return
	}
	if err != nil {
		log.Printf("set alert status: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not update alert rule")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		AlertID string `json:"alert_id"`
		Status  string `json:"status"`
		TimeNS  string `json:"time_ns"`
	}{AlertID: rule.AlertID, Status: rule.Status, TimeNS: ns(start)})
}

func (s *Server) deleteAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	err := s.store.DeleteAlert(r.Context(), r.PathValue("alert_id"))
	if errors.Is(err, store.ErrAlertNotFound) {
		fail(w, start, http.StatusNotFound, "alert rule not found")
		return
	}
	if err != nil {
		log.Printf("delete alert: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not remove alert rule")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
		TimeNS string `json:"time_ns"`
	}{Status: "deleted", TimeNS: ns(start)})
}

func (s *Server) alertsSocket(w http.ResponseWriter, r *http.Request) {
	if s.cfg.APIKey != "" && !subtleKeyMatch(r.URL.Query().Get("api_key"), s.cfg.APIKey) {
		fail(w, time.Now(), http.StatusUnauthorized, "missing or invalid API key")
		return
	}
	s.hub.ServeHTTP(w, r)
}
