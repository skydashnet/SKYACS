package cwmp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/skydashnet/skyacs/internal/database"
	"github.com/skydashnet/skyacs/internal/models"
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
	trustedProxies   []*net.IPNet
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
		sessions:         NewSessionManager(2 * time.Minute),
		deviceRepo:       deviceRepo,
		taskRepo:         taskRepo,
		parameterRepo:    parameterRepo,
		provisioningRepo: provisioningRepo,
		faultRepo:        faultRepo,
		blockedRepo:      blockedRepo,
		cwmpUsername:     os.Getenv("CWMP_USERNAME"),
		cwmpPassword:     os.Getenv("CWMP_PASSWORD"),
		allowedNetworks:  parseAllowedNetworks(os.Getenv("CWMP_ALLOWED_CIDRS")),
		trustedProxies:   parseAllowedNetworks(os.Getenv("CWMP_TRUSTED_PROXY_CIDRS")),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.isNetworkAllowed(h.clientAddress(r)) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if h.cwmpUsername != "" || h.cwmpPassword != "" {
		if !h.isSecureRequest(r) {
			http.Error(w, "TLS required", http.StatusUpgradeRequired)
			return
		}
		username, password, ok := r.BasicAuth()
		if !ok || !secureEqual(username, h.cwmpUsername) || !secureEqual(password, h.cwmpPassword) {
			w.Header().Set("WWW-Authenticate", `Basic realm="SKYACS CWMP"`)
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
			Secure:   h.isSecureRequest(r),
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
		response, namespace, err := h.handleEmptyPost(r.Context(), sessionKey)
		if err != nil {
			log.Printf("Error dispatching CWMP request: %v", err)
			h.sendFault(w, err, nil)
			return
		}
		if response == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		responseBytes, err := GenerateSOAPEnvelopeWithContext(response, namespace, newMessageID())
		if err != nil {
			h.failCurrentTask(r.Context(), sessionKey, err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		if _, err := w.Write(responseBytes); err != nil {
			log.Printf("Error writing CWMP request: %v", err)
		}
		return
	}

	response, err := h.routeMessage(r.Context(), envelope, sessionKey)
	if err != nil {
		log.Printf("Error processing message: %v", err)
		h.sendFault(w, err, envelope)
		return
	}

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	if response == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	responseBytes, err := GenerateSOAPEnvelopeForRequest(response, envelope)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(responseBytes); err != nil {
		log.Printf("Error writing CWMP response: %v", err)
	}
}

func (h *Handler) routeMessage(ctx context.Context, envelope *SOAPEnvelope, remoteAddr string) (interface{}, error) {
	msgType := DetectMessageType(&envelope.Body)
	log.Printf("Received CWMP message: %s from %s", msgType, remoteAddr)

	switch msgType {
	case "Inform":
		return h.handleInform(ctx, envelope, remoteAddr)

	case "GetParameterValuesResponse":
		return h.handleGetParameterValuesResponse(ctx, envelope.Body.GetParameterValuesResp, remoteAddr)

	case "GetParameterNamesResponse":
		return h.handleGetParameterNamesResponse(ctx, envelope.Body.GetParameterNamesResp, remoteAddr)

	case "SetParameterValuesResponse":
		return h.handleSetParameterValuesResponse(ctx, envelope.Body.SetParameterValuesResp, remoteAddr)

	case "RebootResponse":
		return h.handleRebootResponse(ctx, remoteAddr)

	case "FactoryResetResponse":
		return h.handleFactoryResetResponse(ctx, remoteAddr)

	case "DownloadResponse":
		return h.handleDownloadResponse(ctx, envelope.Body.DownloadResponse, remoteAddr)

	case "TransferComplete":
		return h.handleTransferComplete(ctx, envelope.Body.TransferComplete, remoteAddr)

	case "Fault":
		return h.handleFault(ctx, envelope.Body.Fault, remoteAddr)

	default:
		return &SOAPFault{FaultCode: "Client", FaultString: "CWMP fault", Detail: FaultDetail{CWMPFault: &CWMPFault{FaultCode: "8000", FaultString: "Method not supported"}}}, nil
	}
}

func (h *Handler) handleInform(ctx context.Context, envelope *SOAPEnvelope, remoteAddr string) (interface{}, error) {
	inform := envelope.Body.Inform
	if inform == nil || inform.DeviceId.SerialNumber == "" {
		return nil, fmt.Errorf("device serial number is required")
	}
	if h.blockedRepo != nil {
		blocked, err := h.blockedRepo.IsBlocked(ctx, inform.DeviceId.SerialNumber)
		if err != nil {
			return nil, fmt.Errorf("check device admission: %w", err)
		}
		if blocked {
			log.Printf("Blocked device rejected: %s", inform.DeviceId.SerialNumber)
			h.sessions.Remove(remoteAddr)
			if h.deviceRepo != nil {
				if err := h.deviceRepo.SetOffline(ctx, inform.DeviceId.SerialNumber); err != nil {
					log.Printf("Failed to mark blocked device offline: %v", err)
				}
			}
			return &InformResponse{MaxEnvelopes: 1}, nil
		}
	}

	response, err := ProcessInform(inform)
	if err != nil {
		return nil, err
	}

	var deviceID int64

	if h.deviceRepo != nil {
		params := ExtractImportantParameters(inform.ParameterList.Parameters)

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

		if err := h.deviceRepo.UpsertFromInform(ctx, device); err != nil {
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
			if err := h.parameterRepo.UpsertMany(ctx, deviceID, paramsToSave); err != nil {
				log.Printf("Error saving Inform parameters: %v", err)
			}
		}
	}

	hasBootstrap := hasEvent(inform, EventBootstrap)
	var provisioning *SetParameterValues
	var provisioningRules []models.ProvisioningApplication
	if hasBootstrap && deviceID > 0 && h.provisioningRepo != nil {
		rules, err := h.provisioningRepo.ListPendingForDevice(ctx, deviceID, inform.DeviceId.Manufacturer, inform.DeviceId.ProductClass)
		if err == nil && len(rules) > 0 {
			log.Printf("Applying %d provisioning rules to device %s", len(rules), inform.DeviceId.SerialNumber)

			spv := &SetParameterValues{ParameterKey: "auto-provisioning"}
			for _, rule := range rules {
				provisioningRules = append(provisioningRules, models.ProvisioningApplication{RuleID: rule.ID, RuleVersion: rule.Version})
				spv.ParameterList.Parameters = append(spv.ParameterList.Parameters, ParameterValueStruct{
					Name:  rule.ParameterName,
					Value: rule.ParameterValue,
					Type:  rule.ParameterType,
				})
			}

			if len(spv.ParameterList.Parameters) > 0 {
				provisioning = spv
			}
		}
	}

	session := h.sessions.GetOrCreate(remoteAddr, inform.DeviceId.SerialNumber)
	session.mu.Lock()
	session.State = StateInformReceived
	session.DeviceID = deviceID
	session.DataModelRoot = DetectDataModelRoot(inform.ParameterList.Parameters)
	session.CWMPNamespace = envelope.CWMPNamespace
	session.Provisioning = provisioning
	session.ProvisioningRules = provisioningRules
	// Full-tree discovery is intentionally limited to BOOTSTRAP. PERIODIC
	// informs already carry telemetry and must stay cheap for large fleets.
	session.AutoFetchReady = hasBootstrap
	session.mu.Unlock()

	return response, nil
}

func hasEvent(inform *Inform, code string) bool {
	for _, event := range inform.Event.Events {
		if event.EventCode == code {
			return true
		}
	}
	return false
}

func (h *Handler) handleEmptyPost(ctx context.Context, sessionID string) (interface{}, string, error) {
	session := h.sessions.Get(sessionID)
	if session == nil {
		return nil, CWMPNamespace10, nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()

	namespace := defaultNamespace(session.CWMPNamespace)
	request, err := h.getNextTask(ctx, session)
	if err != nil || request != nil {
		return request, namespace, err
	}
	if session.Provisioning != nil {
		request = session.Provisioning
		session.Provisioning = nil
		session.State = StateProcessingTasks
		return request, namespace, nil
	}
	if session.AutoFetchReady {
		session.AutoFetchReady = false
		session.AutoFetchPhase = 1
		session.State = StateProcessingTasks
		return &GetParameterValues{ParameterNames: []string{session.DataModelRoot}}, namespace, nil
	}
	session.State = StateIdle
	return nil, namespace, nil
}

func (h *Handler) handleGetParameterValuesResponse(ctx context.Context, resp *GetParameterValuesResp, remoteAddr string) (interface{}, error) {
	log.Printf("Received GetParameterValuesResponse with %d parameters from %s", len(resp.ParameterList.Parameters), remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		log.Printf("No active session found for %s", remoteAddr)
		return nil, nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()

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
		if err := h.parameterRepo.UpsertMany(ctx, session.DeviceID, params); err != nil {
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
			if database.IsSensitiveParameterName(p.Name) {
				result[p.Name] = "[REDACTED]"
			} else {
				result[p.Name] = p.Value
			}
		}
		if err := h.taskRepo.UpdateStatus(ctx, session.CurrentTaskID, models.TaskStatusCompleted, result, ""); err != nil {
			return nil, fmt.Errorf("complete parameter task: %w", err)
		}
		session.CurrentTaskID = 0
	}

	return h.getNextTask(ctx, session)
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

func (h *Handler) handleGetParameterNamesResponse(ctx context.Context, resp *GetParameterNamesResp, remoteAddr string) (interface{}, error) {
	if resp == nil {
		return nil, nil
	}
	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if h.parameterRepo != nil && session.DeviceID > 0 {
		params := make([]models.DeviceParameter, 0, len(resp.ParameterList))
		for _, parameter := range resp.ParameterList {
			writable := parameter.Writable
			params = append(params, models.DeviceParameter{Name: parameter.Name, Writable: &writable})
		}
		if err := h.parameterRepo.UpsertWritable(ctx, session.DeviceID, params); err != nil {
			log.Printf("Error saving writable parameter flags: %v", err)
		}
	}
	session.AutoFetchPhase = 0
	session.State = StateIdle
	return h.getNextTask(ctx, session)
}

func (h *Handler) handleSetParameterValuesResponse(ctx context.Context, resp *SetParameterValuesResp, remoteAddr string) (interface{}, error) {
	if resp == nil {
		return nil, errors.New("missing SetParameterValuesResponse")
	}
	log.Printf("SetParameterValues status: %d from %s", resp.Status, remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
		if h.taskRepo != nil {
			if err := h.taskRepo.UpdateStatus(ctx, session.CurrentTaskID, models.TaskStatusCompleted, map[string]int{"status": resp.Status}, ""); err != nil {
				return nil, fmt.Errorf("complete set-parameter task: %w", err)
			}
		}
		session.CurrentTaskID = 0
		return h.getNextTask(ctx, session)
	}
	if session.State == StateProcessingTasks && len(session.ProvisioningRules) > 0 {
		if h.provisioningRepo != nil {
			if err := h.provisioningRepo.MarkApplied(ctx, session.DeviceID, session.ProvisioningRules); err != nil {
				return nil, fmt.Errorf("record provisioning application: %w", err)
			}
		}
		session.ProvisioningRules = nil
		return h.getNextTask(ctx, session)
	}

	return nil, nil
}

func (h *Handler) handleRebootResponse(ctx context.Context, remoteAddr string) (interface{}, error) {
	log.Printf("Received RebootResponse from %s", remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
		if h.taskRepo != nil {
			if err := h.taskRepo.UpdateStatus(ctx, session.CurrentTaskID, models.TaskStatusCompleted, nil, ""); err != nil {
				return nil, fmt.Errorf("complete reboot task: %w", err)
			}
		}
		session.CurrentTaskID = 0
		return h.getNextTask(ctx, session)
	}

	return nil, nil
}

func (h *Handler) handleFactoryResetResponse(ctx context.Context, remoteAddr string) (interface{}, error) {
	log.Printf("Received FactoryResetResponse from %s", remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
		if h.taskRepo != nil {
			if err := h.taskRepo.UpdateStatus(ctx, session.CurrentTaskID, models.TaskStatusCompleted, nil, ""); err != nil {
				return nil, fmt.Errorf("complete factory-reset task: %w", err)
			}
		}
		session.CurrentTaskID = 0
		return h.getNextTask(ctx, session)
	}

	return nil, nil
}

func (h *Handler) handleDownloadResponse(ctx context.Context, resp *DownloadResponse, remoteAddr string) (interface{}, error) {
	if resp == nil {
		return nil, nil
	}
	log.Printf("Received DownloadResponse: Status=%d from %s", resp.Status, remoteAddr)

	session := h.sessions.Get(remoteAddr)
	if session == nil {
		return nil, nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
		if resp.Status == 0 {
			if h.taskRepo != nil {
				if err := h.taskRepo.UpdateStatus(ctx, session.CurrentTaskID, models.TaskStatusCompleted, nil, ""); err != nil {
					return nil, fmt.Errorf("complete download task: %w", err)
				}
			}
			session.CurrentTaskID = 0
			return h.getNextTask(ctx, session)
		}
		log.Printf("Download started, waiting for TransferComplete")
		return nil, nil
	}
	return nil, nil
}

func (h *Handler) handleTransferComplete(ctx context.Context, tc *TransferComplete, remoteAddr string) (interface{}, error) {
	if tc == nil {
		return nil, nil
	}
	log.Printf("Received TransferComplete: CommandKey=%s, FaultCode=%d from %s", tc.CommandKey, tc.FaultStruct.FaultCode, remoteAddr)

	if h.taskRepo != nil && tc.CommandKey != "" {
		status := models.TaskStatusCompleted
		errorMessage := ""
		if tc.FaultStruct.FaultCode != 0 {
			status = models.TaskStatusFailed
			errorMessage = tc.FaultStruct.FaultString
		}
		result := map[string]interface{}{
			"fault_code":    tc.FaultStruct.FaultCode,
			"start_time":    tc.StartTime,
			"complete_time": tc.CompleteTime,
		}
		if err := h.taskRepo.UpdateByCommandKey(ctx, tc.CommandKey, status, result, errorMessage); err != nil && err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("complete transfer task: %w", err)
		}
	}

	return &TransferCompleteResponse{}, nil
}

func (h *Handler) handleFault(ctx context.Context, fault *SOAPFault, remoteAddr string) (interface{}, error) {
	if fault == nil {
		return nil, errors.New("missing SOAP fault")
	}
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
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.State == StateProcessingTasks {
		if h.faultRepo != nil && session.DeviceID > 0 && session.AutoFetchPhase == 0 {
			deviceFault := &models.Fault{
				DeviceID:    session.DeviceID,
				FaultCode:   faultCode,
				FaultString: faultMessage,
			}
			if err := h.faultRepo.Create(ctx, deviceFault); err != nil {
				log.Printf("Error saving fault: %v", err)
			}
		}

		if session.AutoFetchPhase > 0 {
			log.Printf("Auto-fetch phase %d failed, continuing to next phase", session.AutoFetchPhase)
			return h.continueAutoFetch(session)
		}

		if session.CurrentTaskID > 0 && h.taskRepo != nil {
			if err := h.taskRepo.UpdateStatus(ctx, session.CurrentTaskID, models.TaskStatusFailed, nil, faultMessage); err != nil {
				return nil, fmt.Errorf("fail task after CWMP fault: %w", err)
			}
			session.CurrentTaskID = 0
		}
		return h.getNextTask(ctx, session)
	}

	return nil, nil
}

func (h *Handler) getNextTask(ctx context.Context, session *Session) (interface{}, error) {
	if h.taskRepo == nil || session.DeviceID == 0 {
		session.State = StateIdle
		return nil, nil
	}
	if h.blockedRepo != nil {
		blocked, err := h.blockedRepo.IsBlocked(ctx, session.SerialNumber)
		if err != nil {
			return nil, fmt.Errorf("check device admission before task dispatch: %w", err)
		}
		if blocked {
			session.State = StateIdle
			session.CurrentTaskID = 0
			return nil, nil
		}
	}

	for {
		task, err := h.taskRepo.ClaimNextPending(ctx, session.DeviceID)
		if err != nil {
			return nil, fmt.Errorf("claim next task: %w", err)
		}
		if task == nil {
			session.State = StateIdle
			session.CurrentTaskID = 0
			return nil, nil
		}

		session.CurrentTaskID = task.ID
		session.State = StateProcessingTasks
		request, err := h.buildTaskRequest(task)
		if err == nil {
			return request, nil
		}
		if updateErr := h.taskRepo.UpdateStatus(ctx, task.ID, models.TaskStatusFailed, nil, err.Error()); updateErr != nil {
			return nil, fmt.Errorf("invalid task payload: %v; persist failure: %w", err, updateErr)
		}
		session.CurrentTaskID = 0
	}
}

func (h *Handler) buildTaskRequest(task *models.Task) (interface{}, error) {
	switch task.Type {
	case models.TaskTypeGetParameterValues:
		var params []string
		if err := json.Unmarshal(task.Payload, &params); err != nil {
			return nil, fmt.Errorf("decode get-parameter payload: %w", err)
		}
		if len(params) > 0 {
			log.Printf("Sending GetParameterValues: %v", params)
			return &GetParameterValues{ParameterNames: params}, nil
		}

	case models.TaskTypeSetParameterValues:
		var paramMap map[string]string
		if err := json.Unmarshal(task.Payload, &paramMap); err != nil {
			return nil, fmt.Errorf("decode set-parameter payload: %w", err)
		}
		if len(paramMap) > 0 {
			spv := &SetParameterValues{ParameterKey: task.CommandKey}
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
		return &Reboot{CommandKey: task.CommandKey}, nil

	case models.TaskTypeFactoryReset:
		log.Printf("Sending FactoryReset command")
		return &FactoryReset{}, nil

	case models.TaskTypeDownload:
		var payload map[string]interface{}
		if err := json.Unmarshal(task.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode download payload: %w", err)
		}

		fileType, _ := payload["file_type"].(string)
		url, _ := payload["url"].(string)
		fileSize, _ := payload["file_size"].(float64)
		targetFilename, _ := payload["target_filename"].(string)

		if url != "" {
			log.Printf("Sending Download: %s", url)
			return &Download{
				CommandKey:     task.CommandKey,
				FileType:       fileType,
				URL:            url,
				FileSize:       int64(fileSize),
				TargetFileName: targetFilename,
			}, nil
		}
	}

	return nil, fmt.Errorf("unsupported or empty task payload for %s", task.Type)
}

func (h *Handler) sendFault(w http.ResponseWriter, err error, request *SOAPEnvelope) {
	fault := &SOAPFault{
		FaultCode:   "Server",
		FaultString: err.Error(),
	}

	response, marshalErr := GenerateSOAPEnvelopeForRequest(fault, request)
	if marshalErr != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(response)
}

func (h *Handler) failCurrentTask(ctx context.Context, sessionID string, failure error) {
	session := h.sessions.Get(sessionID)
	if session == nil || h.taskRepo == nil {
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.CurrentTaskID == 0 {
		return
	}
	if err := h.taskRepo.UpdateStatus(ctx, session.CurrentTaskID, models.TaskStatusFailed, nil, failure.Error()); err != nil {
		log.Printf("Error failing undispatched task: %v", err)
	}
	session.CurrentTaskID = 0
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

func newMessageID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("skyacs-%d", time.Now().UnixNano())
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

func (h *Handler) isSecureRequest(request *http.Request) bool {
	if request.TLS != nil {
		return true
	}
	if !networkContains(h.trustedProxies, request.RemoteAddr) {
		return false
	}
	return strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https")
}

func (h *Handler) clientAddress(request *http.Request) string {
	if !networkContains(h.trustedProxies, request.RemoteAddr) {
		return request.RemoteAddr
	}
	forwarded := strings.Split(request.Header.Get("X-Forwarded-For"), ",")
	for index := len(forwarded) - 1; index >= 0; index-- {
		candidate := strings.TrimSpace(forwarded[index])
		if net.ParseIP(candidate) == nil {
			continue
		}
		if !networkContains(h.trustedProxies, candidate) {
			return candidate
		}
	}
	return request.RemoteAddr
}

func networkContains(networks []*net.IPNet, remoteAddress string) bool {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
