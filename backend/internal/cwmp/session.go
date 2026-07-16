package cwmp

import (
	"sync"
	"time"
)

// SessionState track current state of a CWMP session
type SessionState int

const (
	StateWaitingInform SessionState = iota
	StateInformReceived
	StateProcessingTasks
	StateIdle
	StateEnding
)

// Session represents single CPE session
type Session struct {
	ID             string
	SerialNumber   string
	DeviceID       int64
	CurrentTaskID  int64
	AutoFetchPhase int
	DataModelRoot  string

	State        SessionState
	CreatedAt    time.Time
	LastActivity time.Time

	PendingTasks []Task
	TaskIndex    int
}

// Task represents pending action untuk CPE
type Task struct {
	Type    string // "GetParameterValues", "SetParameterValues", etc
	Payload interface{}
}

// SessionManager manage active CWMP sessions
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	timeout  time.Duration
}

func NewSessionManager(timeout time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		timeout:  timeout,
	}

	// Start cleanup goroutine
	go sm.cleanupLoop()

	return sm
}

// GetOrCreate get existing session atau create new
func (sm *SessionManager) GetOrCreate(sessionID, serialNumber string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.sessions[sessionID]; ok {
		s.LastActivity = time.Now()
		return s
	}

	s := &Session{
		ID:           sessionID,
		SerialNumber: serialNumber,
		State:        StateWaitingInform,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		PendingTasks: []Task{},
	}
	sm.sessions[sessionID] = s
	return s
}

// Get session by ID
func (sm *SessionManager) Get(sessionID string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[sessionID]
}

// Remove session
func (sm *SessionManager) Remove(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}

// AddTask add pending task untuk device
func (sm *SessionManager) AddTask(serialNumber string, task Task) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Find session by serial number dan add task
	for _, s := range sm.sessions {
		if s.SerialNumber == serialNumber {
			s.PendingTasks = append(s.PendingTasks, task)
			return
		}
	}

	// The task remains queued and will be picked up by the next device session.
}

// GetNextTask get next pending task untuk session
func (s *Session) GetNextTask() *Task {
	if s.TaskIndex >= len(s.PendingTasks) {
		return nil
	}
	task := &s.PendingTasks[s.TaskIndex]
	s.TaskIndex++
	return task
}

// cleanupLoop remove expired sessions
func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.cleanup()
	}
}

func (sm *SessionManager) cleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, s := range sm.sessions {
		if now.Sub(s.LastActivity) > sm.timeout {
			delete(sm.sessions, id)
		}
	}
}
