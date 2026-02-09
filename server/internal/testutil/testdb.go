package testutil

import (
	"testing"

	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupTestDB creates and returns a fresh SQLite in-memory database for testing.
// PostgreSQL-specific tables (User, Dog) are created with raw SQL to avoid
// gen_random_uuid() syntax errors. Tests must set UUIDs manually.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create tables that use gen_random_uuid() with raw SQL (SQLite-compatible)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS platform_accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT,
			platform TEXT,
			platform_id TEXT UNIQUE,
			created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS dogs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			breed TEXT NOT NULL,
			birthday DATETIME NOT NULL,
			birthday_estimated BOOLEAN DEFAULT 0,
			gender TEXT NOT NULL,
			weight REAL,
			neutered BOOLEAN DEFAULT 0,
			profile_photo TEXT,
			medical_notes TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
	}

	// AutoMigrate tables that don't have PostgreSQL-specific defaults
	err = db.AutoMigrate(
		&domain.DogCategoryContext{},
		&domain.DogDynamicSummary{},
	)
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}
