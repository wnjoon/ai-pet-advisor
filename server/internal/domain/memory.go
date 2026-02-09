package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// --- L1: Recent Context ---

// ChatSnippet is the minimal unit of an L1 conversation fragment.
type ChatSnippet struct {
	UserText string `json:"u"` // User utterance summary
	AIText   string `json:"a"` // AI response summary
	Time     string `json:"t"` // ISO8601 timestamp
}

// ChatSnippets is a slice of ChatSnippet that implements GORM JSONB support.
type ChatSnippets []ChatSnippet

func (s ChatSnippets) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *ChatSnippets) Scan(value any) error {
	if value == nil {
		*s = ChatSnippets{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if !ok {
			return errors.New("ChatSnippets.Scan: unsupported type")
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, s)
}

// DogCategoryContext is the L1 DB model: per-category recent conversation context.
type DogCategoryContext struct {
	ID          uint         `gorm:"primaryKey" json:"id"`
	DogID       string       `gorm:"uniqueIndex:idx_dog_cat;type:uuid;not null" json:"dog_id"`
	Category    string       `gorm:"uniqueIndex:idx_dog_cat;not null" json:"category"`
	RecentItems ChatSnippets `gorm:"type:jsonb;default:'[]'" json:"recent_items"`
	MaxItems    int          `gorm:"default:30" json:"max_items"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// --- L2: Dynamic Summary ---

// CategoryStatus tracks the state and confidence for a single category in L2.
type CategoryStatus struct {
	Category   string    `json:"c"`    // 식사, 교육, 건강, 기분, 수면, 사회화, 환경
	Baseline   string    `json:"b"`    // Long-term dominant pattern
	LatestObs  string    `json:"l"`    // Recent transient observation
	Confidence int       `json:"conf"` // Pattern confidence (1-5)
	StatusTag  string    `json:"tag"`  // Stable / Changing / Anomaly
	UpdatedAt  time.Time `json:"u_at"` // Last update for this category
}

// CategoryStatuses is a slice of CategoryStatus that implements GORM JSONB support.
type CategoryStatuses []CategoryStatus

func (s CategoryStatuses) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *CategoryStatuses) Scan(value any) error {
	if value == nil {
		*s = CategoryStatuses{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if !ok {
			return errors.New("CategoryStatuses.Scan: unsupported type")
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, s)
}

// DogDynamicSummary is the L2 DB model: a dog's dynamic state snapshot.
type DogDynamicSummary struct {
	DogID            string           `gorm:"primaryKey;type:uuid" json:"dog_id"`
	CategoryStatuses CategoryStatuses `gorm:"type:jsonb;default:'[]'" json:"category_statuses"`
	RapportNotes     string           `gorm:"type:text" json:"rapport_notes,omitempty"` // MVP: unused, reserved
	LastUpdated      time.Time        `json:"last_updated"`
}

// Categories is the canonical list of all supported categories.
var Categories = []string{
	"식사", "교육", "건강", "기분", "수면", "사회화", "환경",
}
