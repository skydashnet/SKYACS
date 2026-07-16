package netutil

import "testing"

func TestValidateDeviceURLRejectsRestrictedTargets(t *testing.T) {
	for _, target := range []string{"file:///etc/passwd", "http://127.0.0.1:8080/", "http://[::1]/", "http://169.254.1.1/"} {
		if err := ValidateDeviceURL(target); err == nil {
			t.Fatalf("restricted target %q was accepted", target)
		}
	}
}
