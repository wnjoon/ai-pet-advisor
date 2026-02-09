package service

import (
	"errors"
	"time"

	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
)

// MemoryManager handles L1 memory operations (append, retrieve, eviction).
type MemoryManager struct {
	memoryRepo *repository.MemoryRepository
}

// NewMemoryManager creates a new MemoryManager.
func NewMemoryManager(memoryRepo *repository.MemoryRepository) *MemoryManager {
	return &MemoryManager{memoryRepo: memoryRepo}
}

// EvictedResult holds the evicted snippets when L1 overflows, for L2 reconciliation.
type EvictedResult struct {
	DogID    string
	Category string
	Evicted  domain.ChatSnippets
}

// AppendToL1 adds a ChatSnippet to the specified category's L1 context.
// If MaxItems is exceeded, the oldest items are evicted and returned for L2 processing.
// Returns nil EvictedResult if no overflow occurred.
func (m *MemoryManager) AppendToL1(dogID, category string, snippet domain.ChatSnippet) (*EvictedResult, error) {
	if snippet.Time == "" {
		snippet.Time = time.Now().Format(time.RFC3339)
	}

	ctx, err := m.memoryRepo.GetCategoryContext(dogID, category)
	if err != nil {
		return nil, errors.New("category context not found for dog")
	}

	ctx.RecentItems = append(ctx.RecentItems, snippet)

	var evicted domain.ChatSnippets
	if len(ctx.RecentItems) > ctx.MaxItems {
		overflow := len(ctx.RecentItems) - ctx.MaxItems
		evicted = make(domain.ChatSnippets, overflow)
		copy(evicted, ctx.RecentItems[:overflow])
		ctx.RecentItems = ctx.RecentItems[overflow:]
	}

	ctx.UpdatedAt = time.Now()
	if err := m.memoryRepo.UpsertCategoryContext(ctx); err != nil {
		return nil, err
	}

	if len(evicted) > 0 {
		return &EvictedResult{
			DogID:    dogID,
			Category: category,
			Evicted:  evicted,
		}, nil
	}

	return nil, nil
}

// GetRecentContext retrieves L1 recent items for a specific category.
func (m *MemoryManager) GetRecentContext(dogID, category string) (*domain.DogCategoryContext, error) {
	return m.memoryRepo.GetCategoryContext(dogID, category)
}

// GetAllRecentContexts retrieves all L1 category contexts for a dog.
func (m *MemoryManager) GetAllRecentContexts(dogID string) ([]domain.DogCategoryContext, error) {
	return m.memoryRepo.GetAllCategoryContexts(dogID)
}

// UpdateMaxItems updates the MaxItems limit for all categories of a dog (tier system).
func (m *MemoryManager) UpdateMaxItems(dogID string, maxItems int) error {
	if maxItems <= 0 {
		return errors.New("maxItems must be greater than 0")
	}

	contexts, err := m.memoryRepo.GetAllCategoryContexts(dogID)
	if err != nil {
		return err
	}

	for i := range contexts {
		contexts[i].MaxItems = maxItems
		if err := m.memoryRepo.UpsertCategoryContext(&contexts[i]); err != nil {
			return err
		}
	}

	return nil
}
