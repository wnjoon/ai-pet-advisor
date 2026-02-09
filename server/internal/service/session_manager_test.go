package service

import (
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	mgr := NewSessionManager(30, nil, nil)
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	sess := mgr.CreateSession(userID, dogID, platform)

	if sess == nil {
		t.Fatal("CreateSession returned nil")
	}
	if sess.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
	if sess.UserID != userID {
		t.Errorf("UserID = %s, want %s", sess.UserID, userID)
	}
	if sess.DogID != dogID {
		t.Errorf("DogID = %s, want %s", sess.DogID, dogID)
	}
	if sess.Platform != platform {
		t.Errorf("Platform = %s, want %s", sess.Platform, platform)
	}
	if sess.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
	if sess.LastActiveAt.IsZero() {
		t.Error("LastActiveAt should not be zero")
	}
	if !sess.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestCreateSession_ReplacesPrevious(t *testing.T) {
	mgr := NewSessionManager(30, nil, nil)
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	// Create first session
	sess1 := mgr.CreateSession(userID, dogID, platform)
	if !sess1.IsActive {
		t.Fatal("First session should be active")
	}

	// Create second session for same user+platform
	sess2 := mgr.CreateSession(userID, dogID, platform)
	if !sess2.IsActive {
		t.Fatal("Second session should be active")
	}

	// First session should be ended
	if sess1.IsActive {
		t.Error("First session should be inactive after creating second session")
	}

	// Verify second session is different
	if sess1.SessionID == sess2.SessionID {
		t.Error("Second session should have different SessionID")
	}
}

func TestGetActiveSession_Found(t *testing.T) {
	mgr := NewSessionManager(30, nil, nil)
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	created := mgr.CreateSession(userID, dogID, platform)

	retrieved := mgr.GetActiveSession(userID, platform)
	if retrieved == nil {
		t.Fatal("GetActiveSession returned nil")
	}
	if retrieved.SessionID != created.SessionID {
		t.Errorf("SessionID = %s, want %s", retrieved.SessionID, created.SessionID)
	}
}

func TestGetActiveSession_NotFound(t *testing.T) {
	mgr := NewSessionManager(30, nil, nil)

	sess := mgr.GetActiveSession("nonexistent", "kakao")
	if sess != nil {
		t.Error("GetActiveSession should return nil for non-existent session")
	}
}

func TestGetActiveSession_Timeout(t *testing.T) {
	mgr := NewSessionManager(1, nil, nil) // 1 minute timeout
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	sess := mgr.CreateSession(userID, dogID, platform)

	// Manipulate LastActiveAt to 2 minutes ago
	sess.LastActiveAt = time.Now().Add(-2 * time.Minute)

	retrieved := mgr.GetActiveSession(userID, platform)
	if retrieved != nil {
		t.Error("GetActiveSession should return nil for timed out session")
	}

	// Verify session was ended
	if sess.IsActive {
		t.Error("Timed out session should be marked inactive")
	}
}

func TestGetOrCreateSession_ExistingActive(t *testing.T) {
	mgr := NewSessionManager(30, nil, nil)
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	created := mgr.CreateSession(userID, dogID, platform)

	retrieved := mgr.GetOrCreateSession(userID, dogID, platform)
	if retrieved.SessionID != created.SessionID {
		t.Errorf("GetOrCreateSession should return existing session, got SessionID = %s, want %s", retrieved.SessionID, created.SessionID)
	}
}

func TestGetOrCreateSession_CreatesNew(t *testing.T) {
	mgr := NewSessionManager(30, nil, nil)
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	sess := mgr.GetOrCreateSession(userID, dogID, platform)
	if sess == nil {
		t.Fatal("GetOrCreateSession returned nil")
	}
	if sess.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
	if sess.UserID != userID {
		t.Errorf("UserID = %s, want %s", sess.UserID, userID)
	}
	if !sess.IsActive {
		t.Error("New session should be active")
	}
}

func TestUpdateLastActive(t *testing.T) {
	mgr := NewSessionManager(30, nil, nil)
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	sess := mgr.CreateSession(userID, dogID, platform)
	originalTime := sess.LastActiveAt

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	mgr.UpdateLastActive(sess.SessionID)

	// Retrieve session to verify update
	retrieved := mgr.GetActiveSession(userID, platform)
	if retrieved == nil {
		t.Fatal("Session not found after update")
	}
	if !retrieved.LastActiveAt.After(originalTime) {
		t.Errorf("LastActiveAt should be updated, got %v, original %v", retrieved.LastActiveAt, originalTime)
	}
}

func TestEndSession(t *testing.T) {
	mgr := NewSessionManager(30, nil, nil)
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	sess := mgr.CreateSession(userID, dogID, platform)
	if !sess.IsActive {
		t.Fatal("Session should be active initially")
	}

	mgr.EndSession(sess.SessionID)

	if sess.IsActive {
		t.Error("Session should be inactive after EndSession")
	}

	// GetActiveSession should return nil for ended session
	retrieved := mgr.GetActiveSession(userID, platform)
	if retrieved != nil {
		t.Error("GetActiveSession should return nil for ended session")
	}
}

func TestSwitchDog(t *testing.T) {
	mgr := NewSessionManager(30, nil, nil)
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	sess := mgr.CreateSession(userID, dogID, platform)
	newDogID := "dog2"

	result := mgr.SwitchDog(sess.SessionID, newDogID)
	if result == nil {
		t.Fatal("SwitchDog returned nil")
	}
	if result.DogID != newDogID {
		t.Errorf("DogID = %s, want %s", result.DogID, newDogID)
	}
	if sess.DogID != newDogID {
		t.Errorf("Original session DogID = %s, want %s", sess.DogID, newDogID)
	}
}

func TestCleanupExpired(t *testing.T) {
	mgr := NewSessionManager(1, nil, nil) // 1 minute timeout
	userID := "user1"
	dogID := "dog1"
	platform := "kakao"

	sess := mgr.CreateSession(userID, dogID, platform)

	// Manipulate LastActiveAt to 2 minutes ago
	sess.LastActiveAt = time.Now().Add(-2 * time.Minute)

	if !sess.IsActive {
		t.Fatal("Session should be active before cleanup")
	}

	mgr.CleanupExpired()

	if sess.IsActive {
		t.Error("Session should be inactive after CleanupExpired")
	}
}
