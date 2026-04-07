package auth

import (
	"strings"
	"testing"
	"time"
)

func TestIssueAndValidate(t *testing.T) {
	ts := NewTokenService("test-secret", 30)

	token, userID, err := ts.Issue()
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}
	if token == "" || userID == "" {
		t.Fatal("Issue() returned empty token or userID")
	}

	validatedID, err := ts.Validate(token)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if validatedID != userID {
		t.Errorf("Validate() returned %q, want %q", validatedID, userID)
	}
}

func TestValidateForgedSignature(t *testing.T) {
	ts := NewTokenService("test-secret", 30)

	token, _, err := ts.Issue()
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}

	// Tamper with the signature
	parts := strings.SplitN(token, ":", 3)
	forged := parts[0] + ":" + parts[1] + ":deadbeef"

	_, err = ts.Validate(forged)
	if err == nil {
		t.Fatal("Validate() should fail for forged signature")
	}
	if !strings.Contains(err.Error(), "invalid signature") {
		t.Errorf("expected 'invalid signature' error, got: %v", err)
	}
}

func TestValidateExpiredToken(t *testing.T) {
	// TTL of 0 minutes means tokens expire immediately
	ts := &TokenService{
		secret: []byte("test-secret"),
		ttl:    -1 * time.Second, // already expired
	}

	token, _, err := ts.Issue()
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}

	_, err = ts.Validate(token)
	if err == nil {
		t.Fatal("Validate() should fail for expired token")
	}
	if !strings.Contains(err.Error(), "token expired") {
		t.Errorf("expected 'token expired' error, got: %v", err)
	}
}

func TestValidateInvalidFormat(t *testing.T) {
	ts := NewTokenService("test-secret", 30)

	cases := []string{
		"",
		"only-one-part",
		"two:parts",
	}

	for _, tc := range cases {
		_, err := ts.Validate(tc)
		if err == nil {
			t.Errorf("Validate(%q) should fail", tc)
		}
	}
}

func TestDifferentSecretRejectsToken(t *testing.T) {
	ts1 := NewTokenService("secret-one", 30)
	ts2 := NewTokenService("secret-two", 30)

	token, _, err := ts1.Issue()
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}

	_, err = ts2.Validate(token)
	if err == nil {
		t.Fatal("Validate() should fail with different secret")
	}
}
