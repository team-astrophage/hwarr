package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsOriginAllowed(t *testing.T) {
	allowed := []string{"http://localhost:5173", "http://localhost:3000"}

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"allowed origin", "http://localhost:5173", true},
		{"another allowed origin", "http://localhost:3000", true},
		{"disallowed origin", "http://evil.com", false},
		{"empty origin", "", false},
		{"partial match", "http://localhost", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOriginAllowed(tt.origin, allowed)
			if got != tt.want {
				t.Errorf("isOriginAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func newTestRouter(allowedOrigins []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(corsMiddleware(allowedOrigins))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func TestCorsMiddleware_AllowedOrigin(t *testing.T) {
	r := newTestRouter([]string{"http://localhost:5173"})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("expected Access-Control-Allow-Origin = %q, got %q", "http://localhost:5173", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials = true, got %q", got)
	}
}

func TestCorsMiddleware_DisallowedOrigin(t *testing.T) {
	r := newTestRouter([]string{"http://localhost:5173"})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header, got %q", got)
	}
}

func TestCorsMiddleware_NoOrigin(t *testing.T) {
	r := newTestRouter([]string{"http://localhost:5173"})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header, got %q", got)
	}
}

func TestCorsMiddleware_OptionsAllowedOrigin(t *testing.T) {
	r := newTestRouter([]string{"http://localhost:5173"})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("expected Access-Control-Allow-Origin = %q, got %q", "http://localhost:5173", got)
	}
}
