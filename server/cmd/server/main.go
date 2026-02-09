package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	agentpkg "github.com/wnjoon/ai-pet-advisor/server/internal/agent"
	"github.com/wnjoon/ai-pet-advisor/server/internal/config"
	"github.com/wnjoon/ai-pet-advisor/server/internal/handler"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
	"github.com/wnjoon/ai-pet-advisor/server/internal/service"
)

func main() {
	// Load .env file (ignore error if not present, e.g. in production)
	_ = godotenv.Load()

	cfg := config.Load()
	ctx := context.Background()

	// Database
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)
	dogRepo := repository.NewDogRepository(db)
	memoryRepo := repository.NewMemoryRepository(db)

	// Services
	userService := service.NewUserService(userRepo)
	dogService := service.NewDogService(dogRepo, userRepo)
	memoryManager := service.NewMemoryManager(memoryRepo)

	// Reconciler (with AI if API key is set, otherwise rule-based fallback)
	var reconciliationAI service.ReconciliationAI
	if cfg.GoogleAPIKey != "" {
		ai, err := service.NewGeminiReconciliationAI(ctx, cfg.GoogleAPIKey, cfg.GeminiModel)
		if err != nil {
			log.Printf("Warning: failed to create Gemini reconciliation AI, using fallback: %v", err)
		} else {
			reconciliationAI = ai
		}
	}
	reconciler := service.NewReconciler(memoryRepo, reconciliationAI)

	// ADK Agent (optional: only if API key is configured)
	var chatHandler *handler.ChatHandler
	if cfg.GoogleAPIKey != "" {
		advisorAgent, err := agentpkg.New(ctx, agentpkg.Config{
			GoogleAPIKey: cfg.GoogleAPIKey,
			GeminiModel:  cfg.GeminiModel,
			Deps: &agentpkg.ToolDeps{
				DogRepo:       dogRepo,
				MemoryManager: memoryManager,
				Reconciler:    reconciler,
				MemoryRepo:    memoryRepo,
			},
		})
		if err != nil {
			log.Printf("Warning: failed to create ADK agent: %v", err)
		} else {
			chatHandler = handler.NewChatHandler(advisorAgent)
		}
	}

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
	if chatHandler != nil {
		chatHandler.RegisterRoutes(api)
	}

	// Start server
	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
