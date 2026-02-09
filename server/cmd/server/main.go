package main

import (
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/wnjoon/ai-pet-advisor/server/internal/config"
	"github.com/wnjoon/ai-pet-advisor/server/internal/handler"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
	"github.com/wnjoon/ai-pet-advisor/server/internal/service"
)

func main() {
	// Load .env file (ignore error if not present, e.g. in production)
	_ = godotenv.Load()

	cfg := config.Load()

	// Database
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)
	dogRepo := repository.NewDogRepository(db)

	// Services
	userService := service.NewUserService(userRepo)
	dogService := service.NewDogService(dogRepo, userRepo)

	// Handlers
	userHandler := handler.NewUserHandler(userService)
	dogHandler := handler.NewDogHandler(dogService)

	// Router
	gin.SetMode(cfg.GinMode)
	r := gin.New()

	// Middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.WebviewBaseURL},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// API routes
	api := r.Group("/api")
	userHandler.RegisterRoutes(api)
	dogHandler.RegisterRoutes(api)

	// Start server
	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
