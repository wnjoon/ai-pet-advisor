package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
)

// SessionManager manages chat sessions in-memory with timeout detection.
type SessionManager struct {
	sessions   sync.Map // key: "userID:platform" → *domain.ChatSession
	timeoutMin int
	reconciler *Reconciler
	memoryMgr  *MemoryManager
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(timeoutMin int, reconciler *Reconciler, memoryMgr *MemoryManager) *SessionManager {
	return &SessionManager{
		timeoutMin: timeoutMin,
		reconciler: reconciler,
		memoryMgr:  memoryMgr,
	}
}

// sessionKey generates the map key for a user+platform combination.
func sessionKey(userID, platform string) string {
	return userID + ":" + platform
}

// CreateSession creates a new chat session.
func (m *SessionManager) CreateSession(userID, dogID, platform string) *domain.ChatSession {
	now := time.Now()
	sess := &domain.ChatSession{
		SessionID:    uuid.NewString(),
		UserID:       userID,
		DogID:        dogID,
		Platform:     platform,
		StartedAt:    now,
		LastActiveAt: now,
		IsActive:     true,
	}

	key := sessionKey(userID, platform)

	// End previous session if exists
	if prev, ok := m.sessions.Load(key); ok {
		prevSess := prev.(*domain.ChatSession)
		if prevSess.IsActive {
			m.endSession(prevSess)
		}
	}

	m.sessions.Store(key, sess)
	return sess
}

// GetActiveSession retrieves the active session for a user+platform.
// If the session has timed out, it is automatically ended and nil is returned.
func (m *SessionManager) GetActiveSession(userID, platform string) *domain.ChatSession {
	key := sessionKey(userID, platform)
	val, ok := m.sessions.Load(key)
	if !ok {
		return nil
	}

	sess := val.(*domain.ChatSession)
	if !sess.IsActive {
		return nil
	}

	// Lazy timeout check
	if m.isTimedOut(sess) {
		m.endSession(sess)
		return nil
	}

	return sess
}

// GetOrCreateSession retrieves the active session or creates a new one.
func (m *SessionManager) GetOrCreateSession(userID, dogID, platform string) *domain.ChatSession {
	sess := m.GetActiveSession(userID, platform)
	if sess != nil {
		return sess
	}
	return m.CreateSession(userID, dogID, platform)
}

// UpdateLastActive refreshes the session's last activity timestamp.
func (m *SessionManager) UpdateLastActive(sessionID string) {
	m.sessions.Range(func(key, value any) bool {
		sess := value.(*domain.ChatSession)
		if sess.SessionID == sessionID {
			sess.LastActiveAt = time.Now()
			return false // stop iteration
		}
		return true
	})
}

// EndSession explicitly ends a session and triggers L2 reconciliation.
func (m *SessionManager) EndSession(sessionID string) {
	m.sessions.Range(func(key, value any) bool {
		sess := value.(*domain.ChatSession)
		if sess.SessionID == sessionID {
			m.endSession(sess)
			return false
		}
		return true
	})
}

// SwitchDog changes the active dog for a session.
func (m *SessionManager) SwitchDog(sessionID, newDogID string) *domain.ChatSession {
	var result *domain.ChatSession
	m.sessions.Range(func(key, value any) bool {
		sess := value.(*domain.ChatSession)
		if sess.SessionID == sessionID && sess.IsActive {
			sess.DogID = newDogID
			sess.LastActiveAt = time.Now()
			result = sess
			return false
		}
		return true
	})
	return result
}

// CleanupExpired scans all sessions and ends any that have timed out.
// Can be called periodically from a background goroutine.
func (m *SessionManager) CleanupExpired() {
	m.sessions.Range(func(key, value any) bool {
		sess := value.(*domain.ChatSession)
		if sess.IsActive && m.isTimedOut(sess) {
			m.endSession(sess)
		}
		return true
	})
}

// isTimedOut checks if a session has exceeded the timeout.
func (m *SessionManager) isTimedOut(sess *domain.ChatSession) bool {
	return time.Since(sess.LastActiveAt) > time.Duration(m.timeoutMin)*time.Minute
}

// endSession marks a session as inactive and triggers L2 reconciliation.
func (m *SessionManager) endSession(sess *domain.ChatSession) {
	sess.IsActive = false

	// Trigger L2 reconciliation with all L1 recent items for this dog
	if m.reconciler != nil && m.memoryMgr != nil {
		contexts, err := m.memoryMgr.GetAllRecentContexts(sess.DogID)
		if err == nil {
			var allSnippets domain.ChatSnippets
			for _, ctx := range contexts {
				allSnippets = append(allSnippets, ctx.RecentItems...)
			}
			if len(allSnippets) > 0 {
				_ = m.reconciler.ReconcileOnSessionEnd(sess.DogID, allSnippets)
			}
		}
	}
}
