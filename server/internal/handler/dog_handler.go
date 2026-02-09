package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wnjoon/ai-pet-advisor/server/internal/service"
)

// DogHandler handles HTTP requests for dog management.
type DogHandler struct {
	dogService *service.DogService
}

// NewDogHandler creates a new DogHandler.
func NewDogHandler(dogService *service.DogService) *DogHandler {
	return &DogHandler{dogService: dogService}
}

// RegisterRoutes registers dog-related routes on the given router group.
func (h *DogHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/dogs", h.CreateDog)
	rg.GET("/dogs/:id", h.GetDog)
	rg.GET("/users/:user_id/dogs", h.GetDogsByUser)
	rg.PATCH("/dogs/:id", h.UpdateDog)
	rg.DELETE("/dogs/:id", h.DeleteDog)
}

// CreateDog handles POST /api/dogs
func (h *DogHandler) CreateDog(c *gin.Context) {
	var req service.CreateDogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dog, err := h.dogService.CreateDog(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dog)
}

// GetDog handles GET /api/dogs/:id
func (h *DogHandler) GetDog(c *gin.Context) {
	id := c.Param("id")

	dog, err := h.dogService.GetDog(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dog not found"})
		return
	}

	c.JSON(http.StatusOK, dog)
}

// GetDogsByUser handles GET /api/users/:user_id/dogs
func (h *DogHandler) GetDogsByUser(c *gin.Context) {
	userID := c.Param("user_id")

	dogs, err := h.dogService.GetDogsByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dogs)
}

// UpdateDog handles PATCH /api/dogs/:id
func (h *DogHandler) UpdateDog(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateDogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dog, err := h.dogService.UpdateDog(id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dog)
}

// DeleteDog handles DELETE /api/dogs/:id
func (h *DogHandler) DeleteDog(c *gin.Context) {
	id := c.Param("id")

	if err := h.dogService.DeleteDog(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
