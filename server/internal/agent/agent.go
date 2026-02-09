package agent

import (
	"context"
	"fmt"
	"strings"

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

	// Create the LLM agent
	advisorAgent, err := llmagent.New(llmagent.Config{
		Name:        "canine_advisor",
		Description: "반려견 전담 조언자 - 강형욱 페르소나 기반 AI 어드바이저",
		Model:       model,
		Instruction: SystemPrompt,
		Tools: []tool.Tool{
			loadContextTool,
			searchHistoryTool,
			saveAndReconcileTool,
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
	// Ensure session exists
	sessionID := req.SessionID
	if sessionID == "" {
		// Create new session
		resp, err := a.sessionService.Create(ctx, &session.CreateRequest{
			AppName: a.appName,
			UserID:  req.UserID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		sessionID = resp.Session.ID()
	}

	// Build user message with dog_id context
	msgText := fmt.Sprintf("[dog_id: %s]\n%s", req.DogID, req.Text)
	msg := genai.NewContentFromText(msgText, "user")

	// Run agent
	var responseText strings.Builder
	for event, err := range a.runner.Run(ctx, req.UserID, sessionID, msg, agent.RunConfig{}) {
		if err != nil {
			return nil, fmt.Errorf("agent error: %w", err)
		}
		if event == nil {
			continue
		}
		if event.Author == "canine_advisor" && event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" && !part.Thought {
					responseText.WriteString(part.Text)
				}
			}
		}
	}

	return &ChatResponse{
		Text:      responseText.String(),
		SessionID: sessionID,
	}, nil
}

// GetSessionService returns the session service for external management.
func (a *AdvisorAgent) GetSessionService() session.Service {
	return a.sessionService
}
