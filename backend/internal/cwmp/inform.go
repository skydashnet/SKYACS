package cwmp

import (
	"log"
)

// ProcessInform handle Inform message dari CPE
// Return InformResponse untuk dikirim balik
func ProcessInform(inform *Inform) (*InformResponse, error) {
	log.Printf("Inform dari device: %s (SN: %s)",
		inform.DeviceId.Manufacturer,
		inform.DeviceId.SerialNumber,
	)

	// Log events
	for _, event := range inform.Event.Events {
		log.Printf("  Event: %s", event.EventCode)
	}

	// Extract important parameters
	params := EkstrakParameterPenting(inform.ParameterList.Parameters)
	log.Printf("  IP: %s, Firmware: %s",
		params["ExternalIPAddress"],
		params["SoftwareVersion"],
	)

	// TODO: Save/update device ke database
	// deviceRepo.Upsert(inform.DeviceId, params)

	// Return InformResponse
	return &InformResponse{
		MaxEnvelopes: 1,
	}, nil
}

// EkstrakParameterPenting extract commonly needed parameters
func EkstrakParameterPenting(params []ParameterValueStruct) map[string]string {
	result := make(map[string]string)

	parameterKeys := map[string]string{
		"InternetGatewayDevice.DeviceInfo.SoftwareVersion":          "SoftwareVersion",
		"InternetGatewayDevice.DeviceInfo.HardwareVersion":          "HardwareVersion",
		"InternetGatewayDevice.DeviceInfo.UpTime":                   "UpTime",
		"InternetGatewayDevice.ManagementServer.ConnectionRequestURL": "ConnectionRequestURL",
		// WANConnectionDevice.1
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.ExternalIPAddress": "ExternalIPAddress",
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.1.WANPPPConnection.1.ExternalIPAddress": "ExternalIPAddress",
		// WANConnectionDevice.2 (GM220-S, dll)
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.2.WANIPConnection.1.ExternalIPAddress": "ExternalIPAddress",
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.2.WANPPPConnection.1.ExternalIPAddress": "ExternalIPAddress",
		// WANConnectionDevice.3
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.3.WANIPConnection.1.ExternalIPAddress": "ExternalIPAddress",
		"InternetGatewayDevice.WANDevice.1.WANConnectionDevice.3.WANPPPConnection.1.ExternalIPAddress": "ExternalIPAddress",
		// Device:2 data model
		"Device.DeviceInfo.SoftwareVersion": "SoftwareVersion",
		"Device.DeviceInfo.HardwareVersion": "HardwareVersion",
		"Device.ManagementServer.ConnectionRequestURL": "ConnectionRequestURL",
	}

	for _, p := range params {
		if key, ok := parameterKeys[p.Name]; ok {
			result[key] = p.Value
		}
	}

	return result
}

// GetEventCodes extract event codes dari Inform
func GetEventCodes(inform *Inform) []string {
	codes := make([]string, 0, len(inform.Event.Events))
	for _, e := range inform.Event.Events {
		codes = append(codes, e.EventCode)
	}
	return codes
}

// IsBootstrap check apakah ini bootstrap event
func IsBootstrap(inform *Inform) bool {
	for _, e := range inform.Event.Events {
		if e.EventCode == EventBootstrap {
			return true
		}
	}
	return false
}

// IsPeriodic check apakah ini periodic inform
func IsPeriodic(inform *Inform) bool {
	for _, e := range inform.Event.Events {
		if e.EventCode == EventPeriodic {
			return true
		}
	}
	return false
}
