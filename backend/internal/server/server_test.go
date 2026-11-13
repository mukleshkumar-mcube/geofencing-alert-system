package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"geofence/internal/config"
	"geofence/internal/server"
	"geofence/internal/store"
	"geofence/internal/ws"

	"github.com/gorilla/websocket"
)

func testServer(t *testing.T, cfg config.Config) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ts := httptest.NewServer(server.New(st, ws.New(), cfg).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, method, url, body string, headers map[string]string) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("status %d body %s", res.StatusCode, raw)
		}
	}
	return res.StatusCode, payload
}

func requireTime(t *testing.T, payload map[string]any) {
	t.Helper()
	raw, ok := payload["time_ns"].(string)
	if !ok || raw == "" {
		t.Fatalf("time_ns missing in %#v", payload)
	}
	if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
		t.Fatalf("time_ns %q is not an integer: %v", raw, err)
	}
}

const downtown = `{
  "name": "Downtown Delivery Zone",
  "description": "Main delivery area",
  "coordinates": [[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]],
  "category": "delivery_zone"
}`

const square = `{
  "name": "Yard",
  "coordinates": [[0,0],[0,1],[1,1],[1,0],[0,0]],
  "category": "restricted_zone"
}`

func TestGeofenceValidation(t *testing.T) {
	ts := testServer(t, config.Config{})
	code, payload := do(t, http.MethodPost, ts.URL+"/geofences", `{
		"name": "Open",
		"coordinates": [[0,0],[0,1],[1,1],[1,0]],
		"category": "delivery_zone"
	}`, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("status %d %#v", code, payload)
	}
	if !strings.Contains(payload["error"].(string), "closed") {
		t.Fatalf("error = %#v", payload["error"])
	}
	requireTime(t, payload)

	code, payload = do(t, http.MethodPost, ts.URL+"/geofences", downtown, nil)
	if code != http.StatusCreated {
		t.Fatalf("create status %d %#v", code, payload)
	}
	if payload["status"] != "active" || payload["name"] != "Downtown Delivery Zone" {
		t.Fatalf("unexpected create body %#v", payload)
	}
	requireTime(t, payload)

	code, payload = do(t, http.MethodGet, ts.URL+"/geofences?category=delivery_zone", "", nil)
	if code != http.StatusOK {
		t.Fatalf("list status %d", code)
	}
	items := payload["geofences"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 geofence, got %#v", items)
	}
	requireTime(t, payload)

	code, _ = do(t, http.MethodGet, ts.URL+"/geofences?category=nope", "", nil)
	if code != http.StatusBadRequest {
		t.Fatalf("bad category status %d", code)
	}
}

func TestTrackingFlow(t *testing.T) {
	ts := testServer(t, config.Config{})
	_, created := do(t, http.MethodPost, ts.URL+"/geofences", downtown, nil)
	fenceID := created["id"].(string)

	code, vehicle := do(t, http.MethodPost, ts.URL+"/vehicles", `{
		"vehicle_number": "KA-01-AB-1234",
		"driver_name": "John Doe",
		"vehicle_type": "truck",
		"phone": "+14155550100"
	}`, nil)
	if code != http.StatusCreated {
		t.Fatalf("vehicle status %d %#v", code, vehicle)
	}
	vehicleID := vehicle["id"].(string)
	requireTime(t, vehicle)

	code, dup := do(t, http.MethodPost, ts.URL+"/vehicles", `{
		"vehicle_number": "ka-01-ab-1234",
		"driver_name": "Jane",
		"vehicle_type": "car",
		"phone": "+14155550111"
	}`, nil)
	if code != http.StatusConflict {
		t.Fatalf("duplicate status %d %#v", code, dup)
	}

	code, missing := do(t, http.MethodPost, ts.URL+"/vehicles/location", `{
		"vehicle_id": "veh_missing",
		"latitude": 37.78,
		"longitude": -122.414,
		"timestamp": "2025-01-15T10:35:00Z"
	}`, nil)
	if code != http.StatusNotFound {
		t.Fatalf("missing vehicle status %d %#v", code, missing)
	}

	code, outside := do(t, http.MethodPost, ts.URL+"/vehicles/location", `{
		"vehicle_id": "`+vehicleID+`",
		"latitude": 37.78,
		"longitude": -122.43,
		"timestamp": "2025-01-15T10:35:00Z"
	}`, nil)
	if code != http.StatusOK || outside["location_updated"] != true {
		t.Fatalf("outside update %#v", outside)
	}
	if len(outside["current_geofences"].([]any)) != 0 {
		t.Fatalf("expected no geofence, got %#v", outside["current_geofences"])
	}

	code, inside := do(t, http.MethodPost, ts.URL+"/vehicles/location", `{
		"vehicle_id": "`+vehicleID+`",
		"latitude": 37.78,
		"longitude": -122.414,
		"timestamp": "2025-01-15T10:36:00Z"
	}`, nil)
	if code != http.StatusOK {
		t.Fatalf("inside status %d %#v", code, inside)
	}
	hits := inside["current_geofences"].([]any)
	if len(hits) != 1 || hits[0].(map[string]any)["geofence_id"] != fenceID {
		t.Fatalf("hits %#v", hits)
	}
	if hits[0].(map[string]any)["status"] != "inside" {
		t.Fatalf("status %#v", hits[0])
	}

	code, snap := do(t, http.MethodGet, ts.URL+"/vehicles/location/"+vehicleID, "", nil)
	if code != http.StatusOK {
		t.Fatalf("snapshot %d %#v", code, snap)
	}
	if snap["vehicle_number"] != "KA-01-AB-1234" {
		t.Fatalf("snapshot %#v", snap)
	}
	loc := snap["current_location"].(map[string]any)
	if loc["latitude"].(float64) != 37.78 {
		t.Fatalf("location %#v", loc)
	}

	code, history := do(t, http.MethodGet, ts.URL+"/violations/history?vehicle_id="+vehicleID+"&limit=10", "", nil)
	if code != http.StatusOK {
		t.Fatalf("history %d", code)
	}
	if history["total_count"].(float64) != 1 {
		t.Fatalf("expected one entry, got %#v", history)
	}
	row := history["violations"].([]any)[0].(map[string]any)
	if row["event_type"] != "entry" || row["geofence_name"] != "Downtown Delivery Zone" {
		t.Fatalf("row %#v", row)
	}
}

func TestRealtimeAlerts(t *testing.T) {
	ts := testServer(t, config.Config{})
	_, created := do(t, http.MethodPost, ts.URL+"/geofences", downtown, nil)
	fenceID := created["id"].(string)
	_, vehicle := do(t, http.MethodPost, ts.URL+"/vehicles", `{
		"vehicle_number": "MH-12-CD-7781",
		"driver_name": "Priya Nair",
		"vehicle_type": "van",
		"phone": "+14155550198"
	}`, nil)
	vehicleID := vehicle["id"].(string)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/alerts"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// No rule yet: the crossing is stored, but nothing is broadcast.
	do(t, http.MethodPost, ts.URL+"/vehicles/location", locationBody(vehicleID, 37.78, -122.414, "2025-01-15T10:40:00Z"), nil)
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected no alert before a rule exists")
	}
	conn.Close()

	code, rule := do(t, http.MethodPost, ts.URL+"/alerts/configure", `{
		"geofence_id": "`+fenceID+`",
		"vehicle_id": "`+vehicleID+`",
		"event_type": "both"
	}`, nil)
	if code != http.StatusCreated || rule["status"] != "active" {
		t.Fatalf("rule %d %#v", code, rule)
	}

	conn = dial(t, ts.URL)
	defer conn.Close()

	// Already inside, so step outside and come back to raise entry, then leave.
	do(t, http.MethodPost, ts.URL+"/vehicles/location", locationBody(vehicleID, 37.78, -122.43, "2025-01-15T10:41:00Z"), nil)
	exitMsg := readAlert(t, conn)
	if exitMsg["event_type"] != "exit" {
		t.Fatalf("exit payload %#v", exitMsg)
	}

	do(t, http.MethodPost, ts.URL+"/vehicles/location", locationBody(vehicleID, 37.78, -122.414, "2025-01-15T10:42:00Z"), nil)
	entryMsg := readAlert(t, conn)
	if entryMsg["event_type"] != "entry" {
		t.Fatalf("entry payload %#v", entryMsg)
	}
	gotVehicle := entryMsg["vehicle"].(map[string]any)
	gotFence := entryMsg["geofence"].(map[string]any)
	if gotVehicle["vehicle_number"] != "MH-12-CD-7781" || gotFence["geofence_name"] != "Downtown Delivery Zone" {
		t.Fatalf("entry payload %#v", entryMsg)
	}
	if entryMsg["location"].(map[string]any)["longitude"].(float64) != -122.414 {
		t.Fatalf("location %#v", entryMsg["location"])
	}

	code, listed := do(t, http.MethodGet, ts.URL+"/alerts?vehicle_id="+vehicleID, "", nil)
	if code != http.StatusOK || len(listed["alerts"].([]any)) != 1 {
		t.Fatalf("alerts %#v", listed)
	}
}

func TestRestrictedZoneAlertsWithoutExtraRule(t *testing.T) {
	ts := testServer(t, config.Config{})
	code, created := do(t, http.MethodPost, ts.URL+"/geofences", square, nil)
	if code != http.StatusCreated {
		t.Fatal(created)
	}
	_, vehicle := do(t, http.MethodPost, ts.URL+"/vehicles", `{
		"vehicle_number": "DL-09-ZZ-1001",
		"driver_name": "Asha Menon",
		"vehicle_type": "car",
		"phone": "+14155550999"
	}`, nil)

	conn := dial(t, ts.URL)
	defer conn.Close()
	do(t, http.MethodPost, ts.URL+"/vehicles/location", locationBody(vehicle["id"].(string), 0.5, 0.5, "2025-01-15T11:00:00Z"), nil)
	msg := readAlert(t, conn)
	if msg["event_type"] != "entry" {
		t.Fatalf("payload %#v", msg)
	}
	if msg["geofence"].(map[string]any)["category"] != "restricted_zone" {
		t.Fatalf("category %#v", msg["geofence"])
	}

	_, rules := do(t, http.MethodGet, ts.URL+"/alerts", "", nil)
	foundBoth := false
	for _, item := range rules["alerts"].([]any) {
		rule := item.(map[string]any)
		if rule["event_type"] == "both" && rule["vehicle_id"] == nil {
			foundBoth = true
		}
	}
	if !foundBoth {
		t.Fatalf("expected an automatic fleet-wide rule, got %#v", rules["alerts"])
	}
}

func TestRateLimitAndAPIKey(t *testing.T) {
	ts := testServer(t, config.Config{RatePerMin: 2})
	for i := 0; i < 2; i++ {
		code, _ := do(t, http.MethodPost, ts.URL+"/vehicles/location", `{}`, nil)
		if code == http.StatusTooManyRequests {
			t.Fatalf("request %d was limited too early", i+1)
		}
	}
	code, payload := do(t, http.MethodPost, ts.URL+"/vehicles/location", `{}`, nil)
	if code != http.StatusTooManyRequests {
		t.Fatalf("third request status %d %#v", code, payload)
	}

	locked := testServer(t, config.Config{APIKey: "harbor-secret"})
	code, _ = do(t, http.MethodGet, locked.URL+"/geofences", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("missing key status %d", code)
	}
	code, payload = do(t, http.MethodGet, locked.URL+"/geofences", "", map[string]string{"X-API-Key": "harbor-secret"})
	if code != http.StatusOK {
		t.Fatalf("with key status %d %#v", code, payload)
	}
	code, _ = do(t, http.MethodGet, locked.URL+"/health", "", nil)
	if code != http.StatusOK {
		t.Fatalf("health should stay open, got %d", code)
	}
}

func TestListPagination(t *testing.T) {
	ts := testServer(t, config.Config{})
	code, downtownCreated := do(t, http.MethodPost, ts.URL+"/geofences", downtown, nil)
	if code != http.StatusCreated {
		t.Fatalf("downtown status %d", code)
	}
	code, _ = do(t, http.MethodPost, ts.URL+"/geofences", square, nil)
	if code != http.StatusCreated {
		t.Fatalf("yard status %d", code)
	}

	code, page := do(t, http.MethodGet, ts.URL+"/geofences?limit=1", "", nil)
	if code != http.StatusOK || len(page["geofences"].([]any)) != 1 || page["total_count"].(float64) != 2 {
		t.Fatalf("geofence page %#v", page)
	}
	code, next := do(t, http.MethodGet, ts.URL+"/geofences?limit=1&offset=1", "", nil)
	if code != http.StatusOK || len(next["geofences"].([]any)) != 1 {
		t.Fatalf("geofence offset %#v", next)
	}

	for _, body := range []string{
		`{"vehicle_number":"KA-01-AB-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+14155550100"}`,
		`{"vehicle_number":"MH-12-CD-7781","driver_name":"Priya Nair","vehicle_type":"van","phone":"+14155550198"}`,
	} {
		code, _ = do(t, http.MethodPost, ts.URL+"/vehicles", body, nil)
		if code != http.StatusCreated {
			t.Fatalf("vehicle status %d", code)
		}
	}
	code, vehicles := do(t, http.MethodGet, ts.URL+"/vehicles?limit=1", "", nil)
	if code != http.StatusOK || len(vehicles["vehicles"].([]any)) != 1 || vehicles["total_count"].(float64) != 2 {
		t.Fatalf("vehicle page %#v", vehicles)
	}

	code, rule := do(t, http.MethodPost, ts.URL+"/alerts/configure", `{
		"geofence_id":"`+downtownCreated["id"].(string)+`",
		"event_type":"entry"
	}`, nil)
	if code != http.StatusCreated {
		t.Fatalf("rule status %d %#v", code, rule)
	}
	code, alerts := do(t, http.MethodGet, ts.URL+"/alerts?limit=1", "", nil)
	if code != http.StatusOK || len(alerts["alerts"].([]any)) != 1 || alerts["total_count"].(float64) < 2 {
		t.Fatalf("alert page %#v", alerts)
	}
	requireTime(t, page)
	requireTime(t, vehicles)
	requireTime(t, alerts)
}

func TestManageAlerts(t *testing.T) {
	ts := testServer(t, config.Config{})
	code, fence := do(t, http.MethodPost, ts.URL+"/geofences", downtown, nil)
	if code != http.StatusCreated {
		t.Fatalf("geofence status %d", code)
	}
	code, rule := do(t, http.MethodPost, ts.URL+"/alerts/configure", `{
		"geofence_id":"`+fence["id"].(string)+`",
		"event_type":"entry"
	}`, nil)
	if code != http.StatusCreated {
		t.Fatalf("create status %d %#v", code, rule)
	}
	id := rule["alert_id"].(string)

	code, paused := do(t, http.MethodPost, ts.URL+"/alerts/"+id+"/status", `{"status":"inactive"}`, nil)
	if code != http.StatusOK || paused["status"] != "inactive" {
		t.Fatalf("pause status %d %#v", code, paused)
	}
	requireTime(t, paused)

	code, listed := do(t, http.MethodGet, ts.URL+"/alerts", "", nil)
	if code != http.StatusOK || listed["alerts"].([]any)[0].(map[string]any)["status"] != "inactive" {
		t.Fatalf("list %#v", listed)
	}

	code, removed := do(t, http.MethodDelete, ts.URL+"/alerts/"+id, "", nil)
	if code != http.StatusOK || removed["status"] != "deleted" {
		t.Fatalf("delete status %d %#v", code, removed)
	}
	code, missing := do(t, http.MethodDelete, ts.URL+"/alerts/"+id, "", nil)
	if code != http.StatusNotFound {
		t.Fatalf("second delete status %d %#v", code, missing)
	}
}

func locationBody(id string, lat, lng float64, ts string) string {
	raw, _ := json.Marshal(map[string]any{
		"vehicle_id": id,
		"latitude":   lat,
		"longitude":  lng,
		"timestamp":  ts,
	})
	return string(raw)
}

func dial(t *testing.T, httpURL string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpURL, "http")+"/ws/alerts", nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func readAlert(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read alert: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	return msg
}
