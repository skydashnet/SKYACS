package cwmp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/skydashnet/miniacs/internal/database"
	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
)

type Handler struct {
	sessions         *SessionManager
	deviceRepo       *database.DeviceRepository
	taskRepo         *database.TaskRepository
	parameterRepo    *database.ParameterRepository
	provisioningRepo *database.ProvisioningRepository
	faultRepo        *database.FaultRepository
	blockedRepo      *database.BlockedDeviceRepository
	cwmpUsername     string
	cwmpPassword     string
	allowedNetworks  []*net.IPNet
}

func NewHandler(db *gorm.DB) *Handler {
	var deviceRepo *database.DeviceRepository
	var taskRepo *database.TaskRepository
	var parameterRepo *database.ParameterRepository
	var provisioningRepo *database.ProvisioningRepository
	var faultRepo *database.FaultRepository
	var blockedRepo *database.BlockedDeviceRepository

	if db != nil {
		deviceRepo = database.NewDeviceRepository(db)
		taskRepo = database.NewTaskRepository(db)
		parameterRepo = database.NewParameterRepository(db)
		provisioningRepo = database.NewProvisioningRepository(db)
		faultRepo = database.NewFaultRepository(db)
		blockedRepo = database.NewBlockedDeviceRepository(db)

	}

	return &Handler{
		sessions:         NewSessionManager(30 * time.Second),
		deviceRepo:       deviceRepo,
		taskRepo:         taskRepo,
		parameterRepo:    parameterRepo,
		provisioningRepo: provisioningRepo,
		faultRepo:        faultRepo,
		blockedRepo:      blockedRepo,
		cwmpUsername:     os.Getenv("CWMP_USERNAME"),
		cwmpPassword:     os.Getenv("CWMP_PASSWORD"),
		allowedNetworks:  parseAllowedNetworks(os.Getenv("CWMP_ALLOWED_CIDRS")),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.isNetworkAllowed(r.RemoteAddr) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if h.cwmpUsername != "" || h.cwmpPassword != "" {
		username, password, ok := r.BasicAuth()
		if !ok || !secureEqual(username, h.cwmpUsername) || !secureEqual(password, h.cwmpPassword) {
			w.Header().Set("WWW-Authenticate", `Basic realm="miniACS CWMP"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionKey := ""
	if cookie, err := r.Cookie("cwmp_session"); err == nil {
		sessionKey = cookie.Value
	}
	if sessionKey == "" {
		sessionKey = newSessionKey()
		http.SetCookie(w, &http.Cookie{
			Name:     "cwmp_session",
			Value:    sessionKey,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
			SameSite: http.SameSiteStrictMode,
			MaxAge:   300,
		})
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	envelope, err := ParseSOAPEnvelope(r.Body)
	if err != nil {
		log.Printf("Error parsing SOAP: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if envelope == nil {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	response, err := h.routeMessage(envelope, sessionKey)
	if err != nil {
		log.Printf("Error processing message: %v", err)
		h.sendFault(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	if response == nil {
		w.Write(GenerateEmptyResponse())
		return
	}

	responseBytes, err := GenerateSOAPEnvelope(response)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Write(responseBytes)
}

func (h *Handler) routeMessage(envelope *SOAPEnvelope, remoteAddr string) (interface{}, error) {
	msgType := DeteksiTipeMessage(&envelope.Body)
	log.Printf("Received CWMP message: %s from %s", msgType, remoteAddr)

	switch msgType {
	case "Inform":
		return h.handleInform(envelope.Body.Inform, remoteAddr)

	case "GetParameterValuesResponse":
		return h.handleGetParameterValuesResponse(envelope.Body.GetParameterValuesResp, remoteAddr)

	case "GetParameterNamesResponse":
		return h.handleGetParameterNamesResponse(envelope.Body.GetParameterNamesResp, remoteAddr)

	case "SetParameterValuesResponse":
		return h.handleSetParameterValuesResponse(envelope.Body.SetParameterValuesResp, remoteAddr)

	case "RebootResponse":
		return h.handleRebootResponse(remoteAddr)

	case "FactoryResetResponse":
		return h.handleFactoryResetResponse(remoteAddr)

	case "DownloadResponse":
		return h.handleDownloadResponse(envelope.Body.DownloadResponse, remoteAddr)

	case "TransferComplete":
		return h.handleTransferComplete(envelope.Body.TransferComplete, remoteAddr)

	case "Fault":
		return h.handleFault(envelope.Body.Fault, remoteAddr)

	default:
		log.Printf("Unknown message type: %s", msgType)
		return nil, nil
	}
}

func (h *Handler) handleInform(inform *Inform, remoteAddr string) (interface{}, error) {
	if inform == nil || inform.DeviceId.SerialNumber == "" {
		return nil, fmt.Errorf("device serial number is required")
	}
	if h.blockedRepo != nil {
		blocked, err := h.blockedRepo.IsBlocked(context.Background(), inform.DeviceId.SerialNumber)
		if err != nil {
			return nil, fmt.Errorf("check device admission: %w", err)
		}
		if blocked {
			log.Printf("Blocked device rejected: %s", inform.DeviceId.SerialNumber)
			return &InformResponse{MaxEnvelopes: 1}, nil
		}
	}

	response, err := ProcessInform(inform)
	if err != nil {
		return nil, err
	}

	var deviceID int64

	if h.deviceRepo != nil {
		params := EkstrakParameterPenting(inform.ParameterList.Parameters)

		var ipAddrStr *string
		if ip := params["ExternalIPAddress"]; ip != "" {
			ipAddrStr = &ip
		}

		device := &models.Device{
			SerialNumber:         inform.DeviceId.SerialNumber,
			OUI:                  inform.DeviceId.OUI,
			Manufacturer:         strPtr(inform.DeviceId.Manufacturer),
			ProductClass:         strPtr(inform.DeviceId.ProductClass),
			HardwareVersion:      strPtr(params["HardwareVersion"]),
			SoftwareVersion:      strPtr(params["SoftwareVersion"]),
			IPAddress:            ipAddrStr,
			ConnectionRequestURL: strPtr(params["ConnectionRequestURL"]),
		}

		if err := h.deviceRepo.UpsertFromInform(context.Background(), device); err != nil {
			log.Printf("Error saving device: %v", err)
		} else {
			deviceID = device.ID
			log.Printf("Device saved: %s (ID: %d)", device.SerialNumber, device.ID)
		}

		if h.parameterRepo != nil && deviceID > 0 && len(inform.ParameterList.Parameters) > 0 {
			paramsToSave := make([]models.DeviceParameter, 0, len(inform.ParameterList.Parameters))
			for _, parameter := range inform.ParameterList.Parameters {
				paramsToSave = append(paramsToSave, models.DeviceParameter{
					DeviceID: deviceID,
					Name:     parameter.Name,
					Value:    parameter.Value,
				})
			}
			if err := h.parameterRepo.UpsertMany(context.Background(), deviceID, paramsToSave); err != nil {
				log.Printf("Error saving Inform parameters: %v", err)
			}
		}
	}

	session := h.sessions.GetOrCreate(remoteAddr, inform.DeviceId.SerialNumber)
	session.State = StateInformReceived
	session.DeviceID = deviceID
	session.DataModelRoot = DetectDataModelRoot(inform.ParameterList.Parameters)

	if h.taskRepo != nil && deviceID > 0 {
		tasks, err := h.taskRepo.GetPendingByDeviceID(context.Background(), deviceID)
		if err != nil {
			log.Printf("Error getting pending tasks: %v", err)
		} else if len(tasks) > 0 {
			log.Printf("Found %d pending tasks for device %d", len(tasks), deviceID)
			task := tasks[0]
			session.CurrentTaskID = task.ID
			session.State = StateProcessingTasks

			h.taskRepo.UpdateStatus(context.Background(), task.ID, models.TaskStatusSent, nil, "")

			return h.buildTaskRequest(task)
		}

		if h.provisioningRepo != nil {
			rules, err := h.provisioningRepo.ListEnabled(context.Background())
			if err == nil && len(rules) > 0 {
				log.Printf("Applying %d provisioning rules to device %s", len(rules), inform.DeviceId.SerialNumber)

				spv := &SetParameterValues{ParameterKey: "auto-provisioning"}
				for _, rule := range rules {
					spv.ParameterList.Parameters = append(spv.ParameterList.Parameters, ParameterValueStruct{
						Name:  rule.ParameterName,
						Value: rule.ParameterValue,
					})
				}

				if len(spv.ParameterList.Parameters) > 0 {
					session.State = StateProcessingTasks
					return spv, nil
				}
			}
		}

		shouldAutoFetch := false
		for _, event := range inform.Event.Events {
			if event.EventCode == EventBootstrap || event.EventCode == EventPeriodic || event.EventCode == EventConnectionReq {
				shouldAutoFetch = true
				break
			}
		}

		if shouldAutoFetch {
			log.Printf("Auto-fetching parameters for device %d", deviceID)
			session.State = StateProcessingTasks
			session.AutoFetchPhase = 1

			return &GetParameterValues{ParameterNames: []string{session.DataModelRoot}}, nil
		}
	}

	return response, nil
}

func (h *Handler) handleGetParameterValuesResponse(resp *GetParameterValuesResp, remoteAddr string) (interface{}, error) {
	log.Printf("Received GetParameterValuesResponse with %d parameters from %s", len(resp.ParameterList.Parameters), remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		log.Printf("No active session found for %s", remoteAddr)
		return nil, nil
	}

	if session.State != StateProcessingTasks {
		log.Printf("Session for %s not in processing state", remoteAddr)
		return nil, nil
	}

	if h.parameterRepo != nil && session.DeviceID > 0 {
		params := make([]models.DeviceParameter, 0, len(resp.ParameterList.Parameters))
		for _, p := range resp.ParameterList.Parameters {
			params = append(params, models.DeviceParameter{
				DeviceID: session.DeviceID,
				Name:     p.Name,
				Value:    p.Value,
			})
		}
		if err := h.parameterRepo.UpsertMany(context.Background(), session.DeviceID, params); err != nil {
			log.Printf("Error saving parameters: %v", err)
		} else {
			log.Printf("Saved %d parameters for device %d (serial: %s)", len(params), session.DeviceID, session.SerialNumber)
		}
	}

	if session.AutoFetchPhase > 0 {
		return h.continueAutoFetch(session)
	}
	if session.CurrentTaskID > 0 && h.taskRepo != nil {
		result := make(map[string]string)
		for _, p := range resp.ParameterList.Parameters {
			result[p.Name] = p.Value
		}
		h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, result, "")
	}

	return h.getNextTask(context.Background(), session)
}

func (h *Handler) continueAutoFetch(session *Session) (interface{}, error) {
	if session.AutoFetchPhase == 1 {
		session.AutoFetchPhase = 2
		return &GetParameterNames{ParameterPath: session.DataModelRoot, NextLevel: false}, nil
	}
	session.AutoFetchPhase = 0
	session.State = StateIdle
	return nil, nil
}

func (h *Handler) handleGetParameterNamesResponse(resp *GetParameterNamesResp, remoteAddr string) (interface{}, error) {
	if resp == nil {
		return nil, nil
	}
	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}
	if h.parameterRepo != nil && session.DeviceID > 0 {
		params := make([]models.DeviceParameter, 0, len(resp.ParameterList))
		for _, parameter := range resp.ParameterList {
			writable := parameter.Writable
			params = append(params, models.DeviceParameter{Name: parameter.Name, Writable: &writable})
		}
		if err := h.parameterRepo.UpsertWritable(context.Background(), session.DeviceID, params); err != nil {
			log.Printf("Error saving writable parameter flags: %v", err)
		}
	}
	session.AutoFetchPhase = 0
	session.State = StateIdle
	return h.getNextTask(context.Background(), session)
}

func (h *Handler) handleSetParameterValuesResponse(resp *SetParameterValuesResp, remoteAddr string) (interface{}, error) {
	log.Printf("SetParameterValues status: %d from %s", resp.Status, remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}

	if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
		if h.taskRepo != nil {
			h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, map[string]int{"status": resp.Status}, "")
		}
		return h.getNextTask(context.Background(), session)
	}

	return nil, nil
}

func (h *Handler) handleRebootResponse(remoteAddr string) (interface{}, error) {
	log.Printf("Received RebootResponse from %s", remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}

	if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
		if h.taskRepo != nil {
			h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, nil, "")
		}
		return h.getNextTask(context.Background(), session)
	}

	return nil, nil
}

func (h *Handler) handleFactoryResetResponse(remoteAddr string) (interface{}, error) {
	log.Printf("Received FactoryResetResponse from %s", remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}

	if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
		if h.taskRepo != nil {
			h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, nil, "")
		}
		return h.getNextTask(context.Background(), session)
	}

	return nil, nil
}

func (h *Handler) handleDownloadResponse(resp *DownloadResponse, remoteAddr string) (interface{}, error) {
	if resp == nil {
		return nil, nil
	}
	log.Printf("Received DownloadResponse: Status=%d from %s", resp.Status, remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}

	if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
		if resp.Status == 0 {
			if h.taskRepo != nil {
				h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, nil, "")
			}
			return h.getNextTask(context.Background(), session)
		}
		log.Printf("Download started, waiting for TransferComplete")
		return nil, nil
	}
	return nil, nil
}

func (h *Handler) handleTransferComplete(tc *TransferComplete, remoteAddr string) (interface{}, error) {
	if tc == nil {
		return nil, nil
	}
	log.Printf("Received TransferComplete: CommandKey=%s, FaultCode=%d from %s", tc.CommandKey, tc.FaultStruct.FaultCode, remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return &TransferCompleteResponse{}, nil
	}

	if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
		if h.taskRepo != nil {
			if tc.FaultStruct.FaultCode == 0 {
				h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, nil, "")
			} else {
				h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusFailed, nil, tc.FaultStruct.FaultString)
			}
		}
		return &TransferCompleteResponse{}, nil
	}

	return &TransferCompleteResponse{}, nil
}

func (h *Handler) handleFault(fault *SOAPFault, remoteAddr string) (interface{}, error) {
	log.Printf("Received SOAP Fault: %s - %s from %s", fault.FaultCode, fault.FaultString, remoteAddr)

	faultCode := fault.FaultCode
	faultMessage := fault.FaultString
	if fault.Detail.CWMPFault != nil {
		log.Printf("  CWMP Fault: %s - %s",
			fault.Detail.CWMPFault.FaultCode,
			fault.Detail.CWMPFault.FaultString,
		)
		faultCode = fault.Detail.CWMPFault.FaultCode
		faultMessage = fault.Detail.CWMPFault.FaultString
	}

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}

	if session.State == StateProcessingTasks {
		if h.faultRepo != nil && session.DeviceID > 0 && session.AutoFetchPhase == 0 {
			deviceFault := &models.Fault{
				DeviceID:    session.DeviceID,
				FaultCode:   faultCode,
				FaultString: faultMessage,
			}
			if err := h.faultRepo.Create(context.Background(), deviceFault); err != nil {
				log.Printf("Error saving fault: %v", err)
			}
		}

		if session.AutoFetchPhase > 0 {
			log.Printf("Auto-fetch phase %d failed, continuing to next phase", session.AutoFetchPhase)
			return h.continueAutoFetch(session)
		}

		if session.CurrentTaskID > 0 && h.taskRepo != nil {
			h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusFailed, nil, faultMessage)
		}
		return h.getNextTask(context.Background(), session)
	}

	return nil, nil
}

func (h *Handler) getNextTask(ctx context.Context, session *Session) (interface{}, error) {
	if h.taskRepo == nil || session.DeviceID == 0 {
		session.State = StateIdle
		return nil, nil
	}

	tasks, err := h.taskRepo.GetPendingByDeviceID(ctx, session.DeviceID)
	if err != nil || len(tasks) == 0 {
		session.State = StateIdle
		session.CurrentTaskID = 0
		return nil, nil
	}

	task := tasks[0]
	session.CurrentTaskID = task.ID
	h.taskRepo.UpdateStatus(context.Background(), task.ID, models.TaskStatusSent, nil, "")

	return h.buildTaskRequest(task)
}

func (h *Handler) buildTaskRequest(task *models.Task) (interface{}, error) {
	switch task.Type {
	case models.TaskTypeGetParameterValues:
		var params []string
		payloadBytes, _ := json.Marshal(task.Payload)
		json.Unmarshal(payloadBytes, &params)
		if len(params) > 0 {
			log.Printf("Sending GetParameterValues: %v", params)
			return &GetParameterValues{ParameterNames: params}, nil
		}

	case models.TaskTypeSetParameterValues:
		var paramMap map[string]string
		payloadBytes, _ := json.Marshal(task.Payload)
		json.Unmarshal(payloadBytes, &paramMap)
		if len(paramMap) > 0 {
			spv := &SetParameterValues{ParameterKey: "miniacs"}
			for name, value := range paramMap {
				spv.ParameterList.Parameters = append(spv.ParameterList.Parameters, ParameterValueStruct{
					Name:  name,
					Value: value,
				})
			}
			log.Printf("Sending SetParameterValues: %d params", len(spv.ParameterList.Parameters))
			return spv, nil
		}

	case models.TaskTypeReboot:
		log.Printf("Sending Reboot command")
		return &Reboot{CommandKey: "miniacs-reboot"}, nil

	case models.TaskTypeFactoryReset:
		log.Printf("Sending FactoryReset command")
		return &FactoryReset{}, nil

	case models.TaskTypeDownload:
		var payload map[string]interface{}
		payloadBytes, _ := json.Marshal(task.Payload)
		json.Unmarshal(payloadBytes, &payload)

		fileType, _ := payload["file_type"].(string)
		url, _ := payload["url"].(string)
		fileSize, _ := payload["file_size"].(float64)
		targetFilename, _ := payload["target_filename"].(string)

		if url != "" {
			log.Printf("Sending Download: %s", url)
			return &Download{
				CommandKey:     fmt.Sprintf("miniacs-dl-%d", task.ID),
				FileType:       fileType,
				URL:            url,
				FileSize:       int64(fileSize),
				TargetFileName: targetFilename,
			}, nil
		}
	}

	return nil, nil
}

func (h *Handler) sendFault(w http.ResponseWriter, err error) {
	fault := &SOAPFault{
		FaultCode:   "Server",
		FaultString: err.Error(),
	}

	response, _ := GenerateSOAPEnvelope(fault)
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(response)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func newSessionKey() string {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value)
}

func secureEqual(left, right string) bool {
	leftHash := sha256.Sum256([]byte(left))
	rightHash := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftHash[:], rightHash[:]) == 1
}

func parseAllowedNetworks(value string) []*net.IPNet {
	var networks []*net.IPNet
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		if _, network, err := net.ParseCIDR(item); err == nil {
			networks = append(networks, network)
		} else {
			log.Printf("Ignoring invalid CWMP_ALLOWED_CIDRS entry %q", item)
		}
	}
	return networks
}

func (h *Handler) isNetworkAllowed(remoteAddress string) bool {
	if len(h.allowedNetworks) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range h.allowedNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
