package cwmp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInformIsAcknowledgedBeforeACSRequest(t *testing.T) {
	handler := NewHandler(nil)
	inform := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-0">
  <soap:Header><cwmp:ID soap:mustUnderstand="1">inform-1</cwmp:ID></soap:Header>
  <soap:Body><cwmp:Inform>
    <DeviceId><Manufacturer>Lab</Manufacturer><OUI>001122</OUI><ProductClass>ONU</ProductClass><SerialNumber>TEST-1</SerialNumber></DeviceId>
    <Event><EventStruct><EventCode>0 BOOTSTRAP</EventCode><CommandKey></CommandKey></EventStruct></Event>
    <MaxEnvelopes>1</MaxEnvelopes><CurrentTime>2026-01-01T00:00:00Z</CurrentTime><RetryCount>0</RetryCount>
    <ParameterList><ParameterValueStruct><Name>Device.DeviceInfo.SerialNumber</Name><Value>TEST-1</Value></ParameterValueStruct></ParameterList>
  </cwmp:Inform></soap:Body>
</soap:Envelope>`

	request := httptest.NewRequest(http.MethodPost, "http://acs.test/", strings.NewReader(inform))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected Inform status %d: %s", response.StatusCode, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "InformResponse") || strings.Contains(recorder.Body.String(), "GetParameterValues") {
		t.Fatalf("Inform was not acknowledged before ACS request: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "inform-1") {
		t.Fatalf("Inform ID was not echoed: %s", recorder.Body.String())
	}

	cookies := response.Cookies()
	if len(cookies) == 0 {
		t.Fatal("CWMP session cookie was not set")
	}
	emptyRequest := httptest.NewRequest(http.MethodPost, "http://acs.test/", nil)
	emptyRequest.AddCookie(cookies[0])
	emptyRecorder := httptest.NewRecorder()
	handler.ServeHTTP(emptyRecorder, emptyRequest)
	if emptyRecorder.Code != http.StatusOK || !strings.Contains(emptyRecorder.Body.String(), "GetParameterValues") {
		t.Fatalf("ACS request was not dispatched after empty POST: status=%d body=%s", emptyRecorder.Code, emptyRecorder.Body.String())
	}
}
