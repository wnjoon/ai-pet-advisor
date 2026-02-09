package service

import (
	"github.com/wnjoon/ai-pet-advisor/server/internal/domain"
	"github.com/wnjoon/ai-pet-advisor/server/internal/repository"
)

// UserService handles business logic for user management.
type UserService struct {
	userRepo *repository.UserRepository
}

// NewUserService creates a new UserService.
func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

// CreateUser registers a new user.
func (s *UserService) CreateUser() (*domain.User, error) {
	user := &domain.User{}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetUser retrieves a user by ID.
func (s *UserService) GetUser(id string) (*domain.User, error) {
	return s.userRepo.GetByID(id)
}

// GetOrCreateByPlatform finds or creates a user by platform credentials.
func (s *UserService) GetOrCreateByPlatform(platform, platformID string) (*domain.User, error) {
	return s.userRepo.GetOrCreateByPlatform(platform, platformID)
}
