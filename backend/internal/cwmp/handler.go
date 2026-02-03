package cwmp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	
	"time"

	"github.com/skydashnet/miniacs/internal/database"
	"gorm.io/gorm"
	"github.com/skydashnet/miniacs/internal/models"
)

type Handler struct {
	sessions         *SessionManager
	deviceRepo       *database.DeviceRepository
	taskRepo         *database.TaskRepository
	parameterRepo    *database.ParameterRepository
	provisioningRepo *database.ProvisioningRepository
	faultRepo        *database.FaultRepository
}

func NewHandler(db *gorm.DB) *Handler {
	var deviceRepo *database.DeviceRepository
	var taskRepo *database.TaskRepository
	var parameterRepo *database.ParameterRepository
	var provisioningRepo *database.ProvisioningRepository
	var faultRepo *database.FaultRepository

	if db != nil {
		deviceRepo = database.NewDeviceRepository(db)
		taskRepo = database.NewTaskRepository(db)
		parameterRepo = database.NewParameterRepository(db)
		provisioningRepo = database.NewProvisioningRepository(db)
		faultRepo = database.NewFaultRepository(db)
	}

	return &Handler{
		sessions:         NewSessionManager(30 * time.Second),
		deviceRepo:       deviceRepo,
		taskRepo:         taskRepo,
		parameterRepo:    parameterRepo,
		provisioningRepo: provisioningRepo,
		faultRepo:        faultRepo,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	response, err := h.routeMessage(envelope)
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

func (h *Handler) routeMessage(envelope *SOAPEnvelope) (interface{}, error) {
	msgType := DeteksiTipeMessage(&envelope.Body)
	log.Printf("Received CWMP message: %s", msgType)

	switch msgType {
	case "Inform":
		return h.handleInform(envelope.Body.Inform)

	case "GetParameterValuesResponse":
		return h.handleGetParameterValuesResponse(envelope.Body.GetParameterValuesResp)

	case "SetParameterValuesResponse":
		return h.handleSetParameterValuesResponse(envelope.Body.SetParameterValuesResp)

	case "RebootResponse":
		return h.handleRebootResponse()

	case "FactoryResetResponse":
		return h.handleFactoryResetResponse()

	case "DownloadResponse":
		return h.handleDownloadResponse(envelope.Body.DownloadResponse)

	case "TransferComplete":
		return h.handleTransferComplete(envelope.Body.TransferComplete)

	case "Fault":
		return h.handleFault(envelope.Body.Fault)

	default:
		log.Printf("Unknown message type: %s", msgType)
		return nil, nil
	}
}

func (h *Handler) handleInform(inform *Inform) (interface{}, error) {
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
	}

	// Store session with device ID
	sessionID := inform.DeviceId.SerialNumber
	session := h.sessions.GetOrCreate(sessionID, inform.DeviceId.SerialNumber)
	session.State = StateInformReceived
	session.DeviceID = deviceID

	// Check for pending tasks in database
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

		// Auto-provisioning: Apply provisioning rules jika ada
		if h.provisioningRepo != nil {
			rules, err := h.provisioningRepo.ListEnabled(context.Background())
			if err == nil && len(rules) > 0 {
				log.Printf("Applying %d provisioning rules to device %s", len(rules), inform.DeviceId.SerialNumber)

				// Build SetParameterValues request
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

		// Auto-fetch parameters penting jika tidak ada pending tasks
		// Cek EventCode - auto-fetch pada BOOTSTRAP, PERIODIC, atau CONNECTION REQUEST
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

			// Phase 1: Get WiFi 2.4G info
			return &GetParameterValues{
				ParameterNames: []string{"InternetGatewayDevice.LANDevice.1.WLANConfiguration.1."},
			}, nil
		}
	}

	return response, nil
}

func (h *Handler) handleGetParameterValuesResponse(resp *GetParameterValuesResp) (interface{}, error) {
	log.Printf("Received GetParameterValuesResponse with %d parameters", len(resp.ParameterList.Parameters))

	for _, session := range h.sessions.sessions {
		if session.State == StateProcessingTasks {
			// Save parameters to database
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
					log.Printf("Saved %d parameters for device %d", len(params), session.DeviceID)
				}
			}

			// Handle auto-fetch phases
			if session.AutoFetchPhase > 0 {
				return h.continueAutoFetch(session)
			}

			// Handle manual task - mark as completed
			if session.CurrentTaskID > 0 && h.taskRepo != nil {
				result := make(map[string]string)
				for _, p := range resp.ParameterList.Parameters {
					result[p.Name] = p.Value
				}
				h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, result, "")
			}

			// Check for more pending tasks
			return h.getNextTask(context.Background(), session)
		}
	}

	return nil, nil
}

// continueAutoFetch lanjut ke phase berikutnya dari auto-fetch
func (h *Handler) continueAutoFetch(session *Session) (interface{}, error) {
	session.AutoFetchPhase++

	var paramPath string
	switch session.AutoFetchPhase {
	case 2:
		paramPath = "InternetGatewayDevice.LANDevice.1.WLANConfiguration.2." // WiFi 5G
	case 3:
		paramPath = "InternetGatewayDevice.WANDevice.1." // WAN
	case 4:
		paramPath = "InternetGatewayDevice.DeviceInfo." // Device Info
	case 5:
		paramPath = "InternetGatewayDevice.UserInterface.X_HW_WebUserInfo." // Modem Credentials (Huawei)
	case 6:
		paramPath = "InternetGatewayDevice.DeviceInfo.X_CMCC_TeleComAccount." // Modem Credentials (China Mobile)
	case 7:
		paramPath = "InternetGatewayDevice.DeviceInfo.X_CT-COM_TeleComAccount." // Modem Credentials (China Telecom)
	case 8:
		paramPath = "InternetGatewayDevice.DeviceInfo.X_ZTE_COM_TeleComAccount." // Modem Credentials (ZTE)
	case 9:
		paramPath = "InternetGatewayDevice.DeviceInfo.X_FH_Account." // Modem Credentials (FiberHome)
	case 10:
		paramPath = "Device.Users.User." // Modem Credentials (Generic TR-181)
	default:
		// Done - reset state
		session.AutoFetchPhase = 0
		session.State = StateIdle
		log.Printf("Auto-fetch completed for device %d", session.DeviceID)
		return nil, nil
	}

	log.Printf("Auto-fetch phase %d: %s", session.AutoFetchPhase, paramPath)
	return &GetParameterValues{
		ParameterNames: []string{paramPath},
	}, nil
}


func (h *Handler) handleSetParameterValuesResponse(resp *SetParameterValuesResp) (interface{}, error) {
	log.Printf("SetParameterValues status: %d", resp.Status)

	for _, session := range h.sessions.sessions {
		if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
			if h.taskRepo != nil {
				h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, map[string]int{"status": resp.Status}, "")
			}
			return h.getNextTask(context.Background(), session)
		}
	}

	return nil, nil
}

func (h *Handler) handleRebootResponse() (interface{}, error) {
	log.Printf("Received RebootResponse")

	for _, session := range h.sessions.sessions {
		if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
			if h.taskRepo != nil {
				h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, nil, "")
			}
			return h.getNextTask(context.Background(), session)
		}
	}

	return nil, nil
}

func (h *Handler) handleFactoryResetResponse() (interface{}, error) {
	log.Printf("Received FactoryResetResponse")

	for _, session := range h.sessions.sessions {
		if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
			if h.taskRepo != nil {
				h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, nil, "")
			}
			return h.getNextTask(context.Background(), session)
		}
	}

	return nil, nil
}

func (h *Handler) handleDownloadResponse(resp *DownloadResponse) (interface{}, error) {
	if resp == nil {
		return nil, nil
	}
	log.Printf("Received DownloadResponse: Status=%d", resp.Status)

	// Status 0 = completed immediately, 1 = download started (will get TransferComplete later)
	for _, session := range h.sessions.sessions {
		if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
			if resp.Status == 0 {
				// Completed immediately
				if h.taskRepo != nil {
					h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, nil, "")
				}
				return h.getNextTask(context.Background(), session)
			}
			// Status 1 = akan dapat TransferComplete nanti
			log.Printf("Download started, waiting for TransferComplete")
			return nil, nil
		}
	}
	return nil, nil
}

func (h *Handler) handleTransferComplete(tc *TransferComplete) (interface{}, error) {
	if tc == nil {
		return nil, nil
	}
	log.Printf("Received TransferComplete: CommandKey=%s, FaultCode=%d", tc.CommandKey, tc.FaultStruct.FaultCode)

	// Mark task as completed or failed based on FaultCode
	for _, session := range h.sessions.sessions {
		if session.State == StateProcessingTasks && session.CurrentTaskID > 0 {
			if h.taskRepo != nil {
				if tc.FaultStruct.FaultCode == 0 {
					h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusCompleted, nil, "")
				} else {
					h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusFailed, nil, tc.FaultStruct.FaultString)
				}
			}
			// Send TransferCompleteResponse
			return &TransferCompleteResponse{}, nil
		}
	}

	// Jika tidak ada session aktif, tetap send response
	return &TransferCompleteResponse{}, nil
}

func (h *Handler) handleFault(fault *SOAPFault) (interface{}, error) {
	log.Printf("Received SOAP Fault: %s - %s", fault.FaultCode, fault.FaultString)

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

	for _, session := range h.sessions.sessions {
		if session.State == StateProcessingTasks {
			// Simpan fault ke database (kecuali auto-fetch yang sering fault karena parameter tidak ada)
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

			// Jika sedang auto-fetch, skip ke phase berikutnya
			if session.AutoFetchPhase > 0 {
				log.Printf("Auto-fetch phase %d failed, continuing to next phase", session.AutoFetchPhase)
				return h.continueAutoFetch(session)
			}

			// Mark manual task as failed
			if session.CurrentTaskID > 0 && h.taskRepo != nil {
				h.taskRepo.UpdateStatus(context.Background(), session.CurrentTaskID, models.TaskStatusFailed, nil, faultMessage)
			}
			return h.getNextTask(context.Background(), session)
		}
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
