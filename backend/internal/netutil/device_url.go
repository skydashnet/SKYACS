package netutil

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/icholy/digest"
)

func DeriveDevicePassword(serialNumber, masterSecret string) (string, error) {
	if len(masterSecret) < 16 {
		return "", errors.New("connection request derivation secret must contain at least 16 characters")
	}
	mac := hmac.New(sha256.New, []byte(masterSecret))
	_, _ = mac.Write([]byte(serialNumber))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:18]), nil
}

// NewDeviceHTTPClient returns a redirect-safe client whose transport connects
// only to the IP addresses resolved and validated here. This closes the DNS
// rebinding window between URL validation and the actual socket connection.
func NewDeviceHTTPClient(rawURL, username, password string, timeout time.Duration) (*http.Client, error) {
	parsed, addresses, err := resolveDeviceURL(context.Background(), rawURL)
	if err != nil {
		return nil, err
	}
	hostname := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	dialer := &net.Dialer{Timeout: timeout / 2, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		requestedHost, requestedPort, err := net.SplitHostPort(address)
		if err != nil || !strings.EqualFold(strings.TrimSuffix(requestedHost, "."), strings.TrimSuffix(hostname, ".")) || requestedPort != port {
			return nil, fmt.Errorf("device URL redirect or destination change rejected")
		}
		var lastErr error
		for _, ip := range addresses {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, fmt.Errorf("device URL connection failed: %w", lastErr)
	}
	digestTransport := &digest.Transport{Username: username, Password: password, Transport: transport}
	return &http.Client{
		Timeout:   timeout,
		Transport: digestTransport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func resolveDeviceURL(parent context.Context, rawURL string) (*url.URL, []net.IP, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, nil, fmt.Errorf("invalid device URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, nil, fmt.Errorf("device URL must use HTTP or HTTPS")
	}
	if parsed.User != nil {
		return nil, nil, fmt.Errorf("device URL must not contain embedded credentials")
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return nil, nil, fmt.Errorf("invalid device URL port")
		}
	}

	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, parsed.Hostname())
	if err != nil || len(addresses) == 0 {
		return nil, nil, fmt.Errorf("device URL host cannot be resolved")
	}
	validated := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ip := address.IP
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return nil, nil, fmt.Errorf("device URL targets a restricted address")
		}
		allowed, err := connectionRequestAddressAllowed(ip)
		if err != nil {
			return nil, nil, err
		}
		if !allowed {
			return nil, nil, fmt.Errorf("device URL is outside CONNECTION_REQUEST_ALLOWED_CIDRS")
		}
		validated = append(validated, ip)
	}
	return parsed, validated, nil
}

func connectionRequestAddressAllowed(ip net.IP) (bool, error) {
	raw := strings.TrimSpace(os.Getenv("CONNECTION_REQUEST_ALLOWED_CIDRS"))
	if raw == "" {
		return true, nil
	}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if exact := net.ParseIP(item); exact != nil {
			if exact.Equal(ip) {
				return true, nil
			}
			continue
		}
		_, network, err := net.ParseCIDR(item)
		if err != nil {
			return false, fmt.Errorf("invalid CONNECTION_REQUEST_ALLOWED_CIDRS entry %q", item)
		}
		if network.Contains(ip) {
			return true, nil
		}
	}
	return false, nil
}
