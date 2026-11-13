package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"geofence/internal/model"
	"geofence/internal/store"
)

func (s *Server) listViolations(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	limit, offset, err := pageParams(r, 50, 500)
	if err != nil {
		fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	q := r.URL.Query()
	filter := store.ViolationFilter{
		VehicleID:  strings.TrimSpace(q.Get("vehicle_id")),
		GeofenceID: strings.TrimSpace(q.Get("geofence_id")),
		Limit:      limit,
		Offset:     offset,
	}
	if raw := strings.TrimSpace(q.Get("start_date")); raw != "" {
		ts, err := parseTimestamp(raw)
		if err != nil {
			fail(w, start, http.StatusBadRequest, "start_date must be ISO 8601, for example 2025-01-01T00:00:00Z")
			return
		}
		ms := ts.UnixMilli()
		filter.StartMS = &ms
	}
	if raw := strings.TrimSpace(q.Get("end_date")); raw != "" {
		ts, err := parseTimestamp(raw)
		if err != nil {
			fail(w, start, http.StatusBadRequest, "end_date must be ISO 8601, for example 2025-01-31T23:59:59Z")
			return
		}
		ms := ts.UnixMilli()
		filter.EndMS = &ms
	}
	if filter.StartMS != nil && filter.EndMS != nil && *filter.StartMS > *filter.EndMS {
		fail(w, start, http.StatusBadRequest, "start_date must be earlier than end_date")
		return
	}

	items, total, err := s.store.ListViolations(r.Context(), filter)
	if err != nil {
		log.Printf("list violations: %v", err)
		fail(w, start, http.StatusInternalServerError, "could not load history")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Violations []model.Violation `json:"violations"`
		TotalCount int               `json:"total_count"`
		TimeNS     string            `json:"time_ns"`
	}{Violations: items, TotalCount: total, TimeNS: ns(start)})
}
