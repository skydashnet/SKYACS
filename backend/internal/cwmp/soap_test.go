package cwmp

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestParseSOAPAcceptsCWMP12AndDetectsTransferComplete(t *testing.T) {
	payload := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-2">
  <soap:Header><cwmp:ID soap:mustUnderstand="1">request-42</cwmp:ID></soap:Header>
  <soap:Body><cwmp:TransferComplete><CommandKey>skyacs-task-9</CommandKey><FaultStruct><FaultCode>0</FaultCode><FaultString></FaultString></FaultStruct><StartTime>2026-01-01T00:00:00Z</StartTime><CompleteTime>2026-01-01T00:01:00Z</CompleteTime></cwmp:TransferComplete></soap:Body>
</soap:Envelope>`
	envelope, err := ParseSOAPEnvelope(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseSOAPEnvelope: %v", err)
	}
	if envelope.CWMPNamespace != "urn:dslforum-org:cwmp-1-2" {
		t.Fatalf("unexpected namespace %q", envelope.CWMPNamespace)
	}
	if envelope.Header.ID != "request-42" {
		t.Fatalf("unexpected ID %q", envelope.Header.ID)
	}
	if got := DetectMessageType(&envelope.Body); got != "TransferComplete" {
		t.Fatalf("unexpected message type %q", got)
	}
}

func TestSOAPResponseEchoesIDNamespaceAndTransferComplete(t *testing.T) {
	request := &SOAPEnvelope{CWMPNamespace: "urn:dslforum-org:cwmp-1-2", Header: SOAPHeader{ID: "request-42"}}
	encoded, err := GenerateSOAPEnvelopeForRequest(&TransferCompleteResponse{}, request)
	if err != nil {
		t.Fatalf("GenerateSOAPEnvelopeForRequest: %v", err)
	}
	if !bytes.Contains(encoded, []byte("request-42")) || !bytes.Contains(encoded, []byte("TransferCompleteResponse")) {
		t.Fatalf("response does not echo request metadata: %s", encoded)
	}
	if !bytes.Contains(encoded, []byte("urn:dslforum-org:cwmp-1-2")) {
		t.Fatalf("response lost CWMP namespace: %s", encoded)
	}
	var document interface{}
	if err := xml.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("generated SOAP is not well formed: %v\n%s", err, encoded)
	}
}

func TestSetParameterValuesIncludesSOAPTypesAndArrayMetadata(t *testing.T) {
	request := &SetParameterValues{
		ParameterList: ParameterList{Parameters: []ParameterValueStruct{
			{Name: "Device.ManagementServer.PeriodicInformEnable", Value: "true", Type: "boolean"},
		}},
		ParameterKey: "skyacs-task-10",
	}
	encoded, err := GenerateSOAPEnvelopeWithContext(request, CWMPNamespace10, "command-10")
	if err != nil {
		t.Fatalf("GenerateSOAPEnvelopeWithContext: %v", err)
	}
	for _, expected := range []string{"arrayType", "ParameterValueStruct[1]", "xsi:type", "xsd:boolean"} {
		if !bytes.Contains(encoded, []byte(expected)) {
			t.Fatalf("missing %q in SOAP: %s", expected, encoded)
		}
	}
	var document interface{}
	if err := xml.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("generated SOAP is not well formed: %v\n%s", err, encoded)
	}
}
