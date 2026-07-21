package cwmp

import (
	"net/http/httptest"
	"testing"
)

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

func TestForwardedHTTPSRequiresTrustedProxy(t *testing.T) {
	request := httptest.NewRequest("POST", "http://skyacs/", nil)
	request.RemoteAddr = "198.51.100.10:1234"
	request.Header.Set("X-Forwarded-Proto", "https")
	handler := &Handler{trustedProxies: parseAllowedNetworks("127.0.0.1/32")}
	if handler.isSecureRequest(request) {
		t.Fatal("untrusted proxy was allowed to spoof HTTPS")
	}
	request.RemoteAddr = "127.0.0.1:1234"
	if !handler.isSecureRequest(request) {
		t.Fatal("trusted HTTPS proxy was rejected")
	}
}

func TestCWMPClientAddressRejectsSpoofedForwardedHop(t *testing.T) {
	request := httptest.NewRequest("POST", "http://skyacs/", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.66, 198.51.100.25, 127.0.0.1")
	handler := &Handler{trustedProxies: parseAllowedNetworks("127.0.0.1/32")}
	if got := handler.clientAddress(request); got != "198.51.100.25" {
		t.Fatalf("unexpected forwarded client address %q", got)
	}
}

func TestInformValidationRejectsOversizedIdentityAndInvalidParameter(t *testing.T) {
	inform := &Inform{DeviceId: DeviceId{SerialNumber: "serial\nspoof"}}
	if err := validateInform(inform); err == nil {
		t.Fatal("log-injection serial was accepted")
	}
	inform = &Inform{DeviceId: DeviceId{SerialNumber: "SERIAL-1"}, ParameterList: ParameterList{Parameters: []ParameterValueStruct{{Name: "invalid.root", Value: "x"}}}}
	if err := validateInform(inform); err == nil {
		t.Fatal("invalid parameter root was accepted")
	}
}

func TestSessionStateDoesNotCrossDeviceIdentity(t *testing.T) {
	manager := &SessionManager{sessions: make(map[string]*Session)}
	first := manager.GetOrCreate("cookie", "SERIAL-A")
	first.mu.Lock()
	first.DeviceID = 42
	first.CurrentTaskID = 99
	first.mu.Unlock()
	second := manager.GetOrCreate("cookie", "SERIAL-B")
	if first == second || second.SerialNumber != "SERIAL-B" || second.DeviceID != 0 || second.CurrentTaskID != 0 {
		t.Fatal("session state crossed device identity")
	}
}

func TestSessionManagerHasBoundedSize(t *testing.T) {
	manager := &SessionManager{sessions: make(map[string]*Session), maxSize: 2}
	manager.GetOrCreate("first", "SERIAL-1")
	manager.GetOrCreate("second", "SERIAL-2")
	manager.GetOrCreate("third", "SERIAL-3")
	if len(manager.sessions) != 2 {
		t.Fatalf("expected bounded session map, got %d entries", len(manager.sessions))
	}
}
