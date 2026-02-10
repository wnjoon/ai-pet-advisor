package kakao

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	agentpkg "github.com/wnjoon/ai-pet-advisor/server/internal/agent"
	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"github.com/wnjoon/ai-pet-advisor/server/internal/service"
)

type HandlerDeps struct {
	Agent          *agentpkg.AdvisorAgent
	SessionManager *service.SessionManager
	UserService    *service.UserService
	DogService     *service.DogService
	APIKey         string // KAKAO_SKILL_API_KEY for auth
}

type Handler struct {
	deps *HandlerDeps
}

func NewHandler(deps *HandlerDeps) *Handler {
	return &Handler{deps: deps}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	kakao := r.Group("/kakao")
	if h.deps.APIKey != "" {
		kakao.Use(h.authMiddleware())
	}
	kakao.POST("/skill", h.HandleSkill)
}

func (h *Handler) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("x-api-key")
		if apiKey != h.deps.APIKey {
			c.JSON(http.StatusUnauthorized, NewErrorResponse("Unauthorized"))
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) HandleSkill(c *gin.Context) {
	var req KakaoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("Invalid request"))
		return
	}

	log.Printf("[HandleSkill] action=%q utterance=%q callbackUrl=%q",
		req.Action.Name, req.UserRequest.Utterance, req.UserRequest.CallbackURL)

	kakaoUserID := req.UserRequest.User.ID
	user, err := h.deps.UserService.GetOrCreateByPlatform("kakao", kakaoUserID)
	if err != nil {
		log.Printf("Failed to get or create user: %v", err)
		c.JSON(http.StatusInternalServerError, NewErrorResponse("User service error"))
		return
	}

	actionName := req.Action.Name
	switch actionName {
	case "action_chat", "":
		h.handleChat(c, &req, user)
	case "action_menu":
		h.handleMenu(c)
	case "action_switch_dog":
		h.handleSwitchDog(c, &req, user)
	default:
		h.handleChat(c, &req, user)
	}
}

func (h *Handler) handleChat(c *gin.Context, req *KakaoRequest, user *domain.User) {
	callbackURL := req.UserRequest.CallbackURL
	log.Printf("[handleChat] callbackURL=%q, utterance=%q", callbackURL, req.UserRequest.Utterance)

	// Get active dog for this user's session
	sess := h.deps.SessionManager.GetActiveSession(user.ID, "kakao")
	var dogID string
	if sess != nil {
		dogID = sess.DogID
	} else {
		// No active session - try to get user's first dog
		dogs, err := h.deps.DogService.GetDogsByUser(user.ID)
		if err != nil || len(dogs) == 0 {
			// No dogs registered
			resp := NewSimpleTextResponse("등록된 반려견이 없습니다. 먼저 반려견을 등록해주세요.")
			if callbackURL != "" {
				c.JSON(http.StatusOK, NewCallbackAck("확인 중입니다..."))
				h.sendCallback(callbackURL, resp)
			} else {
				c.JSON(http.StatusOK, resp)
			}
			return
		}
		dogID = dogs[0].ID
		sess = h.deps.SessionManager.CreateSession(user.ID, dogID, "kakao")
	}

	// Update session activity
	h.deps.SessionManager.UpdateLastActive(sess.SessionID)

	if callbackURL == "" {
		// No callback URL: KakaoTalk requires response within 5s, but AI+tools need more time.
		// This path is only hit when callback is not enabled in OpenBuilder.
		// Return a guide message instead of attempting a timeout-prone AI call.
		c.JSON(http.StatusOK, NewSimpleTextResponse("응답 준비에 시간이 필요합니다. 잠시 후 다시 시도해주세요.\n\n(운영자: OpenBuilder에서 '콜백 사용'을 활성화해주세요)"))
		return
	}

	// Async mode: immediately respond with callback acknowledgment
	c.JSON(http.StatusOK, NewCallbackAck("잠시만 기다려주세요, 답변을 준비하고 있어요 🐾"))

	// Run AI agent asynchronously
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in agent goroutine: %v", r)
				h.sendCallback(callbackURL, NewSimpleTextResponse("죄송합니다. 내부 오류가 발생했습니다. 다시 시도해주세요."))
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second) // 55s to stay within 60s callback limit
		defer cancel()

		log.Printf("[goroutine] starting Agent.Chat for user=%s dog=%s", user.ID, dogID)
		resp, err := h.deps.Agent.Chat(ctx, agentpkg.ChatRequest{
			UserID:    user.ID,
			DogID:     dogID,
			SessionID: sess.SessionID,
			Text:      req.UserRequest.Utterance,
		})

		if err != nil {
			log.Printf("Agent chat error: %v", err)
			h.sendCallback(callbackURL, NewSimpleTextResponse("죄송합니다. 응답 생성 중 오류가 발생했습니다. 다시 시도해주세요."))
			return
		}
		log.Printf("[goroutine] Agent.Chat completed, response length=%d", len(resp.Text))

		h.sendCallback(callbackURL, formatByUrgency(resp.Text))
	}()
}

func (h *Handler) handleMenu(c *gin.Context) {
	items := []ListCardItem{
		{Title: "반려견 등록", Description: "새 반려견을 등록합니다", Action: "message", MessageText: "반려견 등록"},
		{Title: "반려견 전환", Description: "다른 반려견으로 전환합니다", Action: "message", MessageText: "반려견 전환"},
		{Title: "리포트 보기", Description: "반려견 건강 리포트를 확인합니다", Action: "message", MessageText: "리포트"},
	}
	c.JSON(http.StatusOK, NewListCardResponse("메뉴", items))
}

func (h *Handler) handleSwitchDog(c *gin.Context, req *KakaoRequest, user *domain.User) {
	// Check if a specific dog was selected (via action params)
	if dogID, ok := req.Action.Params["dog_id"]; ok && dogID != "" {
		sess := h.deps.SessionManager.GetActiveSession(user.ID, "kakao")
		if sess != nil {
			h.deps.SessionManager.SwitchDog(sess.SessionID, dogID)
			// Load dog name for confirmation
			dog, err := h.deps.DogService.GetDog(dogID)
			if err == nil {
				c.JSON(http.StatusOK, NewSimpleTextResponse(fmt.Sprintf("%s(으)로 전환했습니다!", dog.Name)))
				return
			}
		}
		c.JSON(http.StatusOK, NewSimpleTextResponse("반려견을 전환했습니다!"))
		return
	}

	// No specific dog selected - show dog list
	dogs, err := h.deps.DogService.GetDogsByUser(user.ID)
	if err != nil || len(dogs) == 0 {
		c.JSON(http.StatusOK, NewSimpleTextResponse("등록된 반려견이 없습니다."))
		return
	}

	var items []ListCardItem
	for _, dog := range dogs {
		items = append(items, ListCardItem{
			Title:       dog.Name,
			Description: fmt.Sprintf("%s · %.1fkg", dog.Breed, dog.Weight),
			Action:      "message",
			MessageText: fmt.Sprintf("반려견 전환 %s", dog.ID),
		})
	}
	c.JSON(http.StatusOK, NewListCardResponse("반려견 선택", items))
}

// formatByUrgency detects urgency level from agent response text and formats accordingly.
// L3: adds hospital search quick reply
// L4: adds emergency hospital search quick reply
// L1/L2: returns plain text
func formatByUrgency(text string) *KakaoResponse {
	lower := strings.ToLower(text)

	// L4 keywords: seizure, bleeding, unconscious, emergency
	l4Keywords := []string{"응급", "경련", "출혈", "의식", "지금 바로"}
	for _, kw := range l4Keywords {
		if strings.Contains(lower, kw) {
			return NewSimpleTextWithQuickReplies(text, []QuickReply{
				{
					Label:      "24시 동물병원 찾기",
					Action:     "webLink",
					WebLinkURL: "https://m.search.naver.com/search.naver?query=24시+동물병원+근처",
				},
			})
		}
	}

	// L3 keywords: hospital visit recommended
	l3Keywords := []string{"병원", "진료", "수의사", "검사를 받"}
	for _, kw := range l3Keywords {
		if strings.Contains(lower, kw) {
			return NewSimpleTextWithQuickReplies(text, []QuickReply{
				{
					Label:      "동물병원 찾기",
					Action:     "webLink",
					WebLinkURL: "https://m.search.naver.com/search.naver?query=동물병원+근처",
				},
			})
		}
	}

	return NewSimpleTextResponse(text)
}

func (h *Handler) sendCallback(callbackURL string, resp *KakaoResponse) {
	body, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal callback response: %v", err)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// Try once, retry once on failure
	for attempt := 0; attempt < 2; attempt++ {
		httpResp, err := client.Post(callbackURL, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("Callback attempt %d failed: %v", attempt+1, err)
			continue
		}
		io.Copy(io.Discard, httpResp.Body)
		httpResp.Body.Close()

		if httpResp.StatusCode == http.StatusOK {
			return
		}
		log.Printf("Callback attempt %d returned status %d", attempt+1, httpResp.StatusCode)
	}
	log.Printf("All callback attempts failed for URL: %s", callbackURL)
}
