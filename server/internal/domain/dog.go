package domain

import "time"

// Dog represents a registered pet dog with immutable and mutable fields.
type Dog struct {
	// === Immutable Fields ===
	ID                string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID            string    `gorm:"type:uuid;index;not null" json:"user_id"`
	Name              string    `gorm:"not null" json:"name"`
	Breed             string    `gorm:"not null" json:"breed"`
	Birthday          time.Time `gorm:"not null" json:"birthday"`
	BirthdayEstimated bool      `gorm:"default:false" json:"birthday_estimated"` // true = approximate
	Gender            string    `gorm:"not null" json:"gender"`                  // "male" / "female"
	CreatedAt         time.Time `json:"created_at"`

	// === Mutable Fields ===
	Weight       float64   `gorm:"type:decimal(5,2)" json:"weight"`
	Neutered     bool      `gorm:"default:false" json:"neutered"`
	ProfilePhoto string    `gorm:"type:text" json:"profile_photo,omitempty"`
	MedicalNotes string    `gorm:"type:text" json:"medical_notes,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relations
	CategoryContexts []DogCategoryContext `gorm:"foreignKey:DogID" json:"category_contexts,omitempty"`
	DynamicSummary   *DogDynamicSummary   `gorm:"foreignKey:DogID" json:"dynamic_summary,omitempty"`
}
