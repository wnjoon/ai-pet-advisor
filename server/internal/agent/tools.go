package agent

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
	"github.com/wnjoon/ai-pet-advisor/server/internal/service"
)

// ToolDeps holds the dependencies needed by agent tools.
type ToolDeps struct {
	DogRepo       *repository.DogRepository
	MemoryManager *service.MemoryManager
	Reconciler    *service.Reconciler
	MemoryRepo    *repository.MemoryRepository
}

// --- load_context ---

// LoadContextInput is the input schema for the load_context tool.
type LoadContextInput struct {
	DogID string `json:"dog_id"`
}

// LoadContextOutput is the output schema for the load_context tool.
type LoadContextOutput struct {
	Dog        *domain.Dog                  `json:"dog"`
	L1Contexts []domain.DogCategoryContext  `json:"l1_contexts"`
	L2Summary  *domain.DogDynamicSummary    `json:"l2_summary"`
}

// NewLoadContextTool creates the load_context function tool.
func NewLoadContextTool(deps *ToolDeps) (tool.Tool, error) {
	handler := func(ctx tool.Context, input LoadContextInput) (LoadContextOutput, error) {
		dog, err := deps.DogRepo.GetByID(input.DogID)
		if err != nil {
			return LoadContextOutput{}, fmt.Errorf("dog not found: %s", input.DogID)
		}

		l1Contexts, err := deps.MemoryManager.GetAllRecentContexts(input.DogID)
		if err != nil {
			return LoadContextOutput{}, fmt.Errorf("failed to load L1 contexts: %w", err)
		}

		l2Summary, err := deps.Reconciler.GetDynamicSummary(input.DogID)
		if err != nil {
			return LoadContextOutput{}, fmt.Errorf("failed to load L2 summary: %w", err)
		}

		return LoadContextOutput{
			Dog:        dog,
			L1Contexts: l1Contexts,
			L2Summary:  l2Summary,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "load_context",
		Description: "반려견 프로필, L1 최근 대화 컨텍스트, L2 장기 요약을 로드합니다. 대화 시작 시 반드시 호출하세요.",
	}, handler)
}

// --- search_history ---

// SearchHistoryInput is the input schema for the search_history tool.
type SearchHistoryInput struct {
	DogID    string `json:"dog_id"`
	Query    string `json:"query"`
	Category string `json:"category,omitempty"`
}

// SearchHistoryOutput is the output schema for the search_history tool.
type SearchHistoryOutput struct {
	Matches  []HistoryMatch `json:"matches"`
	Baseline string         `json:"baseline,omitempty"`
}

// HistoryMatch represents a matched conversation snippet.
type HistoryMatch struct {
	Category string `json:"category"`
	UserText string `json:"user_text"`
	AIText   string `json:"ai_text"`
	Time     string `json:"time"`
}

// NewSearchHistoryTool creates the search_history function tool.
func NewSearchHistoryTool(deps *ToolDeps) (tool.Tool, error) {
	handler := func(ctx tool.Context, input SearchHistoryInput) (SearchHistoryOutput, error) {
		var matches []HistoryMatch

		contexts, err := deps.MemoryManager.GetAllRecentContexts(input.DogID)
		if err != nil {
			return SearchHistoryOutput{}, fmt.Errorf("failed to search history: %w", err)
		}

		query := strings.ToLower(input.Query)
		for _, c := range contexts {
			if input.Category != "" && c.Category != input.Category {
				continue
			}
			for _, snippet := range c.RecentItems {
				if strings.Contains(strings.ToLower(snippet.UserText), query) ||
					strings.Contains(strings.ToLower(snippet.AIText), query) {
					matches = append(matches, HistoryMatch{
						Category: c.Category,
						UserText: snippet.UserText,
						AIText:   snippet.AIText,
						Time:     snippet.Time,
					})
				}
			}
		}

		// Include L2 baseline for the category if specified
		var baseline string
		if input.Category != "" {
			summary, err := deps.Reconciler.GetDynamicSummary(input.DogID)
			if err == nil {
				for _, cs := range summary.CategoryStatuses {
					if cs.Category == input.Category {
						baseline = cs.Baseline
						break
					}
				}
			}
		}

		return SearchHistoryOutput{
			Matches:  matches,
			Baseline: baseline,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "search_history",
		Description: "과거 대화 이력에서 키워드로 검색합니다. 카테고리별 필터링과 L2 Baseline 조회가 가능합니다.",
	}, handler)
}

// --- update_profile ---

// UpdateProfileInput is the input schema for the update_profile tool.
type UpdateProfileInput struct {
	DogID    string   `json:"dog_id"`
	Weight   *float64 `json:"weight,omitempty"`
	Neutered *bool    `json:"neutered,omitempty"`
}

// UpdateProfileOutput is the output schema for the update_profile tool.
type UpdateProfileOutput struct {
	Updated bool        `json:"updated"`
	Dog     *domain.Dog `json:"dog"`
}

// NewUpdateProfileTool creates the update_profile function tool.
func NewUpdateProfileTool(deps *ToolDeps) (tool.Tool, error) {
	handler := func(ctx tool.Context, input UpdateProfileInput) (UpdateProfileOutput, error) {
		dog, err := deps.DogRepo.GetByID(input.DogID)
		if err != nil {
			return UpdateProfileOutput{}, fmt.Errorf("dog not found: %s", input.DogID)
		}

		changed := false
		if input.Weight != nil && *input.Weight > 0 {
			dog.Weight = *input.Weight
			changed = true
		}
		if input.Neutered != nil {
			dog.Neutered = *input.Neutered
			changed = true
		}

		if !changed {
			return UpdateProfileOutput{Updated: false, Dog: dog}, nil
		}

		if err := deps.DogRepo.Update(dog); err != nil {
			return UpdateProfileOutput{}, fmt.Errorf("failed to update profile: %w", err)
		}

		return UpdateProfileOutput{Updated: true, Dog: dog}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "update_profile",
		Description: "반려견 프로필의 가변 필드(몸무게, 중성화 여부)를 업데이트합니다. 사용자가 대화 중 변경을 요청할 때 호출하세요.",
	}, handler)
}

// --- save_and_reconcile ---

// SaveAndReconcileInput is the input schema for the save_and_reconcile tool.
type SaveAndReconcileInput struct {
	DogID        string   `json:"dog_id"`
	Categories   []string `json:"categories"`
	UserText     string   `json:"user_text"`
	AIText       string   `json:"ai_text"`
	Urgency      string   `json:"urgency,omitempty"`
	BehaviorTags []string `json:"behavior_tags,omitempty"`
}

// SaveAndReconcileOutput is the output schema for the save_and_reconcile tool.
type SaveAndReconcileOutput struct {
	Saved     bool     `json:"saved"`
	Evicted   []string `json:"evicted_categories,omitempty"`
}

// NewSaveAndReconcileTool creates the save_and_reconcile function tool.
func NewSaveAndReconcileTool(deps *ToolDeps) (tool.Tool, error) {
	handler := func(ctx tool.Context, input SaveAndReconcileInput) (SaveAndReconcileOutput, error) {
		snippet := domain.ChatSnippet{
			UserText: input.UserText,
			AIText:   input.AIText,
			Time:     time.Now().Format(time.RFC3339),
		}

		var evictedCategories []string

		for _, category := range input.Categories {
			result, err := deps.MemoryManager.AppendToL1(input.DogID, category, snippet)
			if err != nil {
				return SaveAndReconcileOutput{}, fmt.Errorf("failed to save to L1 (%s): %w", category, err)
			}

			// If L1 overflowed, trigger L2 reconciliation
			if result != nil {
				evictedCategories = append(evictedCategories, category)
				_ = deps.Reconciler.ReconcileOnOverflow(input.DogID, category, result.Evicted)
			}
		}

		return SaveAndReconcileOutput{
			Saved:   true,
			Evicted: evictedCategories,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "save_and_reconcile",
		Description: "대화 내용을 L1에 저장하고, L1 초과 시 L2 Reconciliation을 실행합니다. 대화 종료 시 호출하세요.",
	}, handler)
}
