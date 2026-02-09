package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/genai"

	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
)

// GeminiReconciliationAI implements ReconciliationAI using Gemini API directly.
type GeminiReconciliationAI struct {
	client    *genai.Client
	modelName string
}

// NewGeminiReconciliationAI creates a GeminiReconciliationAI.
func NewGeminiReconciliationAI(ctx context.Context, apiKey, modelName string) (*GeminiReconciliationAI, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	return &GeminiReconciliationAI{
		client:    client,
		modelName: modelName,
	}, nil
}

// Reconcile sends existing statuses and new snippets to Gemini for L2 reconciliation.
func (g *GeminiReconciliationAI) Reconcile(existing domain.CategoryStatuses, snippets domain.ChatSnippets) (domain.CategoryStatuses, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	existingJSON, _ := json.MarshalIndent(existing, "", "  ")
	snippetsJSON, _ := json.MarshalIndent(snippets, "", "  ")

	prompt := fmt.Sprintf(`%s

## 기존 CategoryStatuses
%s

## 새로운 대화 조각들
%s

위 규칙에 따라 갱신된 CategoryStatuses JSON 배열만 반환하세요.`, ReconciliationPrompt, string(existingJSON), string(snippetsJSON))

	resp, err := g.client.Models.GenerateContent(ctx, g.modelName, []*genai.Content{
		genai.NewContentFromText(prompt, "user"),
	}, &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	})
	if err != nil {
		return nil, fmt.Errorf("gemini reconciliation call failed: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return existing, nil
	}

	// Extract text from response
	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			responseText += part.Text
		}
	}

	// Parse the JSON response
	var updated domain.CategoryStatuses
	if err := json.Unmarshal([]byte(responseText), &updated); err != nil {
		return nil, fmt.Errorf("failed to parse reconciliation response: %w", err)
	}

	return updated, nil
}
