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
	DogID string `json:"dog_id" jsonschema:"description=The unique identifier of the dog"`
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
	DogID     string  `json:"dog_id" jsonschema:"description=The unique identifier of the dog"`
	Query     string  `json:"query" jsonschema:"description=Search keyword or phrase"`
	Category  string  `json:"category,omitempty" jsonschema:"description=Optional category filter (식사/교육/건강/기분/수면/사회화/환경)"`
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

// --- save_and_reconcile ---

// SaveAndReconcileInput is the input schema for the save_and_reconcile tool.
type SaveAndReconcileInput struct {
	DogID        string   `json:"dog_id" jsonschema:"description=The unique identifier of the dog"`
	Categories   []string `json:"categories" jsonschema:"description=Categories this conversation belongs to"`
	UserText     string   `json:"user_text" jsonschema:"description=Summary of the user's message"`
	AIText       string   `json:"ai_text" jsonschema:"description=Summary of the AI's response"`
	Urgency      string   `json:"urgency,omitempty" jsonschema:"description=Urgency level: L1/L2/L3/L4"`
	BehaviorTags []string `json:"behavior_tags,omitempty" jsonschema:"description=Optional behavior tags"`
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
