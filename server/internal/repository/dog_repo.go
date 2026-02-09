package repository

import (
	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"gorm.io/gorm"
)

// DogRepository handles Dog persistence.
type DogRepository struct {
	db *gorm.DB
}

// NewDogRepository creates a new DogRepository.
func NewDogRepository(db *gorm.DB) *DogRepository {
	return &DogRepository{db: db}
}

// Create inserts a new Dog along with its initial L1 contexts and L2 summary.
func (r *DogRepository) Create(dog *domain.Dog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(dog).Error; err != nil {
			return err
		}

		// Initialize L1: empty context for each of the 7 categories
		for _, cat := range domain.Categories {
			ctx := domain.DogCategoryContext{
				DogID:       dog.ID,
				Category:    cat,
				RecentItems: domain.ChatSnippets{},
				MaxItems:    30, // Default tier
			}
			if err := tx.Create(&ctx).Error; err != nil {
				return err
			}
		}

		// Initialize L2: empty dynamic summary
		summary := domain.DogDynamicSummary{
			DogID:            dog.ID,
			CategoryStatuses: domain.CategoryStatuses{},
		}
		if err := tx.Create(&summary).Error; err != nil {
			return err
		}

		return nil
	})
}

// GetByID retrieves a Dog by ID.
func (r *DogRepository) GetByID(id string) (*domain.Dog, error) {
	var dog domain.Dog
	if err := r.db.First(&dog, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &dog, nil
}

// GetByUserID retrieves all Dogs for a given user.
func (r *DogRepository) GetByUserID(userID string) ([]domain.Dog, error) {
	var dogs []domain.Dog
	if err := r.db.Where("user_id = ?", userID).Order("created_at ASC").Find(&dogs).Error; err != nil {
		return nil, err
	}
	return dogs, nil
}

// Update modifies mutable fields of a Dog.
func (r *DogRepository) Update(dog *domain.Dog) error {
	return r.db.Model(dog).Select(
		"Weight", "Neutered", "ProfilePhoto", "MedicalNotes", "UpdatedAt",
	).Updates(dog).Error
}

// Delete removes a Dog and its associated L1/L2 data.
func (r *DogRepository) Delete(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete L1 contexts
		if err := tx.Where("dog_id = ?", id).Delete(&domain.DogCategoryContext{}).Error; err != nil {
			return err
		}
		// Delete L2 summary
		if err := tx.Where("dog_id = ?", id).Delete(&domain.DogDynamicSummary{}).Error; err != nil {
			return err
		}
		// Delete dog
		if err := tx.Where("id = ?", id).Delete(&domain.Dog{}).Error; err != nil {
			return err
		}
		return nil
	})
}
