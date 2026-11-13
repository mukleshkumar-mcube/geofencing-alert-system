package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"geofence/internal/geo"
	"geofence/internal/model"
)

func (s *Server) createGeofence(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Coordinates [][]float64 `json:"coordinates"`
		Category    string      `json:"category"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 120 {
		fail(w, start, http.StatusBadRequest, "name is required and must be at most 120 characters")
		return
	}
	description := strings.TrimSpace(req.Description)
	if len([]rune(description)) > 2000 {
		fail(w, start, http.StatusBadRequest, "description must be at most 2000 characters")
		return
	}
	if !model.ValidCategory(req.Category) {
		fail(w, start, http.StatusBadRequest, "category must be one of delivery_zone, restricted_zone, toll_zone, customer_area")
		return
	}
	if err := geo.ValidatePolygon(req.Coordinates); err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}

	g, err := s.store.CreateGeofence(r.Context(), name, description, req.Category, req.Coordinates)
	if err != nil {
		log.Printf("create geofence: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not create geofence")
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
		TimeNS string `json:"time_ns"`
	}{
		ID:     g.ID,
		Name:   g.Name,
		Status: "active",
		TimeNS: ns(start),
	})
}

func (s *Server) listGeofences(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category != "" && !model.ValidCategory(category) {
		fail(w, start, http.StatusBadRequest, "category must be one of delivery_zone, restricted_zone, toll_zone, customer_area")
		return
	}
	limit, offset, err := pageParams(r, 500, 500)
	if err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := s.store.ListGeofences(r.Context(), category, limit, offset)
	if err != nil {
		log.Printf("list geofences: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not list geofences")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Geofences  []model.Geofence `json:"geofences"`
		TotalCount int              `json:"total_count"`
		TimeNS     string           `json:"time_ns"`
	}{Geofences: items, TotalCount: total, TimeNS: ns(start)})
}
