package repository

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
)

// NewDB creates a new GORM DB connection and runs AutoMigrate.
func NewDB(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("Connected to database")

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

func runMigrations(db *gorm.DB) error {
	log.Println("Running auto migrations...")

	if err := db.AutoMigrate(
		&domain.User{},
		&domain.PlatformAccount{},
		&domain.Dog{},
		&domain.DogCategoryContext{},
		&domain.DogDynamicSummary{},
	); err != nil {
		return err
	}

	log.Println("Auto migrations completed")
	return nil
}
