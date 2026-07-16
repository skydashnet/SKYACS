package netutil

import "testing"

func TestDeviceHTTPClientRejectsRestrictedTargets(t *testing.T) {
	for _, target := range []string{"file:///etc/passwd", "http://127.0.0.1:8080/", "http://[::1]/", "http://169.254.1.1/"} {
		if _, err := NewDeviceHTTPClient(target, "", "", 10); err == nil {
			t.Fatalf("restricted target %q was accepted", target)
		}
	}
}

func TestDeriveDevicePasswordUsesMasterSecret(t *testing.T) {
	first, err := DeriveDevicePassword("SERIAL-1", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	second, err := DeriveDevicePassword("SERIAL-1", "fedcba9876543210fedcba9876543210")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("different master secrets produced the same device password")
	}
	if _, err := DeriveDevicePassword("SERIAL-1", "weak"); err == nil {
		t.Fatal("weak derivation secret was accepted")
	}
}

func TestDeviceHTTPClientHonorsDestinationAllowlist(t *testing.T) {
	t.Setenv("CONNECTION_REQUEST_ALLOWED_CIDRS", "198.51.100.0/24")
	if _, err := NewDeviceHTTPClient("http://192.0.2.10:7547/", "", "", 10); err == nil {
		t.Fatal("destination outside configured allowlist was accepted")
	}
	if _, err := NewDeviceHTTPClient("http://198.51.100.10:7547/", "", "", 10); err != nil {
		t.Fatalf("allowlisted destination rejected: %v", err)
	}
}
