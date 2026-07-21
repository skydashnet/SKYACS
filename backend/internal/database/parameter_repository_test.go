package database

import (
	"encoding/json"
	"testing"

	"github.com/skydashnet/skyacs/internal/models"
)

func TestSensitiveParameterEncryptionRoundTrip(t *testing.T) {
	if err := ConfigureParameterEncryption("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	encrypted, err := encryptParameterValue("subscriber-secret")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "subscriber-secret" {
		t.Fatal("sensitive value was stored as plaintext")
	}
	decrypted, err := decryptParameterValue(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "subscriber-secret" {
		t.Fatalf("unexpected decrypted value %q", decrypted)
	}
}

func TestSensitiveSetTaskPayloadEncryptedAndRedacted(t *testing.T) {
	if err := ConfigureParameterEncryption("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTaskWithPayload(1, models.TaskTypeSetParameterValues, map[string]string{
		"Device.WiFi.AccessPoint.1.Security.KeyPassphrase": "wifi-secret",
		"Device.WiFi.SSID.1.SSID":                          "Office",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := encryptSetTaskPayload(task); err != nil {
		t.Fatal(err)
	}
	if string(task.Payload) == "" || string(task.Payload) == "wifi-secret" {
		t.Fatal("task payload was not encoded")
	}
	var stored map[string]string
	if err := json.Unmarshal(task.Payload, &stored); err != nil {
		t.Fatal(err)
	}
	if stored["Device.WiFi.AccessPoint.1.Security.KeyPassphrase"] == "wifi-secret" {
		t.Fatal("sensitive task value remained plaintext")
	}
	if err := decryptSetTaskPayload(task); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(task.Payload, &stored); err != nil || stored["Device.WiFi.AccessPoint.1.Security.KeyPassphrase"] != "wifi-secret" {
		t.Fatal("task payload did not decrypt for dispatch")
	}
	redactSetTaskPayload(task)
	if err := json.Unmarshal(task.Payload, &stored); err != nil || stored["Device.WiFi.AccessPoint.1.Security.KeyPassphrase"] != "[REDACTED]" {
		t.Fatal("task payload was not redacted for API history")
	}
}

func TestSensitiveProvisioningValueDecryptsForDispatch(t *testing.T) {
	if err := ConfigureParameterEncryption("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	encrypted, err := encryptParameterValue("wifi-secret")
	if err != nil {
		t.Fatal(err)
	}
	rules := []*models.ProvisioningRule{{ID: 1, ParameterName: "Device.WiFi.AccessPoint.1.Security.KeyPassphrase", ParameterValue: encrypted}}
	if err := decryptProvisioningRules(rules); err != nil {
		t.Fatal(err)
	}
	if rules[0].ParameterValue != "wifi-secret" {
		t.Fatal("sensitive provisioning value was not decrypted")
	}
}

func TestSensitiveParameterDetection(t *testing.T) {
	for _, name := range []string{"Device.WiFi.AccessPoint.1.Security.KeyPassphrase", "InternetGatewayDevice.WANPPPConnection.1.Password", "Device.Users.User.1.Username"} {
		if !IsSensitiveParameterName(name) {
			t.Fatalf("sensitive parameter %q was not detected", name)
		}
	}
	if IsSensitiveParameterName("Device.DeviceInfo.UpTime") {
		t.Fatal("non-sensitive telemetry was classified as sensitive")
	}
}
