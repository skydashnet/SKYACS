package api

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestValidUsername(t *testing.T) {
	for _, username := range []string{"net.admin", "operator-1", "noc_user"} {
		if !validUsername(username) {
			t.Fatalf("valid username %q rejected", username)
		}
	}
	for _, username := range []string{"x", "bad user", "operator/../../admin"} {
		if validUsername(username) {
			t.Fatalf("invalid username %q accepted", username)
		}
	}
}

func TestFirmwareTokenUsesCryptographicEntropy(t *testing.T) {
	token, err := newFirmwareToken()
	if err != nil {
		t.Fatalf("newFirmwareToken: %v", err)
	}
	decoded, err := hex.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("expected a 32-byte hex token, got %q", token)
	}
}

func TestLoginLimiter(t *testing.T) {
	limiter := newLoginLimiter(2, time.Hour)
	if !limiter.Allow("192.0.2.1") || !limiter.Allow("192.0.2.1") {
		t.Fatal("valid attempts rejected")
	}
	if limiter.Allow("192.0.2.1") {
		t.Fatal("rate limit was not enforced")
	}
	limiter.Reset("192.0.2.1")
	if !limiter.Allow("192.0.2.1") {
		t.Fatal("reset did not clear attempts")
	}
}
