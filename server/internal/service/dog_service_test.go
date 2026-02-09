package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
	"github.com/wnjoon/ai-pet-advisor/server/internal/testutil"
	"gorm.io/gorm"
)

// setupTestService creates a test database, registers UUID auto-generation callback,
// creates repositories, and returns a DogService instance.
func setupTestService(t *testing.T) (*DogService, *gorm.DB, string) {
	t.Helper()

	db := testutil.SetupTestDB(t)

	// Register UUID auto-generation callback for SQLite
	// (since SQLite doesn't support gen_random_uuid() like PostgreSQL)
	db.Callback().Create().Before("gorm:create").Register("set_uuid", func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		if field := tx.Statement.Schema.LookUpField("ID"); field != nil {
			idValue, _ := field.ValueOf(tx.Statement.Context, tx.Statement.ReflectValue)
			// Only set UUID if ID is empty string
			if idStr, ok := idValue.(string); ok && idStr == "" {
				_ = field.Set(tx.Statement.Context, tx.Statement.ReflectValue, uuid.NewString())
			}
		}
	})

	// Create a test user with a known UUID
	userID := uuid.NewString()
	user := &domain.User{ID: userID}
	err := db.Create(user).Error
	require.NoError(t, err, "failed to create test user")

	// Create repositories and service
	dogRepo := repository.NewDogRepository(db)
	userRepo := repository.NewUserRepository(db)
	service := NewDogService(dogRepo, userRepo)

	return service, db, userID
}

func TestCreateDog_ExactBirthday(t *testing.T) {
	service, _, userID := setupTestService(t)

	birthday := "2024-06-15"
	req := CreateDogRequest{
		UserID:   userID,
		Name:     "Max",
		Breed:    "Golden Retriever",
		Gender:   "male",
		Weight:   25.5,
		Neutered: false,
		Birthday: &birthday,
	}

	dog, err := service.CreateDog(req)

	require.NoError(t, err)
	assert.NotEmpty(t, dog.ID)
	assert.Equal(t, userID, dog.UserID)
	assert.Equal(t, "Max", dog.Name)
	assert.Equal(t, "Golden Retriever", dog.Breed)
	assert.Equal(t, "male", dog.Gender)
	assert.Equal(t, 25.5, dog.Weight)
	assert.False(t, dog.Neutered)
	assert.False(t, dog.BirthdayEstimated, "birthday should not be estimated")
	assert.Equal(t, 2024, dog.Birthday.Year())
	assert.Equal(t, time.June, dog.Birthday.Month())
	assert.Equal(t, 15, dog.Birthday.Day())
}

func TestCreateDog_AgeMonths(t *testing.T) {
	service, _, userID := setupTestService(t)

	ageMonths := 8
	req := CreateDogRequest{
		UserID:    userID,
		Name:      "Bella",
		Breed:     "Labrador",
		Gender:    "female",
		Weight:    20.0,
		Neutered:  true,
		AgeMonths: &ageMonths,
	}

	dog, err := service.CreateDog(req)

	require.NoError(t, err)
	assert.NotEmpty(t, dog.ID)
	assert.Equal(t, "Bella", dog.Name)
	assert.True(t, dog.BirthdayEstimated, "birthday should be estimated")

	// Verify that the birthday is approximately 8 months ago
	now := time.Now()
	expectedYear := now.Year()
	expectedMonth := now.Month() - 8
	if expectedMonth <= 0 {
		expectedYear--
		expectedMonth += 12
	}

	assert.Equal(t, expectedYear, dog.Birthday.Year(), "birthday year should match expected")
	assert.Equal(t, expectedMonth, dog.Birthday.Month(), "birthday month should match expected")
	assert.Equal(t, 1, dog.Birthday.Day(), "birthday day should be 1st of the month")
}

func TestCreateDog_NoBirthday(t *testing.T) {
	service, _, userID := setupTestService(t)

	req := CreateDogRequest{
		UserID:   userID,
		Name:     "Charlie",
		Breed:    "Beagle",
		Gender:   "male",
		Weight:   15.0,
		Neutered: false,
		// Neither Birthday nor AgeMonths provided
	}

	dog, err := service.CreateDog(req)

	assert.Error(t, err)
	assert.Nil(t, dog)
	assert.Contains(t, err.Error(), "either birthday or age_months must be provided")
}

func TestCreateDog_UserNotFound(t *testing.T) {
	service, _, _ := setupTestService(t)

	nonExistentUserID := uuid.NewString()
	birthday := "2024-01-01"
	req := CreateDogRequest{
		UserID:   nonExistentUserID,
		Name:     "Ghost",
		Breed:    "Husky",
		Gender:   "male",
		Weight:   30.0,
		Neutered: false,
		Birthday: &birthday,
	}

	dog, err := service.CreateDog(req)

	assert.Error(t, err)
	assert.Nil(t, dog)
	assert.Contains(t, err.Error(), "user not found")
}

func TestUpdateDog_MutableFields(t *testing.T) {
	service, _, userID := setupTestService(t)

	// Create a dog first
	birthday := "2023-01-01"
	createReq := CreateDogRequest{
		UserID:   userID,
		Name:     "Rocky",
		Breed:    "Bulldog",
		Gender:   "male",
		Weight:   25.0,
		Neutered: false,
		Birthday: &birthday,
	}
	dog, err := service.CreateDog(createReq)
	require.NoError(t, err)

	// Update mutable fields
	newWeight := 28.5
	newNeutered := true
	updateReq := UpdateDogRequest{
		Weight:   &newWeight,
		Neutered: &newNeutered,
	}

	updatedDog, err := service.UpdateDog(dog.ID, updateReq)

	require.NoError(t, err)
	assert.Equal(t, dog.ID, updatedDog.ID)
	assert.Equal(t, 28.5, updatedDog.Weight, "weight should be updated")
	assert.True(t, updatedDog.Neutered, "neutered should be updated")
	// Immutable fields should remain unchanged
	assert.Equal(t, "Rocky", updatedDog.Name)
	assert.Equal(t, "Bulldog", updatedDog.Breed)
	assert.Equal(t, "male", updatedDog.Gender)
}

func TestUpdateDog_InvalidWeight(t *testing.T) {
	service, _, userID := setupTestService(t)

	// Create a dog first
	birthday := "2023-01-01"
	createReq := CreateDogRequest{
		UserID:   userID,
		Name:     "Daisy",
		Breed:    "Poodle",
		Gender:   "female",
		Weight:   10.0,
		Neutered: false,
		Birthday: &birthday,
	}
	dog, err := service.CreateDog(createReq)
	require.NoError(t, err)

	// Try to update with invalid weight
	invalidWeight := 0.0
	updateReq := UpdateDogRequest{
		Weight: &invalidWeight,
	}

	updatedDog, err := service.UpdateDog(dog.ID, updateReq)

	assert.Error(t, err)
	assert.Nil(t, updatedDog)
	assert.Contains(t, err.Error(), "weight must be greater than 0")
}

func TestGetDogsByUser(t *testing.T) {
	service, _, userID := setupTestService(t)

	// Create two dogs for the same user
	birthday1 := "2023-01-01"
	req1 := CreateDogRequest{
		UserID:   userID,
		Name:     "Dog1",
		Breed:    "Breed1",
		Gender:   "male",
		Weight:   20.0,
		Neutered: false,
		Birthday: &birthday1,
	}
	dog1, err := service.CreateDog(req1)
	require.NoError(t, err)

	birthday2 := "2023-06-01"
	req2 := CreateDogRequest{
		UserID:   userID,
		Name:     "Dog2",
		Breed:    "Breed2",
		Gender:   "female",
		Weight:   15.0,
		Neutered: true,
		Birthday: &birthday2,
	}
	dog2, err := service.CreateDog(req2)
	require.NoError(t, err)

	// Get all dogs for the user
	dogs, err := service.GetDogsByUser(userID)

	require.NoError(t, err)
	assert.Len(t, dogs, 2, "should return 2 dogs")

	// Verify both dogs are in the list (order might vary)
	dogIDs := []string{dogs[0].ID, dogs[1].ID}
	assert.Contains(t, dogIDs, dog1.ID)
	assert.Contains(t, dogIDs, dog2.ID)
}

func TestDeleteDog(t *testing.T) {
	service, _, userID := setupTestService(t)

	// Create a dog
	birthday := "2023-01-01"
	createReq := CreateDogRequest{
		UserID:   userID,
		Name:     "ToDelete",
		Breed:    "TestBreed",
		Gender:   "male",
		Weight:   20.0,
		Neutered: false,
		Birthday: &birthday,
	}
	dog, err := service.CreateDog(createReq)
	require.NoError(t, err)

	// Verify dog exists
	retrievedDog, err := service.GetDog(dog.ID)
	require.NoError(t, err)
	assert.Equal(t, dog.ID, retrievedDog.ID)

	// Delete the dog
	err = service.DeleteDog(dog.ID)
	require.NoError(t, err)

	// Verify dog no longer exists
	deletedDog, err := service.GetDog(dog.ID)
	assert.Error(t, err)
	assert.Nil(t, deletedDog)
}

func TestUpdateDog_ProfilePhotoAndMedicalNotes(t *testing.T) {
	service, _, userID := setupTestService(t)

	// Create a dog
	birthday := "2023-01-01"
	createReq := CreateDogRequest{
		UserID:   userID,
		Name:     "PhotoDog",
		Breed:    "TestBreed",
		Gender:   "male",
		Weight:   20.0,
		Neutered: false,
		Birthday: &birthday,
	}
	dog, err := service.CreateDog(createReq)
	require.NoError(t, err)

	// Update profile photo and medical notes
	photo := "https://example.com/photo.jpg"
	notes := "Regular checkup scheduled"
	updateReq := UpdateDogRequest{
		ProfilePhoto: &photo,
		MedicalNotes: &notes,
	}

	updatedDog, err := service.UpdateDog(dog.ID, updateReq)

	require.NoError(t, err)
	assert.Equal(t, photo, updatedDog.ProfilePhoto)
	assert.Equal(t, notes, updatedDog.MedicalNotes)
}

func TestCreateDog_InvalidBirthdayFormat(t *testing.T) {
	service, _, userID := setupTestService(t)

	invalidBirthday := "15-06-2024" // Wrong format
	req := CreateDogRequest{
		UserID:   userID,
		Name:     "InvalidDog",
		Breed:    "TestBreed",
		Gender:   "male",
		Weight:   20.0,
		Neutered: false,
		Birthday: &invalidBirthday,
	}

	dog, err := service.CreateDog(req)

	assert.Error(t, err)
	assert.Nil(t, dog)
	assert.Contains(t, err.Error(), "invalid birthday format")
}
