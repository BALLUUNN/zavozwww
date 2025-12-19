package services

import (
	"authServ/internal/domain/entities"
	repositories "authServ/internal/repositories/postgres"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// UserProfilesRepository определяет интерфейс для репозитория профилей пользователей.
type UserProfilesRepository interface {
	SaveProfile(ctx context.Context, profile *entities.UserProfile) error
	UpdateProfile(ctx context.Context, profile *entities.UserProfile) error
	GetProfileByUserID(ctx context.Context, userID uuid.UUID) (*entities.UserProfile, error)
	SearchProfilesByUsername(ctx context.Context, query string) ([]entities.UserProfile, error)
}

// ProfileReq определяет структуру данных для входящего запроса на сохранение профиля.
type ProfileReq struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Age       int    `json:"age"`
	Info      string `json:"info"`
	City      string `json:"city"`
}

// ProfileService определяет интерфейс для сервиса профилей.
type ProfileService interface {
	SaveProfile(ctx context.Context, inputProfile ProfileReq, usersId uuid.UUID) error
	GetUserProfile(ctx context.Context, userID uuid.UUID) (*entities.UserProfile, error)
	SearchProfiles(ctx context.Context, query string) ([]entities.UserProfile, error)
}

// profileService реализует интерфейс ProfileService.
type profileService struct {
	profileRepo UserProfilesRepository
	userRepo    repositories.UsersRepository
}

// NewProfileService создает новый экземпляр сервиса профилей.
func NewProfileService(profileRepo UserProfilesRepository, userRepo repositories.UsersRepository) ProfileService {
	return &profileService{
		profileRepo: profileRepo,
		userRepo:    userRepo,
	}
}

// SaveProfile сохраняет или обновляет профиль пользователя.
func (s *profileService) SaveProfile(ctx context.Context, inputProfile ProfileReq, usersId uuid.UUID) error {
	const op = "services.profileService.SaveProfile"

	user, err := s.userRepo.GetUserByID(ctx, usersId)
	if err != nil {
		return fmt.Errorf("%s: failed to get user: %w", op, err)
	}

	username := user.Username

	userProfile, err := entities.NewUserProfile(usersId, username, inputProfile.FirstName, inputProfile.LastName, inputProfile.Age, inputProfile.Info, inputProfile.City)
	if err != nil {
		return fmt.Errorf("%s: failed to create user profile entity: %w", op, err)
	}

	err = s.profileRepo.UpdateProfile(ctx, userProfile)
	if err != nil {
		_, getErr := s.profileRepo.GetProfileByUserID(ctx, usersId)
		if getErr != nil {
			if createErr := s.profileRepo.SaveProfile(ctx, userProfile); createErr != nil {
				return fmt.Errorf("%s: failed to create profile: %w", op, createErr)
			}
			return nil
		}

		return fmt.Errorf("%s: failed to update profile: %w", op, err)
	}

	return nil
}

func (s *profileService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*entities.UserProfile, error) {
	const op = "services.profileService.GetUserProfile"
	profile, err := s.profileRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserProfileNotFound) {
			user, userErr := s.userRepo.GetUserByID(ctx, userID)
			if userErr != nil {
				return nil, fmt.Errorf("%s: failed to get user for auto-creation: %w", op, userErr)
			}

			newProfile, createErr := entities.NewUserProfile(userID, user.Username, "Unknown", "Unknown", 18, "", "")
			if createErr != nil {
				return nil, fmt.Errorf("%s: failed to create default profile entity: %w", op, createErr)
			}

			if saveErr := s.profileRepo.SaveProfile(ctx, newProfile); saveErr != nil {
				return nil, fmt.Errorf("%s: failed to save default profile: %w", op, saveErr)
			}

			return newProfile, nil
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if profile.Username == "" {
		user, userErr := s.userRepo.GetUserByID(ctx, userID)
		if userErr == nil {
			profile.Username = user.Username
			_ = s.profileRepo.UpdateProfile(ctx, profile)
		}
	}

	return profile, nil
}

func (s *profileService) SearchProfiles(ctx context.Context, query string) ([]entities.UserProfile, error) {
	const op = "services.profileService.SearchProfiles"
	profiles, err := s.profileRepo.SearchProfilesByUsername(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return profiles, nil
}
