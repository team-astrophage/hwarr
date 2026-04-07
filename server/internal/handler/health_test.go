package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// --- test doubles ---

type stubCounter struct{ count int }

func (s *stubCounter) ActiveCount() int { return s.count }

type stubEngine struct{ running bool }

func (s *stubEngine) IsRunning() bool { return s.running }

// --- helpers ---

func setupRouter(h *HealthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r)
	return r
}

func doGet(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestHealthReturnsOK(t *testing.T) {
	h := NewHealthHandler(&stubCounter{count: 0}, &stubEngine{running: false})
	r := setupRouter(h)

	w := doGet(r, "/health")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["connections"] != float64(0) {
		t.Errorf("expected connections=0, got %v", body["connections"])
	}
	if body["engine_running"] != false {
		t.Errorf("expected engine_running=false, got %v", body["engine_running"])
	}
}

func TestHealthReflectsConnectionCount(t *testing.T) {
	h := NewHealthHandler(&stubCounter{count: 42}, &stubEngine{running: true})
	r := setupRouter(h)

	w := doGet(r, "/health")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if body["connections"] != float64(42) {
		t.Errorf("expected connections=42, got %v", body["connections"])
	}
	if body["engine_running"] != true {
		t.Errorf("expected engine_running=true, got %v", body["engine_running"])
	}
}

func TestHealthNilDependencies(t *testing.T) {
	h := NewHealthHandler(nil, nil)
	r := setupRouter(h)

	w := doGet(r, "/health")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["connections"] != float64(0) {
		t.Errorf("expected connections=0, got %v", body["connections"])
	}
	if body["engine_running"] != false {
		t.Errorf("expected engine_running=false, got %v", body["engine_running"])
	}
}

func TestHealthEngineRunningTrue(t *testing.T) {
	h := NewHealthHandler(&stubCounter{count: 5}, &stubEngine{running: true})
	r := setupRouter(h)

	w := doGet(r, "/health")

	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)

	if body["engine_running"] != true {
		t.Errorf("expected engine_running=true, got %v", body["engine_running"])
	}
}
