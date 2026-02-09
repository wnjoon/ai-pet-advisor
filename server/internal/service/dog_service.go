package service

import (
	"errors"
	"time"

	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
)

// DogService handles business logic for dog management.
type DogService struct {
	dogRepo  *repository.DogRepository
	userRepo *repository.UserRepository
}

// NewDogService creates a new DogService.
func NewDogService(dogRepo *repository.DogRepository, userRepo *repository.UserRepository) *DogService {
	return &DogService{dogRepo: dogRepo, userRepo: userRepo}
}

// CreateDogRequest represents the input for registering a new dog.
type CreateDogRequest struct {
	UserID   string  `json:"user_id" binding:"required"`
	Name     string  `json:"name" binding:"required"`
	Breed    string  `json:"breed" binding:"required"`
	Gender   string  `json:"gender" binding:"required,oneof=male female"`
	Weight   float64 `json:"weight" binding:"required,gt=0"`
	Neutered bool    `json:"neutered"`

	// Either Birthday or AgeMonths must be provided
	Birthday  *string `json:"birthday,omitempty"`   // ISO8601 date "2025-06-01"
	AgeMonths *int    `json:"age_months,omitempty"` // Approximate age in months
}

// UpdateDogRequest represents the input for updating mutable dog fields.
type UpdateDogRequest struct {
	Weight       *float64 `json:"weight,omitempty"`
	Neutered     *bool    `json:"neutered,omitempty"`
	ProfilePhoto *string  `json:"profile_photo,omitempty"`
	MedicalNotes *string  `json:"medical_notes,omitempty"`
}

// CreateDog registers a new dog with birthday estimation logic.
func (s *DogService) CreateDog(req CreateDogRequest) (*domain.Dog, error) {
	// Validate user exists
	if _, err := s.userRepo.GetByID(req.UserID); err != nil {
		return nil, errors.New("user not found: register user first")
	}

	// Validate: either birthday or age_months must be provided
	if req.Birthday == nil && req.AgeMonths == nil {
		return nil, errors.New("either birthday or age_months must be provided")
	}

	var birthday time.Time
	var estimated bool

	if req.Birthday != nil {
		// Exact birthday
		parsed, err := time.Parse("2006-01-02", *req.Birthday)
		if err != nil {
			return nil, errors.New("invalid birthday format, expected YYYY-MM-DD")
		}
		birthday = parsed
		estimated = false
	} else {
		// Estimated: N months ago, 1st of that month
		now := time.Now()
		estimated = true
		birthday = time.Date(
			now.Year(), now.Month()-time.Month(*req.AgeMonths), 1,
			0, 0, 0, 0, now.Location(),
		)
	}

	dog := &domain.Dog{
		UserID:            req.UserID,
		Name:              req.Name,
		Breed:             req.Breed,
		Birthday:          birthday,
		BirthdayEstimated: estimated,
		Gender:            req.Gender,
		Weight:            req.Weight,
		Neutered:          req.Neutered,
	}

	if err := s.dogRepo.Create(dog); err != nil {
		return nil, err
	}

	return dog, nil
}

// GetDog retrieves a single dog by ID.
func (s *DogService) GetDog(id string) (*domain.Dog, error) {
	return s.dogRepo.GetByID(id)
}

// GetDogsByUser retrieves all dogs for a user.
func (s *DogService) GetDogsByUser(userID string) ([]domain.Dog, error) {
	return s.dogRepo.GetByUserID(userID)
}

// UpdateDog updates mutable fields of a dog.
func (s *DogService) UpdateDog(id string, req UpdateDogRequest) (*domain.Dog, error) {
	dog, err := s.dogRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if req.Weight != nil {
		if *req.Weight <= 0 {
			return nil, errors.New("weight must be greater than 0")
		}
		dog.Weight = *req.Weight
	}
	if req.Neutered != nil {
		dog.Neutered = *req.Neutered
	}
	if req.ProfilePhoto != nil {
		dog.ProfilePhoto = *req.ProfilePhoto
	}
	if req.MedicalNotes != nil {
		dog.MedicalNotes = *req.MedicalNotes
	}

	if err := s.dogRepo.Update(dog); err != nil {
		return nil, err
	}

	return dog, nil
}

// DeleteDog removes a dog and all associated data.
func (s *DogService) DeleteDog(id string) error {
	return s.dogRepo.Delete(id)
}
