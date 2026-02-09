package repository

import (
	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"gorm.io/gorm"
)

// MemoryRepository handles L1 (CategoryContext) and L2 (DynamicSummary) persistence.
type MemoryRepository struct {
	db *gorm.DB
}

// NewMemoryRepository creates a new MemoryRepository.
func NewMemoryRepository(db *gorm.DB) *MemoryRepository {
	return &MemoryRepository{db: db}
}

// --- L1 ---

// GetCategoryContext retrieves a single L1 context for a dog and category.
func (r *MemoryRepository) GetCategoryContext(dogID, category string) (*domain.DogCategoryContext, error) {
	var ctx domain.DogCategoryContext
	if err := r.db.Where("dog_id = ? AND category = ?", dogID, category).First(&ctx).Error; err != nil {
		return nil, err
	}
	return &ctx, nil
}

// GetAllCategoryContexts retrieves all L1 contexts for a dog.
func (r *MemoryRepository) GetAllCategoryContexts(dogID string) ([]domain.DogCategoryContext, error) {
	var contexts []domain.DogCategoryContext
	if err := r.db.Where("dog_id = ?", dogID).Order("category ASC").Find(&contexts).Error; err != nil {
		return nil, err
	}
	return contexts, nil
}

// UpsertCategoryContext saves or updates an L1 context.
func (r *MemoryRepository) UpsertCategoryContext(ctx *domain.DogCategoryContext) error {
	return r.db.Save(ctx).Error
}

// --- L2 ---

// GetDynamicSummary retrieves the L2 dynamic summary for a dog.
func (r *MemoryRepository) GetDynamicSummary(dogID string) (*domain.DogDynamicSummary, error) {
	var summary domain.DogDynamicSummary
	if err := r.db.Where("dog_id = ?", dogID).First(&summary).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

// UpsertDynamicSummary saves or updates an L2 dynamic summary.
func (r *MemoryRepository) UpsertDynamicSummary(summary *domain.DogDynamicSummary) error {
	return r.db.Save(summary).Error
}
