package domain

import "time"

// ChatSession tracks an active conversation session between a user and the AI.
type ChatSession struct {
	SessionID    string    `json:"session_id"`
	UserID       string    `json:"user_id"`
	DogID        string    `json:"dog_id"`    // Currently active dog
	Platform     string    `json:"platform"`   // "kakao", "telegram"
	StartedAt    time.Time `json:"started_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	IsActive     bool      `json:"is_active"`
}
