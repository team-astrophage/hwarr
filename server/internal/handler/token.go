package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/homepy/hwarr/server/internal/auth"
)

// TokenHandler returns a gin handler that issues HMAC tokens for Socket.IO authentication.
//
//	GET /api/token
//	Response 200: {"token": "...", "user_id": "..."}
//	Response 500: {"error": "token generation failed"}
func TokenHandler(tokenService *auth.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, userID, err := tokenService.Issue()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token, "user_id": userID})
	}
}
