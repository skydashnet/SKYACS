package api

import (
	"crypto/tls"
	"encoding/hex"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestSameOrigin(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32")

	direct := httptest.NewRequest("GET", "http://acs.example.test:7548/health", nil)
	if !isSameOrigin(direct, "http://acs.example.test:7548") {
		t.Fatal("direct same-origin request was rejected")
	}
	if isSameOrigin(direct, "https://evil.example.test") {
		t.Fatal("cross-origin request was accepted")
	}

	proxied := httptest.NewRequest("GET", "http://acs.example.test:8080/api/health", nil)
	proxied.RemoteAddr = "127.0.0.1:41234"
	proxied.Header.Set("X-Forwarded-Proto", "http")
	if !isSameOrigin(proxied, "http://acs.example.test:8080") {
		t.Fatal("trusted reverse-proxy origin was rejected")
	}

	untrusted := httptest.NewRequest("GET", "http://acs.example.test/health", nil)
	untrusted.RemoteAddr = "203.0.113.10:41234"
	untrusted.Header.Set("X-Forwarded-Proto", "https")
	if isSameOrigin(untrusted, "https://acs.example.test") {
		t.Fatal("untrusted forwarded protocol was accepted")
	}

	tlsRequest := httptest.NewRequest("GET", "https://acs.example.test/health", nil)
	tlsRequest.TLS = &tls.ConnectionState{}
	if !isSameOrigin(tlsRequest, "https://acs.example.test") {
		t.Fatal("TLS same-origin request was rejected")
	}
}

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

func TestClientIPOnlyTrustsConfiguredProxy(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	trusted := httptest.NewRequest("GET", "/", nil)
	trusted.RemoteAddr = "127.0.0.1:1234"
	trusted.Header.Set("X-Forwarded-For", "203.0.113.66, 198.51.100.25, 127.0.0.1")
	if got := clientIP(trusted); got != "198.51.100.25" {
		t.Fatalf("trusted proxy address not used: %q", got)
	}

	untrusted := httptest.NewRequest("GET", "/", nil)
	untrusted.RemoteAddr = "203.0.113.9:1234"
	untrusted.Header.Set("X-Forwarded-For", "198.51.100.25")
	if got := clientIP(untrusted); got != "203.0.113.9" {
		t.Fatalf("untrusted proxy spoofed client address: %q", got)
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
	for attempt := 0; attempt < 2; attempt++ {
		if !limiter.Allow("192.0.2.1") {
			t.Fatal("valid attempt rejected")
		}
	}
	if limiter.Allow("192.0.2.1") {
		t.Fatal("rate limit was not enforced")
	}
	limiter.Reset("192.0.2.1")
	if !limiter.Allow("192.0.2.1") {
		t.Fatal("reset did not clear attempts")
	}
}

func TestLoginLimiterHasBoundedMemory(t *testing.T) {
	limiter := newLoginLimiter(1, time.Hour)
	for index := 0; index < maxLimiterEntries; index++ {
		if !limiter.Allow(strconv.Itoa(index)) {
			t.Fatalf("entry %d rejected before the cap", index)
		}
	}
	if limiter.Allow("over-cap") {
		t.Fatal("limiter accepted an entry beyond its memory cap")
	}
}
