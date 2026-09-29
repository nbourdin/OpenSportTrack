package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"opensporttrack/internal/tracking"
)

func TestHTTPAndWebSocketFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := tracking.NewManager()
	defer m.Close()
	ts := httptest.NewServer(NewHandler(ctx, m, slog.New(slog.DiscardHandler)))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/activities", "application/json", strings.NewReader(`{"sport":"running"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var activity tracking.Activity
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		t.Fatal(err)
	}
	resp, err = http.Get(ts.URL + "/api/v1/activities/" + activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get activity status = %d", resp.StatusCode)
	}
	resp, err = http.Get(ts.URL + "/api/v1/activities/missing")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing activity status = %d", resp.StatusCode)
	}
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/activities/" + activity.ID + "/live"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readCancel()
	var snapshot tracking.Message
	if err := wsjson.Read(readCtx, conn, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Type != "snapshot" || snapshot.ActivityID != activity.ID {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	routeURL := ts.URL + "/api/v1/activities/" + activity.ID + "/route"
	routeBody := []byte(`{"positions":[{"latitude":47.21808,"longitude":-1.55199},{"latitude":47.21796,"longitude":-1.55286}]}`)
	req, err := http.NewRequest(http.MethodPut, routeURL, bytes.NewReader(routeBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("route status = %d", resp.StatusCode)
	}
	var routeUpdate tracking.Message
	if err := wsjson.Read(readCtx, conn, &routeUpdate); err != nil {
		t.Fatal(err)
	}
	if routeUpdate.Type != "route" || len(routeUpdate.Route) != 2 {
		t.Fatalf("unexpected route update: %+v", routeUpdate)
	}
	endpoint := ts.URL + "/api/v1/activities/" + activity.ID + "/samples"
	point := []byte(`{"timestamp":"2026-09-29T08:00:00Z","position":{"latitude":47.21808,"longitude":-1.55199},"next":{"timestamp":"2026-09-29T08:00:01Z","position":{"latitude":47.21796,"longitude":-1.55286},"after_ms":500}}`)
	resp, err = http.Post(endpoint, "application/json", bytes.NewReader(point))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("sample status = %d", resp.StatusCode)
	}
	var update tracking.Message
	if err := wsjson.Read(readCtx, conn, &update); err != nil {
		t.Fatal(err)
	}
	if update.Type != "sample" || update.Sample.Position.Latitude != 47.21808 || update.Sample.Next.AfterMS != 500 {
		t.Fatalf("unexpected live update: %+v", update)
	}

	late, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer late.CloseNow()
	var history tracking.Message
	if err := wsjson.Read(readCtx, late, &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Samples) != 1 || len(history.Route) != 2 {
		t.Fatalf("late viewer has %d samples and %d route points", len(history.Samples), len(history.Route))
	}
}
