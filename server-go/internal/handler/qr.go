package handler

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
)

const defaultFrontendURL = "https://bulpan.example.com"

// QRHandler serves the GET /api/qr endpoint.
type QRHandler struct{}

// NewQRHandler creates a new QRHandler.
func NewQRHandler() *QRHandler {
	return &QRHandler{}
}

// getMapURL builds the map page URL, optionally with ?demo=true for GPS fallback.
func getMapURL(demo bool) string {
	base := os.Getenv("FRONTEND_URL")
	if base == "" {
		base = defaultFrontendURL
	}
	base = strings.TrimRight(base, "/")
	if demo {
		return base + "?demo=true"
	}
	return base
}

// Handle generates a QR code PNG image that links to the frontend map page.
//
//	GET /api/qr?demo=false&size=10
//	Response: PNG image bytes
//	Headers: Content-Type: image/png, Cache-Control: public, max-age=3600, X-QR-URL: <url>
func (h *QRHandler) Handle(c *gin.Context) {
	// Parse demo parameter (default: false)
	demoStr := c.DefaultQuery("demo", "false")
	demo := demoStr == "true" || demoStr == "1"

	// Parse size parameter (default: 10, range: 4-40)
	sizeStr := c.DefaultQuery("size", "10")
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 4 || size > 40 {
		size = 10
	}

	url := getMapURL(demo)

	// Generate QR code PNG bytes
	// go-qrcode doesn't support box_size directly; we compute pixel size.
	// QR version auto-selects. For the URL length we use, version ~2 (25 modules).
	// Total pixels = (modules + 2*border) * box_size
	// We use RecoveryLevel Medium to match Python's ERROR_CORRECT_M.
	// Border of 2 modules (matching Python), default go-qrcode border is 4.
	// go-qrcode uses a fixed border, so we set the total image size.
	// Approximate: version 2 = 25 modules, border=2 => 29 * box_size
	// We'll use -size (negative) for auto-size with the library, but go-qrcode
	// takes a positive pixel size. We'll compute a reasonable size.
	//
	// Actually, go-qrcode.Encode takes total image pixel width.
	// Python: image size = (modules + 2*border) * box_size
	// With auto version for a short URL, typically version 2-3 (25-29 modules).
	// We approximate total pixels similarly.
	modules := 29 // reasonable default for short URLs (version 3)
	border := 2
	totalPixels := (modules + 2*border) * size

	pngBytes, err := qrcode.Encode(url, qrcode.Medium, totalPixels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate QR code"})
		return
	}

	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("X-QR-URL", url)
	c.Data(http.StatusOK, "image/png", pngBytes)
}

// Register adds the QR endpoint to the given Gin router group.
func (h *QRHandler) Register(r gin.IRouter) {
	r.GET("/api/qr", h.Handle)
}
