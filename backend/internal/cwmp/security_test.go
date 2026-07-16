package cwmp

import "testing"

func TestDetectDataModelRoot(t *testing.T) {
	if got := DetectDataModelRoot([]ParameterValueStruct{{Name: "InternetGatewayDevice.DeviceInfo.UpTime"}}); got != "InternetGatewayDevice." {
		t.Fatalf("unexpected root %q", got)
	}
	if got := DetectDataModelRoot([]ParameterValueStruct{{Name: "Device.DeviceInfo.UpTime"}}); got != "Device." {
		t.Fatalf("unexpected root %q", got)
	}
}

func TestAllowedNetworkParsing(t *testing.T) {
	networks := parseAllowedNetworks("10.0.0.0/8, 192.0.2.10, invalid")
	if len(networks) != 2 {
		t.Fatalf("expected 2 networks, got %d", len(networks))
	}
	handler := &Handler{allowedNetworks: networks}
	if !handler.isNetworkAllowed("10.20.30.40:7547") {
		t.Fatal("allowed network rejected")
	}
	if handler.isNetworkAllowed("203.0.113.1:7547") {
		t.Fatal("outside network accepted")
	}
}

func TestSecureEqual(t *testing.T) {
	if !secureEqual("credential", "credential") {
		t.Fatal("equal values rejected")
	}
	if secureEqual("credential", "different") {
		t.Fatal("different values accepted")
	}
}
