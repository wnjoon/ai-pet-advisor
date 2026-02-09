package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	agentpkg "github.com/wnjoon/ai-pet-advisor/server/internal/agent"
)

// ChatHandler handles HTTP requests for AI chat.
type ChatHandler struct {
	agent *agentpkg.AdvisorAgent
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(agent *agentpkg.AdvisorAgent) *ChatHandler {
	return &ChatHandler{agent: agent}
}

// RegisterRoutes registers chat-related routes on the given router group.
func (h *ChatHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/chat", h.Chat)
}

// ChatRequest is the JSON body for POST /api/chat.
type ChatRequest struct {
	UserID    string `json:"user_id" binding:"required"`
	DogID     string `json:"dog_id" binding:"required"`
	SessionID string `json:"session_id,omitempty"`
	Text      string `json:"text" binding:"required"`
}

// Chat handles POST /api/chat
func (h *ChatHandler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.agent.Chat(c.Request.Context(), agentpkg.ChatRequest{
		UserID:    req.UserID,
		DogID:     req.DogID,
		SessionID: req.SessionID,
		Text:      req.Text,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
