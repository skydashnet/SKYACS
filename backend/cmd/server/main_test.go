package main

import "testing"

func TestValidateCIDRListFailsClosed(t *testing.T) {
	if err := validateCIDRList("TEST_CIDRS", "10.0.0.0/8,192.0.2.10"); err != nil {
		t.Fatalf("valid CIDR list rejected: %v", err)
	}
	if err := validateCIDRList("TEST_CIDRS", "not-a-network"); err == nil {
		t.Fatal("invalid CIDR list was accepted")
	}
}

func TestEnvIntValidatesPoolBounds(t *testing.T) {
	t.Setenv("TEST_POOL_SIZE", "25")
	if value, err := envInt("TEST_POOL_SIZE", 10, 1, 50); err != nil || value != 25 {
		t.Fatalf("valid pool size rejected: value=%d err=%v", value, err)
	}
	t.Setenv("TEST_POOL_SIZE", "5000")
	if _, err := envInt("TEST_POOL_SIZE", 10, 1, 50); err == nil {
		t.Fatal("out-of-range pool size was accepted")
	}
}

func TestPostgreSQLVersionCompatibility(t *testing.T) {
	for _, versionNumber := range []int{140023, 150018, 160014, 170010, 180004, 190000} {
		major, err := validatePostgreSQLVersionNumber(versionNumber)
		if err != nil {
			t.Fatalf("supported PostgreSQL version %d rejected: %v", versionNumber, err)
		}
		if major != versionNumber/10000 {
			t.Fatalf("unexpected major for %d: %d", versionNumber, major)
		}
	}
	if _, err := validatePostgreSQLVersionNumber(130023); err == nil {
		t.Fatal("unsupported PostgreSQL 13 was accepted")
	}
}
