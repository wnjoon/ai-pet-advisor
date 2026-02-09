package service

import (
	"errors"
	"time"

	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
)

// ReconciliationAI abstracts the AI call for L2 reconciliation.
// Phase 1.6 (ADK) will provide the concrete implementation.
type ReconciliationAI interface {
	// Reconcile takes existing L2 statuses and new snippets,
	// returns updated CategoryStatuses via AI analysis.
	Reconcile(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error)
}

// Reconciler handles L2 dynamic summary updates via AI-driven reconciliation.
type Reconciler struct {
	memoryRepo *repository.MemoryRepository
	ai         ReconciliationAI
}

// NewReconciler creates a new Reconciler.
// Pass nil for ai to use a no-op stub (useful before ADK is connected).
func NewReconciler(memoryRepo *repository.MemoryRepository, ai ReconciliationAI) *Reconciler {
	return &Reconciler{memoryRepo: memoryRepo, ai: ai}
}

// ReconcileOnSessionEnd processes all session snippets and updates L2.
// Called when a chat session ends (timeout or explicit close).
func (r *Reconciler) ReconcileOnSessionEnd(dogID string, sessionSnippets domain.ChatSnippets) error {
	if len(sessionSnippets) == 0 {
		return nil
	}

	summary, err := r.memoryRepo.GetDynamicSummary(dogID)
	if err != nil {
		return errors.New("failed to load L2 summary: " + err.Error())
	}

	updated, err := r.reconcile(summary.CategoryStatuses, sessionSnippets)
	if err != nil {
		return err
	}

	summary.CategoryStatuses = updated
	summary.LastUpdated = time.Now()
	return r.memoryRepo.UpsertDynamicSummary(summary)
}

// ReconcileOnOverflow handles L1 eviction by incorporating evicted snippets into L2.
// Called by MemoryManager when AppendToL1 triggers overflow.
func (r *Reconciler) ReconcileOnOverflow(dogID, category string, evicted domain.ChatSnippets) error {
	if len(evicted) == 0 {
		return nil
	}

	summary, err := r.memoryRepo.GetDynamicSummary(dogID)
	if err != nil {
		return errors.New("failed to load L2 summary: " + err.Error())
	}

	updated, err := r.reconcile(summary.CategoryStatuses, evicted)
	if err != nil {
		return err
	}

	summary.CategoryStatuses = updated
	summary.LastUpdated = time.Now()
	return r.memoryRepo.UpsertDynamicSummary(summary)
}

// GetDynamicSummary retrieves the current L2 summary for a dog.
func (r *Reconciler) GetDynamicSummary(dogID string) (*domain.DogDynamicSummary, error) {
	return r.memoryRepo.GetDynamicSummary(dogID)
}

// reconcile delegates to AI if available, otherwise applies rule-based fallback.
func (r *Reconciler) reconcile(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error) {
	if r.ai != nil {
		return r.ai.Reconcile(existing, snippets)
	}
	// Fallback: rule-based reconciliation (no AI)
	return r.ruleBasedReconcile(existing, snippets)
}

// ruleBasedReconcile is a simple fallback that updates LatestObs timestamps
// without AI analysis. Used before ADK integration is complete.
func (r *Reconciler) ruleBasedReconcile(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error) {
	if len(existing) == 0 {
		return existing, nil
	}

	now := time.Now()
	latestSnippet := ""
	if len(snippets) > 0 {
		last := snippets[len(snippets)-1]
		latestSnippet = last.UserText + " → " + last.AIText
	}

	// Update all categories' LatestObs with the latest snippet summary
	updated := make(domain.CategoryStatuses, len(existing))
	copy(updated, existing)

	for i := range updated {
		if updated[i].LatestObs == "" && latestSnippet != "" {
			updated[i].LatestObs = latestSnippet
			updated[i].UpdatedAt = now
		}
	}

	return updated, nil
}
