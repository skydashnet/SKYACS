package cwmp

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

// ProcessInform validates an Inform and creates the mandatory acknowledgement.
func ProcessInform(inform *Inform) (*InformResponse, error) {
	if err := validateInform(inform); err != nil {
		return nil, err
	}
	log.Printf("Inform from device: %s (SN: %s)",
		inform.DeviceId.Manufacturer,
		inform.DeviceId.SerialNumber,
	)

	// Log events
	for _, event := range inform.Event.Events {
		log.Printf("  Event: %s", event.EventCode)
	}

	// Extract important parameters
	params := ExtractImportantParameters(inform.ParameterList.Parameters)
	log.Printf("  IP: %s, Firmware: %s",
		params["ExternalIPAddress"],
		params["SoftwareVersion"],
	)

	// Return InformResponse
	return &InformResponse{
		MaxEnvelopes: 1,
	}, nil
}

func validateInform(inform *Inform) error {
	if inform == nil {
		return errors.New("missing Inform payload")
	}
	for label, value := range map[string]string{
		"serial number": inform.DeviceId.SerialNumber,
		"manufacturer":  inform.DeviceId.Manufacturer,
		"product class": inform.DeviceId.ProductClass,
		"OUI":           inform.DeviceId.OUI,
	} {
		if len(value) > 128 || strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("invalid device %s", label)
		}
	}
	if strings.TrimSpace(inform.DeviceId.SerialNumber) == "" {
		return errors.New("device serial number is required")
	}
	if len(inform.Event.Events) > 64 {
		return errors.New("too many Inform events")
	}
	if len(inform.ParameterList.Parameters) > 10000 {
		return errors.New("too many Inform parameters")
	}
	for _, parameter := range inform.ParameterList.Parameters {
		if len(parameter.Name) == 0 || len(parameter.Name) > 512 ||
			(!strings.HasPrefix(parameter.Name, "Device.") && !strings.HasPrefix(parameter.Name, "InternetGatewayDevice.")) {
			return fmt.Errorf("invalid Inform parameter name %q", parameter.Name)
		}
		if len(parameter.Value) > 64*1024 {
			return fmt.Errorf("Inform parameter %q exceeds 64 KiB", parameter.Name)
		}
	}
	return nil
}

func DetectDataModelRoot(params []ParameterValueStruct) string {
	for _, param := range params {
		if len(param.Name) >= len("InternetGatewayDevice.") && param.Name[:len("InternetGatewayDevice.")] == "InternetGatewayDevice." {
			return "InternetGatewayDevice."
		}
	}
	return "Device."
}

// ExtractImportantParameters extracts commonly needed parameters.
func ExtractImportantParameters(params []ParameterValueStruct) map[string]string {
	result := make(map[string]string)

	parameterKeys := map[string]string{
		"InternetGatewayDevice.DeviceInfo.SoftwareVersion":            "SoftwareVersion",
		"InternetGatewayDevice.DeviceInfo.HardwareVersion":            "HardwareVersion",
		"InternetGatewayDevice.DeviceInfo.UpTime":                     "UpTime",
		"InternetGatewayDevice.ManagementServer.ConnectionRequestURL": "ConnectionRequestURL",
		// WANConnectionDevice.1
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.ExternalIPAddress":  "ExternalIPAddress",
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.1.WANPPPConnection.1.ExternalIPAddress": "ExternalIPAddress",
		// WANConnectionDevice.2 (GM220-S, dll)
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.2.WANIPConnection.1.ExternalIPAddress":  "ExternalIPAddress",
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.2.WANPPPConnection.1.ExternalIPAddress": "ExternalIPAddress",
		// WANConnectionDevice.3
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.3.WANIPConnection.1.ExternalIPAddress":  "ExternalIPAddress",
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.3.WANPPPConnection.1.ExternalIPAddress": "ExternalIPAddress",
		// Device:2 data model
		"Device.DeviceInfo.SoftwareVersion":            "SoftwareVersion",
		"Device.DeviceInfo.HardwareVersion":            "HardwareVersion",
		"Device.ManagementServer.ConnectionRequestURL": "ConnectionRequestURL",
	}

	for _, p := range params {
		if key, ok := parameterKeys[p.Name]; ok {
			result[key] = p.Value
		}
	}

	return result
}
