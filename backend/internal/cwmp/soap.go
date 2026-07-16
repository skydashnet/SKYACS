package cwmp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	SOAPNamespace       = "http://schemas.xmlsoap.org/soap/envelope/"
	SOAPEncodingNS      = "http://schemas.xmlsoap.org/soap/encoding/"
	XMLSchemaNS         = "http://www.w3.org/2001/XMLSchema"
	XMLSchemaInstanceNS = "http://www.w3.org/2001/XMLSchema-instance"
	CWMPNamespace10     = "urn:dslforum-org:cwmp-1-0"
)

var supportedCWMPNamespaces = map[string]struct{}{
	"urn:dslforum-org:cwmp-1-0": {},
	"urn:dslforum-org:cwmp-1-1": {},
	"urn:dslforum-org:cwmp-1-2": {},
	"urn:dslforum-org:cwmp-1-3": {},
	"urn:dslforum-org:cwmp-1-4": {},
}

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
	namespace, err := detectCWMPNamespace(data)
	if err != nil {
		return nil, err
	}
	envelope.CWMPNamespace = namespace
	envelope.Header.Namespace = namespace
	envelope.Body.Namespace = namespace

	return &envelope, nil
}

func detectCWMPNamespace(data []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	inBody := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("gagal inspect CWMP namespace: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "Body" && start.Name.Space == SOAPNamespace {
			inBody = true
			continue
		}
		if !inBody || start.Name.Local == "Fault" {
			continue
		}
		if start.Name.Space == "" {
			return CWMPNamespace10, nil
		}
		if _, ok := supportedCWMPNamespaces[start.Name.Space]; !ok {
			return "", fmt.Errorf("unsupported CWMP namespace %q", start.Name.Space)
		}
		return start.Name.Space, nil
	}
	return CWMPNamespace10, nil
}

func GenerateSOAPEnvelopeForRequest(body interface{}, request *SOAPEnvelope) ([]byte, error) {
	namespace := CWMPNamespace10
	id := ""
	if request != nil {
		if request.CWMPNamespace != "" {
			namespace = request.CWMPNamespace
		}
		id = request.Header.ID
	}
	return GenerateSOAPEnvelopeWithContext(body, namespace, id)
}

func GenerateSOAPEnvelopeWithContext(body interface{}, namespace, id string) ([]byte, error) {
	if _, ok := supportedCWMPNamespaces[namespace]; !ok {
		namespace = CWMPNamespace10
	}
	envelope := SOAPEnvelope{
		CWMPNamespace: namespace,
		Header:        SOAPHeader{Namespace: namespace, ID: id},
		Body:          SOAPBody{Namespace: namespace},
	}

	switch v := body.(type) {
	case *InformResponse:
		envelope.Body.InformResponse = v
	case *GetParameterValues:
		envelope.Body.GetParameterValues = v
	case *SetParameterValues:
		v.ParameterList.Namespace = namespace
		envelope.Body.SetParameterValues = v
	case *GetParameterNames:
		envelope.Body.GetParameterNames = v
	case *Reboot:
		envelope.Body.Reboot = v
	case *FactoryReset:
		envelope.Body.FactoryReset = v
	case *TransferCompleteResponse:
		envelope.Body.TransferCompleteResp = v
	case *SOAPFault:
		v.Detail.Namespace = namespace
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

// DetectMessageType identifies the CWMP method carried by a SOAP body.
func DetectMessageType(body *SOAPBody) string {
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
	if body.DownloadResponse != nil {
		return "DownloadResponse"
	}
	if body.TransferComplete != nil {
		return "TransferComplete"
	}
	if body.Fault != nil {
		return "Fault"
	}
	return "Unknown"
}

func (header SOAPHeader) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {
	if start.Name.Local == "" {
		start.Name.Local = "Header"
	}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	if header.ID != "" {
		idStart := xml.StartElement{
			Name: xml.Name{Space: header.Namespace, Local: "ID"},
			Attr: []xml.Attr{{Name: xml.Name{Space: SOAPNamespace, Local: "mustUnderstand"}, Value: "1"}},
		}
		if err := encoder.EncodeElement(header.ID, idStart); err != nil {
			return err
		}
	}
	if header.HoldReqs != "" {
		if err := encoder.EncodeElement(header.HoldReqs, xml.StartElement{Name: xml.Name{Space: header.Namespace, Local: "HoldRequests"}}); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(start.End())
}

func (body SOAPBody) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {
	if start.Name.Local == "" {
		start.Name.Local = "Body"
	}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	var value interface{}
	method := ""
	switch {
	case body.Inform != nil:
		value, method = body.Inform, "Inform"
	case body.InformResponse != nil:
		value, method = body.InformResponse, "InformResponse"
	case body.GetParameterValues != nil:
		value, method = body.GetParameterValues, "GetParameterValues"
	case body.GetParameterValuesResp != nil:
		value, method = body.GetParameterValuesResp, "GetParameterValuesResponse"
	case body.SetParameterValues != nil:
		value, method = body.SetParameterValues, "SetParameterValues"
	case body.SetParameterValuesResp != nil:
		value, method = body.SetParameterValuesResp, "SetParameterValuesResponse"
	case body.GetParameterNames != nil:
		value, method = body.GetParameterNames, "GetParameterNames"
	case body.GetParameterNamesResp != nil:
		value, method = body.GetParameterNamesResp, "GetParameterNamesResponse"
	case body.Reboot != nil:
		value, method = body.Reboot, "Reboot"
	case body.RebootResponse != nil:
		value, method = body.RebootResponse, "RebootResponse"
	case body.FactoryReset != nil:
		value, method = body.FactoryReset, "FactoryReset"
	case body.FactoryResetResponse != nil:
		value, method = body.FactoryResetResponse, "FactoryResetResponse"
	case body.Download != nil:
		value, method = body.Download, "Download"
	case body.DownloadResponse != nil:
		value, method = body.DownloadResponse, "DownloadResponse"
	case body.TransferComplete != nil:
		value, method = body.TransferComplete, "TransferComplete"
	case body.TransferCompleteResp != nil:
		value, method = body.TransferCompleteResp, "TransferCompleteResponse"
	case body.Fault != nil:
		value, method = body.Fault, "Fault"
	}
	if value != nil {
		namespace := body.Namespace
		if method == "Fault" {
			namespace = SOAPNamespace
		}
		if err := encoder.EncodeElement(value, xml.StartElement{Name: xml.Name{Space: namespace, Local: method}}); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(start.End())
}

func (list ParameterList) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {
	start.Attr = append(start.Attr,
		xml.Attr{Name: xml.Name{Local: "xmlns:cwmp"}, Value: defaultNamespace(list.Namespace)},
		xml.Attr{Name: xml.Name{Local: "xmlns:soap-enc"}, Value: SOAPEncodingNS},
		xml.Attr{Name: xml.Name{Local: "soap-enc:arrayType"}, Value: "cwmp:ParameterValueStruct[" + strconv.Itoa(len(list.Parameters)) + "]"},
	)
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	for _, parameter := range list.Parameters {
		if err := encoder.Encode(parameter); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(start.End())
}

func (parameter ParameterValueStruct) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {
	start.Name.Local = "ParameterValueStruct"
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	if err := encoder.EncodeElement(parameter.Name, xml.StartElement{Name: xml.Name{Local: "Name"}}); err != nil {
		return err
	}
	valueType := canonicalXSDType(parameter.Type)
	valueStart := xml.StartElement{Name: xml.Name{Local: "Value"}, Attr: []xml.Attr{
		{Name: xml.Name{Local: "xmlns:xsi"}, Value: XMLSchemaInstanceNS},
		{Name: xml.Name{Local: "xmlns:xsd"}, Value: XMLSchemaNS},
		{Name: xml.Name{Local: "xsi:type"}, Value: "xsd:" + valueType},
	}}
	if err := encoder.EncodeElement(parameter.Value, valueStart); err != nil {
		return err
	}
	return encoder.EncodeToken(start.End())
}

func defaultNamespace(namespace string) string {
	if _, ok := supportedCWMPNamespaces[namespace]; ok {
		return namespace
	}
	return CWMPNamespace10
}

func canonicalXSDType(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "xsd:")
	switch value {
	case "boolean", "int", "unsignedInt", "long", "unsignedLong", "dateTime", "base64", "hexBinary":
		return value
	default:
		return "string"
	}
}
