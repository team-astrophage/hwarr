package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupQRRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewQRHandler()
	h.Register(r)
	return r
}

func TestGetMapURL_Default(t *testing.T) {
	os.Unsetenv("FRONTEND_URL")
	got := getMapURL(false)
	if got != defaultFrontendURL {
		t.Errorf("expected %s, got %s", defaultFrontendURL, got)
	}
}

func TestGetMapURL_Custom(t *testing.T) {
	os.Setenv("FRONTEND_URL", "https://custom.example.com/")
	defer os.Unsetenv("FRONTEND_URL")
	got := getMapURL(false)
	if got != "https://custom.example.com" {
		t.Errorf("expected trailing slash stripped, got %s", got)
	}
}

func TestGetMapURL_Demo(t *testing.T) {
	os.Unsetenv("FRONTEND_URL")
	got := getMapURL(true)
	expected := defaultFrontendURL + "?demo=true"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestQREndpoint_DefaultPNG(t *testing.T) {
	r := setupQRRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/qr", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected image/png, got %s", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("expected Cache-Control header, got %s", cc)
	}
	if xqr := w.Header().Get("X-QR-URL"); xqr == "" {
		t.Error("expected X-QR-URL header to be set")
	}
	// Check PNG magic bytes
	body := w.Body.Bytes()
	if len(body) < 4 || body[0] != 0x89 || body[1] != 'P' || body[2] != 'N' || body[3] != 'G' {
		t.Error("response body is not a valid PNG")
	}
}

func TestQREndpoint_DemoMode(t *testing.T) {
	os.Unsetenv("FRONTEND_URL")
	r := setupQRRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/qr?demo=true", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	expected := defaultFrontendURL + "?demo=true"
	if xqr := w.Header().Get("X-QR-URL"); xqr != expected {
		t.Errorf("expected X-QR-URL %s, got %s", expected, xqr)
	}
}

func TestQREndpoint_CustomSize(t *testing.T) {
	r := setupQRRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/qr?size=20", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Larger size should produce a larger PNG
	body := w.Body.Bytes()
	if len(body) < 100 {
		t.Error("expected non-trivial PNG output for size=20")
	}
}

func TestQREndpoint_InvalidSizeFallsBack(t *testing.T) {
	r := setupQRRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/qr?size=999", nil)
	r.ServeHTTP(w, req)

	// Should still succeed with default size
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for invalid size, got %d", w.Code)
	}
}
