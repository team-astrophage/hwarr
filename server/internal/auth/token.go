package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TokenService issues and validates HMAC-signed tokens for Socket.IO authentication.
type TokenService struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenService creates a TokenService with the given secret and TTL in minutes.
func NewTokenService(secret string, ttlMinutes int) *TokenService {
	return &TokenService{
		secret: []byte(secret),
		ttl:    time.Duration(ttlMinutes) * time.Minute,
	}
}

// Issue generates a new token and user ID.
// The token format is: userID:exp:signature
func (ts *TokenService) Issue() (token string, userID string, err error) {
	userID, err = generateUUID()
	if err != nil {
		return "", "", fmt.Errorf("generate user id: %w", err)
	}

	exp := time.Now().Add(ts.ttl).Unix()
	payload := userID + ":" + strconv.FormatInt(exp, 10)

	mac := hmac.New(sha256.New, ts.secret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	token = payload + ":" + sig
	return token, userID, nil
}

// Validate checks the token signature and expiration, returning the user ID.
func (ts *TokenService) Validate(token string) (string, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	userID, expStr, sig := parts[0], parts[1], parts[2]
	payload := userID + ":" + expStr

	mac := hmac.New(sha256.New, ts.secret)
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", fmt.Errorf("invalid signature")
	}

	expInt, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid expiration: %w", err)
	}

	if time.Now().Unix() > expInt {
		return "", fmt.Errorf("token expired")
	}

	return userID, nil
}

// generateUUID produces a UUID v4 using crypto/rand.
func generateUUID() (string, error) {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return "", err
	}
	// Set version 4
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant bits
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}
