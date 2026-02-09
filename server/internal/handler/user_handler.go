package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wnjoon/ai-pet-advisor/server/internal/service"
)

// UserHandler handles HTTP requests for user management.
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// RegisterRoutes registers user-related routes on the given router group.
func (h *UserHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/users", h.CreateUser)
	rg.GET("/users/:user_id", h.GetUser)
}

// CreateUser handles POST /api/users
func (h *UserHandler) CreateUser(c *gin.Context) {
	user, err := h.userService.CreateUser()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// GetUser handles GET /api/users/:user_id
func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("user_id")

	user, err := h.userService.GetUser(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}
