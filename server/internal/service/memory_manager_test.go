package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
	"github.com/wnjoon/ai-pet-advisor/server/internal/testutil"
)

// TestAppendToL1_Normal verifies that a snippet is successfully stored in L1.
func TestAppendToL1_Normal(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()
	category := "식사"

	// Pre-create DogCategoryContext with MaxItems=5
	ctx := &domain.DogCategoryContext{
		DogID:       dogID,
		Category:    category,
		RecentItems: domain.ChatSnippets{},
		MaxItems:    5,
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(ctx).Error)

	// Append a snippet
	snippet := domain.ChatSnippet{
		UserText: "강아지가 밥을 먹었어요",
		AIText:   "좋은 식사 패턴이네요",
		Time:     time.Now().Format(time.RFC3339),
	}

	evicted, err := memManager.AppendToL1(dogID, category, snippet)
	require.NoError(t, err)
	assert.Nil(t, evicted, "no eviction should occur with 1 item and MaxItems=5")

	// Verify it was stored
	retrieved, err := memManager.GetRecentContext(dogID, category)
	require.NoError(t, err)
	assert.Len(t, retrieved.RecentItems, 1)
	assert.Equal(t, "강아지가 밥을 먹었어요", retrieved.RecentItems[0].UserText)
	assert.Equal(t, "좋은 식사 패턴이네요", retrieved.RecentItems[0].AIText)
}

// TestAppendToL1_Eviction verifies that the oldest item is evicted when MaxItems is exceeded.
func TestAppendToL1_Eviction(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()
	category := "건강"

	// Pre-create DogCategoryContext with MaxItems=3
	ctx := &domain.DogCategoryContext{
		DogID:       dogID,
		Category:    category,
		RecentItems: domain.ChatSnippets{},
		MaxItems:    3,
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(ctx).Error)

	// Append 3 snippets (no eviction yet)
	for i := 1; i <= 3; i++ {
		snippet := domain.ChatSnippet{
			UserText: "user text " + string(rune('0'+i)),
			AIText:   "ai text " + string(rune('0'+i)),
			Time:     time.Now().Add(time.Duration(i) * time.Second).Format(time.RFC3339),
		}
		evicted, err := memManager.AppendToL1(dogID, category, snippet)
		require.NoError(t, err)
		assert.Nil(t, evicted, "no eviction for items 1-3")
	}

	// Append 4th snippet - should evict the 1st
	snippet4 := domain.ChatSnippet{
		UserText: "user text 4",
		AIText:   "ai text 4",
		Time:     time.Now().Add(4 * time.Second).Format(time.RFC3339),
	}

	evicted, err := memManager.AppendToL1(dogID, category, snippet4)
	require.NoError(t, err)
	require.NotNil(t, evicted, "eviction should occur")
	assert.Equal(t, dogID, evicted.DogID)
	assert.Equal(t, category, evicted.Category)
	assert.Len(t, evicted.Evicted, 1)
	assert.Equal(t, "user text 1", evicted.Evicted[0].UserText)

	// Verify only items 2, 3, 4 remain
	retrieved, err := memManager.GetRecentContext(dogID, category)
	require.NoError(t, err)
	assert.Len(t, retrieved.RecentItems, 3)
	assert.Equal(t, "user text 2", retrieved.RecentItems[0].UserText)
	assert.Equal(t, "user text 3", retrieved.RecentItems[1].UserText)
	assert.Equal(t, "user text 4", retrieved.RecentItems[2].UserText)
}

// TestAppendToL1_MultipleEvictions verifies that multiple items are evicted if many are added at once.
func TestAppendToL1_MultipleEvictions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()
	category := "교육"

	// Pre-populate with 2 items, MaxItems=2
	existingItems := domain.ChatSnippets{
		{UserText: "old 1", AIText: "response 1", Time: time.Now().Add(-2 * time.Hour).Format(time.RFC3339)},
		{UserText: "old 2", AIText: "response 2", Time: time.Now().Add(-1 * time.Hour).Format(time.RFC3339)},
	}

	ctx := &domain.DogCategoryContext{
		DogID:       dogID,
		Category:    category,
		RecentItems: existingItems,
		MaxItems:    2,
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(ctx).Error)

	// Append 1 more snippet - should evict "old 1"
	newSnippet := domain.ChatSnippet{
		UserText: "new snippet",
		AIText:   "new response",
		Time:     time.Now().Format(time.RFC3339),
	}

	evicted, err := memManager.AppendToL1(dogID, category, newSnippet)
	require.NoError(t, err)
	require.NotNil(t, evicted)
	assert.Len(t, evicted.Evicted, 1)
	assert.Equal(t, "old 1", evicted.Evicted[0].UserText)

	// Verify only "old 2" and "new snippet" remain
	retrieved, err := memManager.GetRecentContext(dogID, category)
	require.NoError(t, err)
	assert.Len(t, retrieved.RecentItems, 2)
	assert.Equal(t, "old 2", retrieved.RecentItems[0].UserText)
	assert.Equal(t, "new snippet", retrieved.RecentItems[1].UserText)
}

// TestGetRecentContext verifies retrieval of a specific category's L1 context.
func TestGetRecentContext(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()
	category := "기분"

	// Create context with some items
	items := domain.ChatSnippets{
		{UserText: "happy dog", AIText: "great mood", Time: time.Now().Format(time.RFC3339)},
		{UserText: "playful", AIText: "energetic", Time: time.Now().Format(time.RFC3339)},
	}

	ctx := &domain.DogCategoryContext{
		DogID:       dogID,
		Category:    category,
		RecentItems: items,
		MaxItems:    10,
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(ctx).Error)

	// Retrieve context
	retrieved, err := memManager.GetRecentContext(dogID, category)
	require.NoError(t, err)
	assert.Equal(t, dogID, retrieved.DogID)
	assert.Equal(t, category, retrieved.Category)
	assert.Len(t, retrieved.RecentItems, 2)
	assert.Equal(t, "happy dog", retrieved.RecentItems[0].UserText)
	assert.Equal(t, "playful", retrieved.RecentItems[1].UserText)
	assert.Equal(t, 10, retrieved.MaxItems)
}

// TestGetAllRecentContexts verifies retrieval of all categories for a dog.
func TestGetAllRecentContexts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()

	// Create multiple category contexts
	categories := []string{"식사", "건강", "교육"}
	for _, cat := range categories {
		ctx := &domain.DogCategoryContext{
			DogID:       dogID,
			Category:    cat,
			RecentItems: domain.ChatSnippets{{UserText: "test", AIText: "test", Time: time.Now().Format(time.RFC3339)}},
			MaxItems:    20,
			UpdatedAt:   time.Now(),
		}
		require.NoError(t, db.Create(ctx).Error)
	}

	// Create a context for a different dog (should not be returned)
	otherDogID := uuid.New().String()
	otherCtx := &domain.DogCategoryContext{
		DogID:       otherDogID,
		Category:    "수면",
		RecentItems: domain.ChatSnippets{},
		MaxItems:    15,
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(otherCtx).Error)

	// Retrieve all contexts for the first dog
	contexts, err := memManager.GetAllRecentContexts(dogID)
	require.NoError(t, err)
	assert.Len(t, contexts, 3, "should only return contexts for the specified dog")

	// Verify categories are returned (should be ordered by category ASC)
	catNames := make([]string, len(contexts))
	for i, ctx := range contexts {
		catNames[i] = ctx.Category
	}
	assert.Contains(t, catNames, "식사")
	assert.Contains(t, catNames, "건강")
	assert.Contains(t, catNames, "교육")
}

// TestUpdateMaxItems verifies that MaxItems is updated for all categories of a dog.
func TestUpdateMaxItems(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()

	// Create contexts for multiple categories
	categories := []string{"식사", "건강", "수면"}
	for _, cat := range categories {
		ctx := &domain.DogCategoryContext{
			DogID:       dogID,
			Category:    cat,
			RecentItems: domain.ChatSnippets{},
			MaxItems:    10,
			UpdatedAt:   time.Now(),
		}
		require.NoError(t, db.Create(ctx).Error)
	}

	// Update MaxItems to 50
	err := memManager.UpdateMaxItems(dogID, 50)
	require.NoError(t, err)

	// Verify all categories have updated MaxItems
	contexts, err := memManager.GetAllRecentContexts(dogID)
	require.NoError(t, err)
	for _, ctx := range contexts {
		assert.Equal(t, 50, ctx.MaxItems, "MaxItems should be updated for category %s", ctx.Category)
	}

	// Test error case: maxItems <= 0
	err = memManager.UpdateMaxItems(dogID, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxItems must be greater than 0")

	err = memManager.UpdateMaxItems(dogID, -5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxItems must be greater than 0")
}
