package cwmp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// ParseSOAPEnvelope parse incoming SOAP/XML dari CPE
func ParseSOAPEnvelope(body io.Reader) (*SOAPEnvelope, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("gagal baca request body: %w", err)
	}

	if len(data) == 0 {
		return nil, nil // Empty request = session end signal
	}

	var envelope SOAPEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("gagal parse SOAP envelope: %w", err)
	}

	return &envelope, nil
}

// GenerateSOAPEnvelope generate SOAP response untuk dikirim ke CPE
func GenerateSOAPEnvelope(body interface{}) ([]byte, error) {
	envelope := SOAPEnvelope{
		Header: SOAPHeader{},
		Body:   SOAPBody{},
	}

	switch v := body.(type) {
	case *InformResponse:
		envelope.Body.InformResponse = v
	case *GetParameterValues:
		envelope.Body.GetParameterValues = v
	case *SetParameterValues:
		envelope.Body.SetParameterValues = v
	case *GetParameterNames:
		envelope.Body.GetParameterNames = v
	case *Reboot:
		envelope.Body.Reboot = v
	case *FactoryReset:
		envelope.Body.FactoryReset = v
	case *SOAPFault:
		envelope.Body.Fault = v
	default:
		return nil, fmt.Errorf("unknown body type: %T", body)
	}

	return marshalSOAP(&envelope)
}

func marshalSOAP(envelope *SOAPEnvelope) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n")

	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	if err := encoder.Encode(envelope); err != nil {
		return nil, fmt.Errorf("gagal encode SOAP envelope: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateEmptyResponse generate empty SOAP response (signal session end)
func GenerateEmptyResponse() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-0">
  <soap:Header/>
  <soap:Body/>
</soap:Envelope>`)
}

// DeteksiTipeMessage detect message type dari SOAP body
func DeteksiTipeMessage(body *SOAPBody) string {
	if body.Inform != nil {
		return "Inform"
	}
	if body.GetParameterValuesResp != nil {
		return "GetParameterValuesResponse"
	}
	if body.SetParameterValuesResp != nil {
		return "SetParameterValuesResponse"
	}
	if body.GetParameterNamesResp != nil {
		return "GetParameterNamesResponse"
	}
	if body.RebootResponse != nil {
		return "RebootResponse"
	}
	if body.FactoryResetResponse != nil {
		return "FactoryResetResponse"
	}
	if body.Fault != nil {
		return "Fault"
	}
	return "Unknown"
}
