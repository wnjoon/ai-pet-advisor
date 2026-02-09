package domain

import "time"

// User represents a registered user (dog owner).
type User struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Dogs             []Dog             `gorm:"foreignKey:UserID" json:"dogs,omitempty"`
	PlatformAccounts []PlatformAccount `gorm:"foreignKey:UserID" json:"platform_accounts,omitempty"`
}

// PlatformAccount links a platform-specific ID to a User.
// MVP: schema only, account linking not yet implemented.
type PlatformAccount struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     string    `gorm:"type:uuid;index" json:"user_id"`
	Platform   string    `gorm:"index" json:"platform"`       // "kakao", "telegram", "web"
	PlatformID string    `gorm:"uniqueIndex" json:"platform_id"` // Platform-specific user ID
	CreatedAt  time.Time `json:"created_at"`
}
