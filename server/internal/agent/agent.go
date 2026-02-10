package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/genai"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
)

// AdvisorAgent wraps the ADK agent and runner for the pet advisor chatbot.
type AdvisorAgent struct {
	runner         *runner.Runner
	sessionService session.Service
	appName        string
	deps           *ToolDeps
}

// Config holds the configuration for creating an AdvisorAgent.
type Config struct {
	GoogleAPIKey string
	GeminiModel  string
	Deps         *ToolDeps
}

// New creates a new AdvisorAgent with the given configuration.
func New(ctx context.Context, cfg Config) (*AdvisorAgent, error) {
	// Create Gemini model
	model, err := gemini.NewModel(ctx, cfg.GeminiModel, &genai.ClientConfig{
		APIKey: cfg.GoogleAPIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini model: %w", err)
	}

	// Create function tools
	loadContextTool, err := NewLoadContextTool(cfg.Deps)
	if err != nil {
		return nil, fmt.Errorf("failed to create load_context tool: %w", err)
	}
	searchHistoryTool, err := NewSearchHistoryTool(cfg.Deps)
	if err != nil {
		return nil, fmt.Errorf("failed to create search_history tool: %w", err)
	}
	saveAndReconcileTool, err := NewSaveAndReconcileTool(cfg.Deps)
	if err != nil {
		return nil, fmt.Errorf("failed to create save_and_reconcile tool: %w", err)
	}
	updateProfileTool, err := NewUpdateProfileTool(cfg.Deps)
	if err != nil {
		return nil, fmt.Errorf("failed to create update_profile tool: %w", err)
	}

	// Create the LLM agent
	advisorAgent, err := llmagent.New(llmagent.Config{
		Name:        "canine_advisor",
		Description: "반려견 전담 조언자 - AI 어드바이저",
		Model:       model,
		Instruction: SystemPrompt,
		Tools: []tool.Tool{
			loadContextTool,
			searchHistoryTool,
			saveAndReconcileTool,
			updateProfileTool,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create advisor agent: %w", err)
	}

	// Create session service (in-memory for MVP)
	sessionService := session.InMemoryService()

	// Create runner
	r, err := runner.New(runner.Config{
		AppName:        "ai-pet-advisor",
		Agent:          advisorAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	return &AdvisorAgent{
		runner:         r,
		sessionService: sessionService,
		appName:        "ai-pet-advisor",
		deps:           cfg.Deps,
	}, nil
}

// ChatRequest represents an incoming chat message.
type ChatRequest struct {
	UserID    string
	DogID     string
	SessionID string
	Text      string
}

// ChatResponse represents the agent's response.
type ChatResponse struct {
	Text      string `json:"text"`
	SessionID string `json:"session_id"`
}

// Chat sends a message to the agent and returns the response.
func (a *AdvisorAgent) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Ensure ADK session exists
	sessionID := req.SessionID
	if sessionID != "" {
		// Check if the session exists in ADK's session service
		_, err := a.sessionService.Get(ctx, &session.GetRequest{
			AppName:   a.appName,
			UserID:    req.UserID,
			SessionID: sessionID,
		})
		if err != nil {
			// Session not found in ADK (e.g. server restart) - create new one
			sessionID = ""
		}
	}
	if sessionID == "" {
		resp, err := a.sessionService.Create(ctx, &session.CreateRequest{
			AppName: a.appName,
			UserID:  req.UserID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		sessionID = resp.Session.ID()
	}

	// Pre-load dog profile and memory (don't rely on model calling load_context)
	msgText := a.buildMessageWithContext(req.DogID, req.Text)
	log.Printf("[Chat] message sent to agent: %s", msgText[:min(len(msgText), 200)])
	msg := genai.NewContentFromText(msgText, "user")

	// Run agent and collect response text from all agent events
	var responseText strings.Builder
	for event, err := range a.runner.Run(ctx, req.UserID, sessionID, msg, agent.RunConfig{}) {
		if err != nil {
			return nil, fmt.Errorf("agent error: %w", err)
		}
		if event == nil || event.Content == nil {
			continue
		}

		// Log event details for debugging
		for _, part := range event.Content.Parts {
			if part.FunctionCall != nil {
				log.Printf("[agent-event] author=%s, tool_call=%s", event.Author, part.FunctionCall.Name)
			}
			if part.FunctionResponse != nil {
				log.Printf("[agent-event] author=%s, tool_response=%s", event.Author, part.FunctionResponse.Name)
			}
		}

		// Collect text from agent (not user) events
		if event.Author != "user" {
			for _, part := range event.Content.Parts {
				if part.Text != "" && !part.Thought {
					responseText.WriteString(part.Text)
				}
			}
		}
	}

	result := strings.TrimSpace(responseText.String())
	if result == "" {
		return nil, fmt.Errorf("agent returned empty response")
	}

	return &ChatResponse{
		Text:      result,
		SessionID: sessionID,
	}, nil
}

// buildMessageWithContext pre-loads the dog profile and memory, then builds
// a message that includes this context so the model doesn't need to call load_context.
func (a *AdvisorAgent) buildMessageWithContext(dogID, userText string) string {
	if dogID == "" || a.deps == nil {
		return userText
	}

	var contextParts []string

	// Load dog profile
	dog, err := a.deps.DogRepo.GetByID(dogID)
	if err != nil {
		log.Printf("[buildContext] failed to load dog: %v", err)
		return fmt.Sprintf("[dog_id: %s]\n%s", dogID, userText)
	}

	// Calculate age
	ageMonths := int(time.Since(dog.Birthday).Hours() / 24 / 30)
	ageStr := fmt.Sprintf("%d개월", ageMonths)
	if ageMonths >= 12 {
		years := ageMonths / 12
		months := ageMonths % 12
		if months > 0 {
			ageStr = fmt.Sprintf("%d년 %d개월", years, months)
		} else {
			ageStr = fmt.Sprintf("%d년", years)
		}
	}

	neuteredStr := "미중성"
	if dog.Neutered {
		neuteredStr = "중성화 완료"
	}

	contextParts = append(contextParts, fmt.Sprintf(
		"[반려견 프로필] 이름: %s | 견종: %s | 나이: %s | 몸무게: %.1fkg | 성별: %s | %s",
		dog.Name, dog.Breed, ageStr, dog.Weight, dog.Gender, neuteredStr,
	))

	// Load L2 summary (brief)
	summary, err := a.deps.MemoryRepo.GetDynamicSummary(dogID)
	if err == nil && len(summary.CategoryStatuses) > 0 {
		var statuses []string
		for _, cs := range summary.CategoryStatuses {
			if cs.Baseline != "" {
				statuses = append(statuses, fmt.Sprintf("%s: %s", cs.Category, cs.Baseline))
			}
		}
		if len(statuses) > 0 {
			contextParts = append(contextParts, fmt.Sprintf("[장기 기록] %s", strings.Join(statuses, " | ")))
		}
	}

	// Load L1 recent items (last snippet per category)
	l1Contexts, err := a.deps.MemoryManager.GetAllRecentContexts(dogID)
	if err == nil {
		var recentItems []string
		for _, c := range l1Contexts {
			if len(c.RecentItems) > 0 {
				last := c.RecentItems[len(c.RecentItems)-1]
				recentItems = append(recentItems, fmt.Sprintf("[%s] Q: %s / A: %s",
					c.Category, truncate(last.UserText, 50), truncate(last.AIText, 80)))
			}
		}
		if len(recentItems) > 0 {
			contextParts = append(contextParts, fmt.Sprintf("[최근 대화]\n%s", strings.Join(recentItems, "\n")))
		}
	}

	contextParts = append(contextParts, fmt.Sprintf("[dog_id: %s]", dogID))
	contextParts = append(contextParts, userText)

	return strings.Join(contextParts, "\n")
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// GetSessionService returns the session service for external management.
func (a *AdvisorAgent) GetSessionService() session.Service {
	return a.sessionService
}
