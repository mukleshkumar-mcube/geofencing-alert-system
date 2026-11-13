package server

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"geofence/internal/config"
	"geofence/internal/ratelimit"
	"geofence/internal/store"
	"geofence/internal/ws"
)

// Server is the HTTP and WebSocket API.
type Server struct {
	store   *store.Store
	hub     *ws.Hub
	cfg     config.Config
	limiter *ratelimit.Limiter
}

// New wires the API around a store and an alert hub.
func New(st *store.Store, hub *ws.Hub, cfg config.Config) *Server {
	return &Server{
		store:   st,
		hub:     hub,
		cfg:     cfg,
		limiter: ratelimit.New(cfg.RatePerMin),
	}
}

// Handler returns the fully wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /{$}", s.root)
	mux.HandleFunc("POST /geofences", s.createGeofence)
	mux.HandleFunc("GET /geofences", s.listGeofences)
	mux.HandleFunc("POST /vehicles", s.createVehicle)
	mux.HandleFunc("GET /vehicles", s.listVehicles)
	mux.HandleFunc("POST /vehicles/location", s.updateLocation)
	mux.HandleFunc("GET /vehicles/location/{vehicle_id}", s.getLocation)
	mux.HandleFunc("POST /alerts/configure", s.configureAlert)
	mux.HandleFunc("GET /alerts", s.listAlerts)
	mux.HandleFunc("POST /alerts/{alert_id}/status", s.setAlertStatus)
	mux.HandleFunc("DELETE /alerts/{alert_id}", s.deleteAlert)
	mux.HandleFunc("GET /violations/history", s.listViolations)
	mux.HandleFunc("GET /ws/alerts", s.alertsSocket)
	return s.withCORS(s.recover(s.logRequests(s.authenticate(mux))))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	start := time.Now()
	writeJSON(w, http.StatusOK, mapBody{"status": "ok", "time_ns": ns(start)})
}

func (s *Server) root(w http.ResponseWriter, _ *http.Request) {
	start := time.Now()
	writeJSON(w, http.StatusOK, mapBody{
		"service":   "harbor-geofence",
		"status":    "ok",
		"websocket": "/ws/alerts",
		"time_ns":   ns(start),
	})
}

type errorBody struct {
	Error  string `json:"error"`
	TimeNS string `json:"time_ns"`
}

type mapBody map[string]string

func ns(start time.Time) string {
	return strconv.FormatInt(time.Since(start).Nanoseconds(), 10)
}

func fail(w http.ResponseWriter, start time.Time, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg, TimeNS: ns(start)})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

var (
	errBadJSON    = errors.New("request body is not valid JSON")
	errBodyTooBig = errors.New("request body is too large")
)

func readJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) || strings.Contains(err.Error(), "too large") {
			return errBodyTooBig
		}
		return errBadJSON
	}
	return nil
}

func pageParams(r *http.Request, def, max int) (int, int, error) {
	limit := def
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > max {
			return 0, 0, errors.New("limit must be a positive integer up to " + strconv.Itoa(max))
		}
		limit = n
	}
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return 0, 0, errors.New("offset must be a non-negative integer")
		}
		offset = n
	}
	return limit, offset, nil
}

func parseTimestamp(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("missing")
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errors.New("bad")
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func eventID() string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return "evt_" + hex.EncodeToString(buf[:])
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.APIKey == "" || r.URL.Path == "/health" || r.URL.Path == "/ws/alerts" {
			next.ServeHTTP(w, r)
			return
		}
		if subtleKeyMatch(r.Header.Get("X-API-Key"), s.cfg.APIKey) {
			next.ServeHTTP(w, r)
			return
		}
		fail(w, time.Now(), http.StatusUnauthorized, "missing or invalid API key")
	})
}

func subtleKeyMatch(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	var diff byte
	for i := 0; i < len(got); i++ {
		diff |= got[i] ^ want[i]
	}
	return diff == 0
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		if r.URL.Path != "/health" {
			log.Printf("%s %s %d %s", r.Method, r.URL.RequestURI(), ww.status, time.Since(start).Round(time.Millisecond))
		}
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				fail(w, time.Now(), http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusWriter) WriteHeader(code int) {
	if s.wrote {
		return
	}
	s.wrote = true
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status = http.StatusOK
		s.wrote = true
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacker not available")
	}
	return h.Hijack()
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }
