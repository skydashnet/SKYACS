package netutil

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"
)

// ValidateDeviceURL permits device-local and public HTTP(S) endpoints while
// preventing a compromised CPE from targeting services on the ACS host itself.
func ValidateDeviceURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("invalid device URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("device URL must use HTTP or HTTPS")
	}
	if parsed.User != nil {
		return fmt.Errorf("device URL must not contain embedded credentials")
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return fmt.Errorf("invalid device URL port")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, parsed.Hostname())
	if err != nil || len(addresses) == 0 {
		return fmt.Errorf("device URL host cannot be resolved")
	}
	for _, address := range addresses {
		ip := address.IP
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("device URL targets a restricted address")
		}
	}
	return nil
}
