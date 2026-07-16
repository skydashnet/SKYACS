package cwmp

import "encoding/xml"

// SOAP Envelope structure untuk TR-069/CWMP
type SOAPEnvelope struct {
	XMLName       xml.Name   `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	CWMPNamespace string     `xml:"-"`
	Header        SOAPHeader `xml:"Header"`
	Body          SOAPBody   `xml:"Body"`
}

type SOAPHeader struct {
	Namespace string `xml:"-"`
	ID        string `xml:"ID,omitempty"`
	HoldReqs  string `xml:"HoldRequests,omitempty"`
}

type SOAPBody struct {
	Namespace              string                    `xml:"-"`
	Inform                 *Inform                   `xml:"Inform,omitempty"`
	InformResponse         *InformResponse           `xml:"InformResponse,omitempty"`
	GetParameterValues     *GetParameterValues       `xml:"GetParameterValues,omitempty"`
	GetParameterValuesResp *GetParameterValuesResp   `xml:"GetParameterValuesResponse,omitempty"`
	SetParameterValues     *SetParameterValues       `xml:"SetParameterValues,omitempty"`
	SetParameterValuesResp *SetParameterValuesResp   `xml:"SetParameterValuesResponse,omitempty"`
	GetParameterNames      *GetParameterNames        `xml:"GetParameterNames,omitempty"`
	GetParameterNamesResp  *GetParameterNamesResp    `xml:"GetParameterNamesResponse,omitempty"`
	Reboot                 *Reboot                   `xml:"Reboot,omitempty"`
	RebootResponse         *RebootResponse           `xml:"RebootResponse,omitempty"`
	FactoryReset           *FactoryReset             `xml:"FactoryReset,omitempty"`
	FactoryResetResponse   *FactoryResetResponse     `xml:"FactoryResetResponse,omitempty"`
	Download               *Download                 `xml:"Download,omitempty"`
	DownloadResponse       *DownloadResponse         `xml:"DownloadResponse,omitempty"`
	TransferComplete       *TransferComplete         `xml:"TransferComplete,omitempty"`
	TransferCompleteResp   *TransferCompleteResponse `xml:"TransferCompleteResponse,omitempty"`
	Fault                  *SOAPFault                `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault,omitempty"`
}

// Inform message dari CPE
type Inform struct {
	DeviceId      DeviceId      `xml:"DeviceId"`
	Event         EventList     `xml:"Event"`
	MaxEnvelopes  int           `xml:"MaxEnvelopes"`
	CurrentTime   string        `xml:"CurrentTime"`
	RetryCount    int           `xml:"RetryCount"`
	ParameterList ParameterList `xml:"ParameterList"`
}

type DeviceId struct {
	Manufacturer string `xml:"Manufacturer"`
	OUI          string `xml:"OUI"`
	ProductClass string `xml:"ProductClass"`
	SerialNumber string `xml:"SerialNumber"`
}

type EventList struct {
	Events []EventStruct `xml:"EventStruct"`
}

type EventStruct struct {
	EventCode  string `xml:"EventCode"`
	CommandKey string `xml:"CommandKey"`
}

type ParameterList struct {
	Namespace  string                 `xml:"-"`
	Parameters []ParameterValueStruct `xml:"ParameterValueStruct"`
}

type ParameterValueStruct struct {
	Name  string `xml:"Name"`
	Value string `xml:"Value"`
	Type  string `xml:"-"`
}

// InformResponse dari ACS ke CPE
type InformResponse struct {
	MaxEnvelopes int `xml:"MaxEnvelopes"`
}

// GetParameterValues - ACS request ke CPE
type GetParameterValues struct {
	ParameterNames []string `xml:"ParameterNames>string"`
}

// GetParameterValuesResponse - CPE response
type GetParameterValuesResp struct {
	ParameterList ParameterList `xml:"ParameterList"`
}

// SetParameterValues - ACS request ke CPE
type SetParameterValues struct {
	ParameterList ParameterList `xml:"ParameterList"`
	ParameterKey  string        `xml:"ParameterKey"`
}

// SetParameterValuesResponse - CPE response
type SetParameterValuesResp struct {
	Status int `xml:"Status"`
}

// GetParameterNames - untuk device discovery
type GetParameterNames struct {
	ParameterPath string `xml:"ParameterPath"`
	NextLevel     bool   `xml:"NextLevel"`
}

type GetParameterNamesResp struct {
	ParameterList []ParameterInfoStruct `xml:"ParameterList>ParameterInfoStruct"`
}

type ParameterInfoStruct struct {
	Name     string `xml:"Name"`
	Writable bool   `xml:"Writable"`
}

// SOAP Fault untuk error handling
type SOAPFault struct {
	FaultCode   string      `xml:"faultcode"`
	FaultString string      `xml:"faultstring"`
	Detail      FaultDetail `xml:"detail,omitempty"`
}

type FaultDetail struct {
	Namespace string     `xml:"-"`
	CWMPFault *CWMPFault `xml:"Fault,omitempty"`
}

type CWMPFault struct {
	FaultCode   string `xml:"FaultCode"`
	FaultString string `xml:"FaultString"`
}

// Event codes constants
const (
	EventBootstrap        = "0 BOOTSTRAP"
	EventBoot             = "1 BOOT"
	EventPeriodic         = "2 PERIODIC"
	EventScheduled        = "3 SCHEDULED"
	EventValueChange      = "4 VALUE CHANGE"
	EventKicked           = "5 KICKED"
	EventConnectionReq    = "6 CONNECTION REQUEST"
	EventTransferComplete = "7 TRANSFER COMPLETE"
	EventDiagComplete     = "8 DIAGNOSTICS COMPLETE"
)

// Reboot - ACS command ke CPE
type Reboot struct {
	CommandKey string `xml:"CommandKey"`
}

// RebootResponse - CPE response
type RebootResponse struct{}

// FactoryReset - ACS command ke CPE
type FactoryReset struct{}

// FactoryResetResponse - CPE response
type FactoryResetResponse struct{}

// Download - ACS command ke CPE untuk download firmware/config
type Download struct {
	CommandKey     string `xml:"CommandKey"`
	FileType       string `xml:"FileType"` // "1 Firmware Upgrade Image", "3 Vendor Configuration File"
	URL            string `xml:"URL"`
	Username       string `xml:"Username"`
	Password       string `xml:"Password"`
	FileSize       int64  `xml:"FileSize"`
	TargetFileName string `xml:"TargetFileName"`
	DelaySeconds   int    `xml:"DelaySeconds"`
	SuccessURL     string `xml:"SuccessURL"`
	FailureURL     string `xml:"FailureURL"`
}

// DownloadResponse - CPE response
type DownloadResponse struct {
	Status       int    `xml:"Status"` // 0=completed, 1=download started
	StartTime    string `xml:"StartTime"`
	CompleteTime string `xml:"CompleteTime"`
}

// TransferComplete - CPE notification setelah download selesai
type TransferComplete struct {
	CommandKey   string      `xml:"CommandKey"`
	FaultStruct  FaultStruct `xml:"FaultStruct"`
	StartTime    string      `xml:"StartTime"`
	CompleteTime string      `xml:"CompleteTime"`
}

type FaultStruct struct {
	FaultCode   int    `xml:"FaultCode"`
	FaultString string `xml:"FaultString"`
}

// TransferCompleteResponse
type TransferCompleteResponse struct{}
