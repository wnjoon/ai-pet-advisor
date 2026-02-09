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

// mockReconciliationAI provides a testable implementation of ReconciliationAI.
type mockReconciliationAI struct {
	reconcileFunc func(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error)
}

func (m *mockReconciliationAI) Reconcile(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error) {
	if m.reconcileFunc != nil {
		return m.reconcileFunc(existing, snippets)
	}
	return existing, nil
}

// TestMemoryIntegration_L1AppendAndRetrieve tests the basic L1 append and retrieve flow.
func TestMemoryIntegration_L1AppendAndRetrieve(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()
	category := "식사"

	// Create L1 context
	ctx := &domain.DogCategoryContext{
		DogID:       dogID,
		Category:    category,
		RecentItems: domain.ChatSnippets{},
		MaxItems:    5,
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(ctx).Error)

	// Append 3 snippets via MemoryManager
	snippets := []domain.ChatSnippet{
		{UserText: "강아지가 사료를 먹었어요", AIText: "좋은 식사 패턴입니다", Time: time.Now().Format(time.RFC3339)},
		{UserText: "오늘은 간식도 줬어요", AIText: "적절한 간식 양이네요", Time: time.Now().Add(1 * time.Minute).Format(time.RFC3339)},
		{UserText: "물도 잘 마셨어요", AIText: "수분 섭취가 좋습니다", Time: time.Now().Add(2 * time.Minute).Format(time.RFC3339)},
	}

	for _, snippet := range snippets {
		evicted, err := memManager.AppendToL1(dogID, category, snippet)
		require.NoError(t, err)
		assert.Nil(t, evicted, "no eviction should occur with 3 items and MaxItems=5")
	}

	// Retrieve via GetRecentContext
	retrieved, err := memManager.GetRecentContext(dogID, category)
	require.NoError(t, err)
	assert.Len(t, retrieved.RecentItems, 3)
	assert.Equal(t, "강아지가 사료를 먹었어요", retrieved.RecentItems[0].UserText)
	assert.Equal(t, "오늘은 간식도 줬어요", retrieved.RecentItems[1].UserText)
	assert.Equal(t, "물도 잘 마셨어요", retrieved.RecentItems[2].UserText)

	// Retrieve via GetAllRecentContexts
	allContexts, err := memManager.GetAllRecentContexts(dogID)
	require.NoError(t, err)
	assert.Len(t, allContexts, 1)
	assert.Equal(t, category, allContexts[0].Category)
	assert.Len(t, allContexts[0].RecentItems, 3)
}

// TestMemoryIntegration_L1OverflowTriggersL2 tests that L1 overflow triggers L2 reconciliation.
func TestMemoryIntegration_L1OverflowTriggersL2(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()
	category := "건강"

	// Create L1 context with MaxItems=3
	ctx := &domain.DogCategoryContext{
		DogID:       dogID,
		Category:    category,
		RecentItems: domain.ChatSnippets{},
		MaxItems:    3,
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(ctx).Error)

	// Create L2 summary with empty CategoryStatuses
	summary := &domain.DogDynamicSummary{
		DogID:            dogID,
		CategoryStatuses: domain.CategoryStatuses{},
		LastUpdated:      time.Now(),
	}
	require.NoError(t, db.Create(summary).Error)

	// Set up mock AI to return a CategoryStatus with baseline
	mockAI := &mockReconciliationAI{
		reconcileFunc: func(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error) {
			return domain.CategoryStatuses{
				{
					Category:   category,
					Baseline:   "사료 잘 먹음",
					LatestObs:  "",
					Confidence: 3,
					StatusTag:  "Stable",
					UpdatedAt:  time.Now(),
				},
			}, nil
		},
	}
	reconciler := NewReconciler(memRepo, mockAI)

	// Append 4 snippets - the 4th should trigger eviction
	snippets := []domain.ChatSnippet{
		{UserText: "snippet 1", AIText: "response 1", Time: time.Now().Format(time.RFC3339)},
		{UserText: "snippet 2", AIText: "response 2", Time: time.Now().Add(1 * time.Minute).Format(time.RFC3339)},
		{UserText: "snippet 3", AIText: "response 3", Time: time.Now().Add(2 * time.Minute).Format(time.RFC3339)},
		{UserText: "snippet 4", AIText: "response 4", Time: time.Now().Add(3 * time.Minute).Format(time.RFC3339)},
	}

	var evictedResult *EvictedResult
	for _, snippet := range snippets {
		evicted, err := memManager.AppendToL1(dogID, category, snippet)
		require.NoError(t, err)
		if evicted != nil {
			evictedResult = evicted
		}
	}

	// Verify eviction occurred on the 4th append
	require.NotNil(t, evictedResult, "eviction should have occurred")
	assert.Equal(t, dogID, evictedResult.DogID)
	assert.Equal(t, category, evictedResult.Category)
	assert.Len(t, evictedResult.Evicted, 1)
	assert.Equal(t, "snippet 1", evictedResult.Evicted[0].UserText)

	// Call reconciler to process evicted snippets
	err := reconciler.ReconcileOnOverflow(dogID, category, evictedResult.Evicted)
	require.NoError(t, err)

	// Verify L2 summary was updated
	updatedSummary, err := reconciler.GetDynamicSummary(dogID)
	require.NoError(t, err)
	assert.Len(t, updatedSummary.CategoryStatuses, 1)
	assert.Equal(t, category, updatedSummary.CategoryStatuses[0].Category)
	assert.Equal(t, "사료 잘 먹음", updatedSummary.CategoryStatuses[0].Baseline)
	assert.Equal(t, 3, updatedSummary.CategoryStatuses[0].Confidence)
	assert.Equal(t, "Stable", updatedSummary.CategoryStatuses[0].StatusTag)
}

// TestMemoryIntegration_SessionEndReconciliation tests the session end reconciliation flow.
func TestMemoryIntegration_SessionEndReconciliation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()
	category := "교육"

	// Create L1 context
	ctx := &domain.DogCategoryContext{
		DogID:       dogID,
		Category:    category,
		RecentItems: domain.ChatSnippets{},
		MaxItems:    10,
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(ctx).Error)

	// Create L2 summary with empty statuses
	summary := &domain.DogDynamicSummary{
		DogID:            dogID,
		CategoryStatuses: domain.CategoryStatuses{},
		LastUpdated:      time.Now(),
	}
	require.NoError(t, db.Create(summary).Error)

	// Set up mock AI to return updated statuses
	mockAI := &mockReconciliationAI{
		reconcileFunc: func(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error) {
			return domain.CategoryStatuses{
				{
					Category:   category,
					Baseline:   "기본 명령어 숙지",
					LatestObs:  "최근 '앉아' 명령 학습 중",
					Confidence: 4,
					StatusTag:  "Changing",
					UpdatedAt:  time.Now(),
				},
			}, nil
		},
	}
	reconciler := NewReconciler(memRepo, mockAI)

	// Append some snippets to L1
	snippets := []domain.ChatSnippet{
		{UserText: "앉아 훈련 시작했어요", AIText: "좋은 시작입니다", Time: time.Now().Format(time.RFC3339)},
		{UserText: "잘 따라하네요", AIText: "꾸준한 연습이 중요합니다", Time: time.Now().Add(1 * time.Minute).Format(time.RFC3339)},
		{UserText: "칭찬해주니 더 잘하네요", AIText: "긍정 강화가 효과적입니다", Time: time.Now().Add(2 * time.Minute).Format(time.RFC3339)},
	}

	for _, snippet := range snippets {
		_, err := memManager.AppendToL1(dogID, category, snippet)
		require.NoError(t, err)
	}

	// Collect all snippets and call ReconcileOnSessionEnd
	sessionCtx, err := memManager.GetRecentContext(dogID, category)
	require.NoError(t, err)

	err = reconciler.ReconcileOnSessionEnd(dogID, sessionCtx.RecentItems)
	require.NoError(t, err)

	// Verify L2 summary was updated
	updatedSummary, err := reconciler.GetDynamicSummary(dogID)
	require.NoError(t, err)
	assert.Len(t, updatedSummary.CategoryStatuses, 1)
	assert.Equal(t, category, updatedSummary.CategoryStatuses[0].Category)
	assert.Equal(t, "기본 명령어 숙지", updatedSummary.CategoryStatuses[0].Baseline)
	assert.Equal(t, "최근 '앉아' 명령 학습 중", updatedSummary.CategoryStatuses[0].LatestObs)
	assert.Equal(t, 4, updatedSummary.CategoryStatuses[0].Confidence)
	assert.Equal(t, "Changing", updatedSummary.CategoryStatuses[0].StatusTag)
}

// TestMemoryIntegration_RuleBasedFallback tests rule-based reconciliation when AI is nil.
func TestMemoryIntegration_RuleBasedFallback(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)

	dogID := uuid.New().String()
	category := "기분"

	// Create reconciler with nil AI (rule-based fallback)
	reconciler := NewReconciler(memRepo, nil)

	// Create L2 summary with initial CategoryStatuses (Baseline set, LatestObs empty)
	initialStatuses := domain.CategoryStatuses{
		{
			Category:   category,
			Baseline:   "평소 활발함",
			LatestObs:  "",
			Confidence: 3,
			StatusTag:  "Stable",
			UpdatedAt:  time.Now().Add(-1 * time.Hour),
		},
		{
			Category:   "수면",
			Baseline:   "밤에 잘 잠",
			LatestObs:  "",
			Confidence: 4,
			StatusTag:  "Stable",
			UpdatedAt:  time.Now().Add(-1 * time.Hour),
		},
	}

	summary := &domain.DogDynamicSummary{
		DogID:            dogID,
		CategoryStatuses: initialStatuses,
		LastUpdated:      time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, db.Create(summary).Error)

	// Call ReconcileOnSessionEnd with snippets
	snippets := domain.ChatSnippets{
		{UserText: "오늘 기분이 좋아보여요", AIText: "건강한 상태입니다", Time: time.Now().Format(time.RFC3339)},
		{UserText: "많이 놀았어요", AIText: "좋은 운동량입니다", Time: time.Now().Add(1 * time.Minute).Format(time.RFC3339)},
	}

	err := reconciler.ReconcileOnSessionEnd(dogID, snippets)
	require.NoError(t, err)

	// Verify LatestObs was updated via rule-based logic
	updatedSummary, err := reconciler.GetDynamicSummary(dogID)
	require.NoError(t, err)
	assert.Len(t, updatedSummary.CategoryStatuses, 2)

	// Find the "기분" category status
	var moodStatus *domain.CategoryStatus
	for i := range updatedSummary.CategoryStatuses {
		if updatedSummary.CategoryStatuses[i].Category == category {
			moodStatus = &updatedSummary.CategoryStatuses[i]
			break
		}
	}

	require.NotNil(t, moodStatus, "기분 category should exist")
	assert.Equal(t, "평소 활발함", moodStatus.Baseline, "Baseline should not change")
	assert.NotEmpty(t, moodStatus.LatestObs, "LatestObs should be updated by rule-based logic")
	assert.Contains(t, moodStatus.LatestObs, "많이 놀았어요", "LatestObs should contain latest snippet text")
	assert.Contains(t, moodStatus.LatestObs, "좋은 운동량입니다", "LatestObs should contain AI response")
}

// TestMemoryIntegration_MultiCategoryFlow tests E2E flow with multiple categories.
func TestMemoryIntegration_MultiCategoryFlow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)
	memManager := NewMemoryManager(memRepo)

	dogID := uuid.New().String()
	categories := []string{"식사", "건강", "교육"}

	// Create L1 contexts for multiple categories
	for _, cat := range categories {
		ctx := &domain.DogCategoryContext{
			DogID:       dogID,
			Category:    cat,
			RecentItems: domain.ChatSnippets{},
			MaxItems:    5,
			UpdatedAt:   time.Now(),
		}
		require.NoError(t, db.Create(ctx).Error)
	}

	// Create L2 summary
	summary := &domain.DogDynamicSummary{
		DogID:            dogID,
		CategoryStatuses: domain.CategoryStatuses{},
		LastUpdated:      time.Now(),
	}
	require.NoError(t, db.Create(summary).Error)

	// Set up mock AI that returns status for each category
	mockAI := &mockReconciliationAI{
		reconcileFunc: func(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error) {
			// Return updated statuses for all categories
			return domain.CategoryStatuses{
				{Category: "식사", Baseline: "규칙적인 식사", LatestObs: "오늘 사료 완식", Confidence: 4, StatusTag: "Stable", UpdatedAt: time.Now()},
				{Category: "건강", Baseline: "건강 상태 양호", LatestObs: "활동적", Confidence: 5, StatusTag: "Stable", UpdatedAt: time.Now()},
				{Category: "교육", Baseline: "기본 훈련 완료", LatestObs: "새로운 명령 학습 중", Confidence: 3, StatusTag: "Changing", UpdatedAt: time.Now()},
			}, nil
		},
	}
	reconciler := NewReconciler(memRepo, mockAI)

	// Append snippets to each category
	testData := map[string][]domain.ChatSnippet{
		"식사": {
			{UserText: "사료 완식했어요", AIText: "좋습니다", Time: time.Now().Format(time.RFC3339)},
			{UserText: "간식도 먹었어요", AIText: "적절합니다", Time: time.Now().Add(1 * time.Minute).Format(time.RFC3339)},
		},
		"건강": {
			{UserText: "활발하게 놀았어요", AIText: "건강한 상태입니다", Time: time.Now().Format(time.RFC3339)},
		},
		"교육": {
			{UserText: "앉아 훈련 중", AIText: "잘하고 있습니다", Time: time.Now().Format(time.RFC3339)},
			{UserText: "손 명령 시도", AIText: "좋은 진전입니다", Time: time.Now().Add(1 * time.Minute).Format(time.RFC3339)},
		},
	}

	// Append all snippets
	for cat, snippets := range testData {
		for _, snippet := range snippets {
			_, err := memManager.AppendToL1(dogID, cat, snippet)
			require.NoError(t, err)
		}
	}

	// Collect all snippets from all categories
	allSnippets := domain.ChatSnippets{}
	for cat := range testData {
		ctx, err := memManager.GetRecentContext(dogID, cat)
		require.NoError(t, err)
		allSnippets = append(allSnippets, ctx.RecentItems...)
	}

	// Reconcile session end
	err := reconciler.ReconcileOnSessionEnd(dogID, allSnippets)
	require.NoError(t, err)

	// Verify L2 summary has all categories
	updatedSummary, err := reconciler.GetDynamicSummary(dogID)
	require.NoError(t, err)
	assert.Len(t, updatedSummary.CategoryStatuses, 3)

	// Verify each category status
	statusMap := make(map[string]domain.CategoryStatus)
	for _, status := range updatedSummary.CategoryStatuses {
		statusMap[status.Category] = status
	}

	assert.Contains(t, statusMap, "식사")
	assert.Equal(t, "규칙적인 식사", statusMap["식사"].Baseline)
	assert.Equal(t, "오늘 사료 완식", statusMap["식사"].LatestObs)

	assert.Contains(t, statusMap, "건강")
	assert.Equal(t, "건강 상태 양호", statusMap["건강"].Baseline)
	assert.Equal(t, "활동적", statusMap["건강"].LatestObs)

	assert.Contains(t, statusMap, "교육")
	assert.Equal(t, "기본 훈련 완료", statusMap["교육"].Baseline)
	assert.Equal(t, "새로운 명령 학습 중", statusMap["교육"].LatestObs)
	assert.Equal(t, "Changing", statusMap["교육"].StatusTag)
}

// TestMemoryIntegration_EmptySnippetsNoReconciliation tests that empty snippets don't trigger reconciliation.
func TestMemoryIntegration_EmptySnippetsNoReconciliation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	memRepo := repository.NewMemoryRepository(db)

	dogID := uuid.New().String()

	// Create L2 summary
	initialStatuses := domain.CategoryStatuses{
		{Category: "식사", Baseline: "초기 상태", LatestObs: "", Confidence: 2, StatusTag: "Stable", UpdatedAt: time.Now().Add(-1 * time.Hour)},
	}
	summary := &domain.DogDynamicSummary{
		DogID:            dogID,
		CategoryStatuses: initialStatuses,
		LastUpdated:      time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, db.Create(summary).Error)

	mockAI := &mockReconciliationAI{
		reconcileFunc: func(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error) {
			t.Fatal("AI should not be called for empty snippets")
			return existing, nil
		},
	}
	reconciler := NewReconciler(memRepo, mockAI)

	// Call ReconcileOnSessionEnd with empty snippets
	err := reconciler.ReconcileOnSessionEnd(dogID, domain.ChatSnippets{})
	require.NoError(t, err)

	// Verify L2 summary was not changed
	retrievedSummary, err := reconciler.GetDynamicSummary(dogID)
	require.NoError(t, err)
	assert.Equal(t, "초기 상태", retrievedSummary.CategoryStatuses[0].Baseline)
	assert.Empty(t, retrievedSummary.CategoryStatuses[0].LatestObs)
}
