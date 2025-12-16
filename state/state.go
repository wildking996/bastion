package state

import (
	"bastion/core"
	"sync"
)

// AppState holds application state
type AppState struct {
	Sessions map[string]core.Session
	sync.RWMutex
}

// Global is the shared application state instance
var Global = &AppState{
	Sessions: make(map[string]core.Session),
}

// RemoveAndStopSession safely removes and stops a session to avoid deadlocks
func (s *AppState) RemoveAndStopSession(id string) bool {
	s.Lock()
	session, exists := s.Sessions[id]
	if exists {
		delete(s.Sessions, id)
	}
	s.Unlock()

	// Stop session outside lock to avoid deadlocks
	if exists {
		session.Stop()
		return true
	}
	return false
}

// GetSession safely fetches a session
func (s *AppState) GetSession(id string) (core.Session, bool) {
	s.RLock()
	defer s.RUnlock()
	session, exists := s.Sessions[id]
	return session, exists
}

// AddSession safely adds a session
func (s *AppState) AddSession(id string, session core.Session) {
	s.Lock()
	defer s.Unlock()
	s.Sessions[id] = session
}

// SessionExists checks whether a session exists
func (s *AppState) SessionExists(id string) bool {
	s.RLock()
	defer s.RUnlock()
	_, exists := s.Sessions[id]
	return exists
}
