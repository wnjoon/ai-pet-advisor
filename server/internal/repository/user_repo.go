package repository

import (
	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"gorm.io/gorm"
)

// UserRepository handles User and PlatformAccount persistence.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new User.
func (r *UserRepository) Create(user *domain.User) error {
	return r.db.Create(user).Error
}

// GetByID retrieves a User by ID.
func (r *UserRepository) GetByID(id string) (*domain.User, error) {
	var user domain.User
	if err := r.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetOrCreateByPlatform finds an existing user by platform ID, or creates a new one.
func (r *UserRepository) GetOrCreateByPlatform(platform, platformID string) (*domain.User, error) {
	var account domain.PlatformAccount
	err := r.db.Where("platform = ? AND platform_id = ?", platform, platformID).First(&account).Error

	if err == nil {
		// Found existing account, load user
		var user domain.User
		if err := r.db.First(&user, "id = ?", account.UserID).Error; err != nil {
			return nil, err
		}
		return &user, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Create new user + platform account in a transaction
	var user domain.User
	txErr := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		account = domain.PlatformAccount{
			UserID:     user.ID,
			Platform:   platform,
			PlatformID: platformID,
		}
		return tx.Create(&account).Error
	})
	if txErr != nil {
		return nil, txErr
	}

	return &user, nil
}
