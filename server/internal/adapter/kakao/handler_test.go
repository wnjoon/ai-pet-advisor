package kakao

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
	"github.com/wnjoon/ai-pet-advisor/server/internal/service"
	"github.com/wnjoon/ai-pet-advisor/server/internal/testutil"
)

func setupTestHandler(t *testing.T) (*Handler, *gorm.DB) {
	db := testutil.SetupTestDB(t)

	// Register UUID callback
	db.Callback().Create().Before("gorm:create").Register("test_set_uuid", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil {
			if field := tx.Statement.Schema.LookUpField("ID"); field != nil {
				if fieldValue, isZero := field.ValueOf(tx.Statement.Context, tx.Statement.ReflectValue); isZero || fieldValue == "" {
					_ = field.Set(tx.Statement.Context, tx.Statement.ReflectValue, uuid.NewString())
				}
			}
		}
	})

	userRepo := repository.NewUserRepository(db)
	dogRepo := repository.NewDogRepository(db)
	userSvc := service.NewUserService(userRepo)
	dogSvc := service.NewDogService(dogRepo, userRepo)
	sessionMgr := service.NewSessionManager(30, nil, nil)

	h := NewHandler(&HandlerDeps{
		Agent:          nil,
		SessionManager: sessionMgr,
		UserService:    userSvc,
		DogService:     dogSvc,
		APIKey:         "test-api-key",
	})
	return h, db
}

func TestFormatByUrgency_L1_PlainText(t *testing.T) {
	text := "일반적인 건강 상태입니다. 걱정하지 마세요."
	resp := formatByUrgency(text)

	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.Outputs, 1)
	require.NotNil(t, resp.Template.Outputs[0].SimpleText)
	assert.Equal(t, text, resp.Template.Outputs[0].SimpleText.Text)
	assert.Nil(t, resp.Template.QuickReplies)
}

func TestFormatByUrgency_L3_HospitalRecommend(t *testing.T) {
	text := "병원 방문을 권장합니다."
	resp := formatByUrgency(text)

	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.Outputs, 1)
	require.NotNil(t, resp.Template.Outputs[0].SimpleText)
	assert.Equal(t, text, resp.Template.Outputs[0].SimpleText.Text)
	require.Len(t, resp.Template.QuickReplies, 1)
	assert.Equal(t, "동물병원 찾기", resp.Template.QuickReplies[0].Label)
	assert.Equal(t, "webLink", resp.Template.QuickReplies[0].Action)
	assert.Contains(t, resp.Template.QuickReplies[0].WebLinkURL, "동물병원+근처")
}

func TestFormatByUrgency_L3_VetRecommend(t *testing.T) {
	text := "수의사의 진료가 필요합니다."
	resp := formatByUrgency(text)

	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.QuickReplies, 1)
	assert.Equal(t, "동물병원 찾기", resp.Template.QuickReplies[0].Label)
	assert.Equal(t, "webLink", resp.Template.QuickReplies[0].Action)
}

func TestFormatByUrgency_L4_Emergency(t *testing.T) {
	text := "응급 상황입니다. 지금 바로 병원에 가세요!"
	resp := formatByUrgency(text)

	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.Outputs, 1)
	require.NotNil(t, resp.Template.Outputs[0].SimpleText)
	assert.Equal(t, text, resp.Template.Outputs[0].SimpleText.Text)
	require.Len(t, resp.Template.QuickReplies, 1)
	assert.Equal(t, "24시 동물병원 찾기", resp.Template.QuickReplies[0].Label)
	assert.Equal(t, "webLink", resp.Template.QuickReplies[0].Action)
	assert.Contains(t, resp.Template.QuickReplies[0].WebLinkURL, "24시+동물병원+근처")
}

func TestFormatByUrgency_L4_Seizure(t *testing.T) {
	text := "경련이 발생했다면 즉시 응급실로 가세요."
	resp := formatByUrgency(text)

	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.QuickReplies, 1)
	assert.Equal(t, "24시 동물병원 찾기", resp.Template.QuickReplies[0].Label)
	assert.Equal(t, "webLink", resp.Template.QuickReplies[0].Action)
}

func TestHandleSkill_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	h.RegisterRoutes(r)

	invalidJSON := []byte("{invalid json")
	c.Request = httptest.NewRequest("POST", "/kakao/skill", bytes.NewReader(invalidJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("x-api-key", "test-api-key")

	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp KakaoResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "2.0", resp.Version)
}

func TestHandleSkill_Menu(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	h.RegisterRoutes(r)

	kakaoReq := KakaoRequest{
		Action: KakaoAction{
			Name: "action_menu",
		},
		UserRequest: KakaoUserRequest{
			User: KakaoUser{
				ID: "test-kakao-user",
			},
		},
	}

	body, _ := json.Marshal(kakaoReq)
	c.Request = httptest.NewRequest("POST", "/kakao/skill", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("x-api-key", "test-api-key")

	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp KakaoResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "2.0", resp.Version)
	require.NotNil(t, resp.Template)
	require.Len(t, resp.Template.Outputs, 1)
	require.NotNil(t, resp.Template.Outputs[0].ListCard)
	assert.Equal(t, "메뉴", resp.Template.Outputs[0].ListCard.Header.Title)
	assert.Greater(t, len(resp.Template.Outputs[0].ListCard.Items), 0)
}

func TestAuthMiddleware_ValidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	h.RegisterRoutes(r)

	kakaoReq := KakaoRequest{
		Action: KakaoAction{
			Name: "action_menu",
		},
		UserRequest: KakaoUserRequest{
			User: KakaoUser{
				ID: "test-kakao-user",
			},
		},
	}

	body, _ := json.Marshal(kakaoReq)
	c.Request = httptest.NewRequest("POST", "/kakao/skill", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("x-api-key", "test-api-key")

	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	h.RegisterRoutes(r)

	kakaoReq := KakaoRequest{
		Action: KakaoAction{
			Name: "action_menu",
		},
		UserRequest: KakaoUserRequest{
			User: KakaoUser{
				ID: "test-kakao-user",
			},
		},
	}

	body, _ := json.Marshal(kakaoReq)
	c.Request = httptest.NewRequest("POST", "/kakao/skill", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("x-api-key", "wrong-api-key")

	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp KakaoResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "2.0", resp.Version)
}
