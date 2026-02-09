package handler

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
	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
	"github.com/wnjoon/ai-pet-advisor/server/internal/service"
	"github.com/wnjoon/ai-pet-advisor/server/internal/testutil"
	"gorm.io/gorm"
)

// setupTestRouter creates a Gin router with all routes registered for testing.
// Returns the router and the user service for creating test users.
func setupTestRouter(t *testing.T) (*gin.Engine, *service.UserService) {
	t.Helper()

	// Setup test database
	db := testutil.SetupTestDB(t)

	// Register callback to auto-generate UUIDs for SQLite
	db.Callback().Create().Before("gorm:create").Register("test_set_uuid", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil {
			if field := tx.Statement.Schema.LookUpField("ID"); field != nil {
				if fieldValue, isZero := field.ValueOf(tx.Statement.Context, tx.Statement.ReflectValue); isZero || fieldValue == "" {
					field.Set(tx.Statement.Context, tx.Statement.ReflectValue, uuid.NewString())
				}
			}
		}
	})

	// Create repositories
	userRepo := repository.NewUserRepository(db)
	dogRepo := repository.NewDogRepository(db)

	// Create services
	userSvc := service.NewUserService(userRepo)
	dogSvc := service.NewDogService(dogRepo, userRepo)

	// Create handlers
	userHandler := NewUserHandler(userSvc)
	dogHandler := NewDogHandler(dogSvc)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	userHandler.RegisterRoutes(api)
	dogHandler.RegisterRoutes(api)

	return r, userSvc
}

// createTestUser creates a user via POST /api/users and returns the user ID.
func createTestUser(t *testing.T, router *gin.Engine) string {
	t.Helper()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "Failed to create test user: %s", w.Body.String())

	var user domain.User
	err := json.Unmarshal(w.Body.Bytes(), &user)
	require.NoError(t, err, "Failed to unmarshal user response")
	require.NotEmpty(t, user.ID, "User ID should not be empty")

	return user.ID
}

// createTestDog creates a dog via POST /api/dogs and returns the dog.
func createTestDog(t *testing.T, router *gin.Engine, userID, name, breed, gender string, weight float64, birthday string) domain.Dog {
	t.Helper()

	reqBody := map[string]interface{}{
		"user_id":  userID,
		"name":     name,
		"breed":    breed,
		"gender":   gender,
		"weight":   weight,
		"birthday": birthday,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/dogs", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "Failed to create test dog: %s", w.Body.String())

	var dog domain.Dog
	err := json.Unmarshal(w.Body.Bytes(), &dog)
	require.NoError(t, err, "Failed to unmarshal dog response")
	require.NotEmpty(t, dog.ID, "Dog ID should not be empty")

	return dog
}

func TestAPI_CreateDog(t *testing.T) {
	router, _ := setupTestRouter(t)
	userID := createTestUser(t, router)

	reqBody := map[string]interface{}{
		"user_id":  userID,
		"name":     "바둑이",
		"breed":    "maltese",
		"gender":   "male",
		"weight":   4.0,
		"birthday": "2024-06-15",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/dogs", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var dog domain.Dog
	err := json.Unmarshal(w.Body.Bytes(), &dog)
	require.NoError(t, err)
	assert.NotEmpty(t, dog.ID)
	assert.Equal(t, userID, dog.UserID)
	assert.Equal(t, "바둑이", dog.Name)
	assert.Equal(t, "maltese", dog.Breed)
	assert.Equal(t, "male", dog.Gender)
	assert.Equal(t, 4.0, dog.Weight)
}

func TestAPI_GetDog(t *testing.T) {
	router, _ := setupTestRouter(t)
	userID := createTestUser(t, router)
	dog := createTestDog(t, router, userID, "바둑이", "maltese", "male", 4.0, "2024-06-15")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dogs/"+dog.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var retrievedDog domain.Dog
	err := json.Unmarshal(w.Body.Bytes(), &retrievedDog)
	require.NoError(t, err)
	assert.Equal(t, dog.ID, retrievedDog.ID)
	assert.Equal(t, "바둑이", retrievedDog.Name)
	assert.Equal(t, "maltese", retrievedDog.Breed)
}

func TestAPI_GetDog_NotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	nonexistentID := uuid.NewString()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dogs/"+nonexistentID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "dog not found")
}

func TestAPI_GetDogsByUser(t *testing.T) {
	router, _ := setupTestRouter(t)
	userID := createTestUser(t, router)

	// Create two dogs
	createTestDog(t, router, userID, "바둑이", "maltese", "male", 4.0, "2024-06-15")
	createTestDog(t, router, userID, "초코", "poodle", "female", 3.5, "2023-01-10")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users/"+userID+"/dogs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var dogs []domain.Dog
	err := json.Unmarshal(w.Body.Bytes(), &dogs)
	require.NoError(t, err)
	assert.Len(t, dogs, 2)

	// Verify dog names
	names := []string{dogs[0].Name, dogs[1].Name}
	assert.Contains(t, names, "바둑이")
	assert.Contains(t, names, "초코")
}

func TestAPI_UpdateDog(t *testing.T) {
	router, _ := setupTestRouter(t)
	userID := createTestUser(t, router)
	dog := createTestDog(t, router, userID, "바둑이", "maltese", "male", 4.0, "2024-06-15")

	// Update weight
	newWeight := 4.5
	updateBody := map[string]interface{}{
		"weight": newWeight,
	}
	bodyBytes, _ := json.Marshal(updateBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/dogs/"+dog.ID, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updatedDog domain.Dog
	err := json.Unmarshal(w.Body.Bytes(), &updatedDog)
	require.NoError(t, err)
	assert.Equal(t, dog.ID, updatedDog.ID)
	assert.Equal(t, newWeight, updatedDog.Weight)
	assert.Equal(t, "바둑이", updatedDog.Name) // Name should not change
}

func TestAPI_DeleteDog(t *testing.T) {
	router, _ := setupTestRouter(t)
	userID := createTestUser(t, router)
	dog := createTestDog(t, router, userID, "바둑이", "maltese", "male", 4.0, "2024-06-15")

	// Delete the dog
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/dogs/"+dog.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify dog is gone
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/dogs/"+dog.ID, nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestAPI_CreateDog_ValidationError(t *testing.T) {
	router, _ := setupTestRouter(t)
	userID := createTestUser(t, router)

	// Missing required fields (no name, breed)
	reqBody := map[string]interface{}{
		"user_id": userID,
		"gender":  "male",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/dogs", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.NotEmpty(t, errResp["error"])
}
