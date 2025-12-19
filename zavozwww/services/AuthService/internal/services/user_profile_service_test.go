package services_test

import (
	"authServ/internal/domain/entities"
	repositories "authServ/internal/repositories/postgres"
	"authServ/internal/services"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockUserProfilesRepository struct {
	mock.Mock
}

func (m *MockUserProfilesRepository) SaveProfile(ctx context.Context, profile *entities.UserProfile) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

func (m *MockUserProfilesRepository) UpdateProfile(ctx context.Context, profile *entities.UserProfile) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

func (m *MockUserProfilesRepository) GetProfileByUserID(ctx context.Context, userID uuid.UUID) (*entities.UserProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.UserProfile), args.Error(1)
}

func (m *MockUserProfilesRepository) DeleteProfile(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserProfilesRepository) SearchProfilesByUsername(ctx context.Context, query string) ([]entities.UserProfile, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entities.UserProfile), args.Error(1)
}

func TestProfileService_SaveProfile(t *testing.T) {
	userID := uuid.New()
	validReq := services.ProfileReq{
		FirstName: "Ivan",
		LastName:  "Ivanov",
		Age:       25,
		Info:      "Developer",
		City:      "Moscow",
		Username:  "ivanov",
	}

	t.Run("Успешное обновление существующего профиля", func(t *testing.T) {
		mockRepo := new(MockUserProfilesRepository)
		mockUserRepo := new(MockUsersRepository)
		service := services.NewProfileService(mockRepo, mockUserRepo)
		ctx := context.Background()

		// Мокаем получение пользователя для извлечения username
		mockUserRepo.On("GetUserByID", ctx, userID).Return(&entities.User{
			ID:       userID,
			Username: "ivanov",
		}, nil)

		mockRepo.On("UpdateProfile", ctx, mock.MatchedBy(func(p *entities.UserProfile) bool {
			return p.UserId == userID && p.FirstName == validReq.FirstName && p.Username == "ivanov"
		})).Return(nil)

		err := service.SaveProfile(ctx, validReq, userID)

		require.NoError(t, err, "Метод не должен возвращать ошибку при успешном обновлении")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Успешное создание нового профиля (Upsert)", func(t *testing.T) {
		mockRepo := new(MockUserProfilesRepository)
		mockUserRepo := new(MockUsersRepository)
		service := services.NewProfileService(mockRepo, mockUserRepo)
		ctx := context.Background()

		// Мокаем получение пользователя для извлечения username
		mockUserRepo.On("GetUserByID", ctx, userID).Return(&entities.User{
			ID:       userID,
			Username: "ivanov",
		}, nil)

		mockRepo.On("UpdateProfile", ctx, mock.Anything).Return(errors.New("user profile not found"))
		mockRepo.On("GetProfileByUserID", ctx, userID).Return(nil, errors.New("user profile not found"))

		mockRepo.On("SaveProfile", ctx, mock.MatchedBy(func(p *entities.UserProfile) bool {
			return p.UserId == userID && p.FirstName == validReq.FirstName && p.Username == "ivanov"
		})).Return(nil)

		err := service.SaveProfile(ctx, validReq, userID)

		require.NoError(t, err, "Метод должен успешно создать профиль, если он не найден для обновления")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Ошибка базы данных при обновлении (не 'not found')", func(t *testing.T) {
		mockRepo := new(MockUserProfilesRepository)
		mockUserRepo := new(MockUsersRepository)
		service := services.NewProfileService(mockRepo, mockUserRepo)
		ctx := context.Background()

		// Мокаем получение пользователя для извлечения username
		mockUserRepo.On("GetUserByID", ctx, userID).Return(&entities.User{
			ID:       userID,
			Username: "ivanov",
		}, nil)

		mockRepo.On("UpdateProfile", ctx, mock.Anything).Return(errors.New("db connection error"))
		mockRepo.On("GetProfileByUserID", ctx, userID).Return(&entities.UserProfile{}, nil)

		err := service.SaveProfile(ctx, validReq, userID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update profile")
		assert.Contains(t, err.Error(), "db connection error")
	})

	t.Run("Ошибка базы данных при создании (после неудачного обновления)", func(t *testing.T) {
		mockRepo := new(MockUserProfilesRepository)
		mockUserRepo := new(MockUsersRepository)
		service := services.NewProfileService(mockRepo, mockUserRepo)
		ctx := context.Background()

		mockUserRepo.On("GetUserByID", ctx, userID).Return(&entities.User{
			ID:       userID,
			Username: "ivanov",
		}, nil)

		mockRepo.On("UpdateProfile", ctx, mock.Anything).Return(errors.New("user profile not found"))
		mockRepo.On("GetProfileByUserID", ctx, userID).Return(nil, errors.New("user profile not found"))

		saveErr := errors.New("duplicate key")
		mockRepo.On("SaveProfile", ctx, mock.Anything).Return(saveErr)

		err := service.SaveProfile(ctx, validReq, userID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create profile", "Ошибка должна указывать на сбой создания")
		assert.Contains(t, err.Error(), "duplicate key")
	})
}

func TestProfileService_GetUserProfile(t *testing.T) {
	userID := uuid.New()
	expectedProfile := &entities.UserProfile{
		UserId:    userID,
		FirstName: "Ivan",
		LastName:  "Ivanov",
		Age:       25,
		Info:      "Developer",
		City:      "Moscow",
		Username:  "ivanov",
	}

	t.Run("Успешное получение профиля", func(t *testing.T) {
		mockRepo := new(MockUserProfilesRepository)
		mockUserRepo := new(MockUsersRepository)
		service := services.NewProfileService(mockRepo, mockUserRepo)
		ctx := context.Background()

		mockRepo.On("GetProfileByUserID", ctx, userID).Return(expectedProfile, nil)

		profile, err := service.GetUserProfile(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, expectedProfile, profile)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Автоматическое создание профиля, если не найден", func(t *testing.T) {
		mockRepo := new(MockUserProfilesRepository)
		mockUserRepo := new(MockUsersRepository)
		service := services.NewProfileService(mockRepo, mockUserRepo)
		ctx := context.Background()

		// 1. GetProfile returns ErrUserProfileNotFound
		mockRepo.On("GetProfileByUserID", ctx, userID).Return(nil, repositories.ErrUserProfileNotFound)

		// 2. GetUserByID returns user
		user := &entities.User{ID: userID, Username: "ivanov"}
		mockUserRepo.On("GetUserByID", ctx, userID).Return(user, nil)

		// 3. SaveProfile is called with default values
		mockRepo.On("SaveProfile", ctx, mock.MatchedBy(func(p *entities.UserProfile) bool {
			return p.UserId == userID && p.Username == "ivanov" && p.FirstName == "Unknown"
		})).Return(nil)

		profile, err := service.GetUserProfile(ctx, userID)

		require.NoError(t, err)
		assert.NotNil(t, profile)
		assert.Equal(t, "Unknown", profile.FirstName)
		assert.Equal(t, "ivanov", profile.Username)
		mockRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Ошибка при получении профиля (не 'not found')", func(t *testing.T) {
		mockRepo := new(MockUserProfilesRepository)
		mockUserRepo := new(MockUsersRepository)
		service := services.NewProfileService(mockRepo, mockUserRepo)
		ctx := context.Background()

		mockRepo.On("GetProfileByUserID", ctx, userID).Return(nil, errors.New("database error"))

		profile, err := service.GetUserProfile(ctx, userID)

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Contains(t, err.Error(), "database error")
		mockRepo.AssertExpectations(t)
	})
}

func TestProfileService_SearchProfiles(t *testing.T) {
	t.Run("Успешный поиск профилей", func(t *testing.T) {
		mockRepo := new(MockUserProfilesRepository)
		mockUserRepo := new(MockUsersRepository)
		service := services.NewProfileService(mockRepo, mockUserRepo)
		ctx := context.Background()

		query := "ivan"
		expectedProfiles := []entities.UserProfile{
			{Username: "ivanov", FirstName: "Ivan"},
			{Username: "ivan_the_terrible", FirstName: "Ivan"},
		}

		mockRepo.On("SearchProfilesByUsername", ctx, query).Return(expectedProfiles, nil)

		profiles, err := service.SearchProfiles(ctx, query)

		require.NoError(t, err)
		assert.Equal(t, expectedProfiles, profiles)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Ошибка при поиске профилей", func(t *testing.T) {
		mockRepo := new(MockUserProfilesRepository)
		mockUserRepo := new(MockUsersRepository)
		service := services.NewProfileService(mockRepo, mockUserRepo)
		ctx := context.Background()

		query := "error"
		mockRepo.On("SearchProfilesByUsername", ctx, query).Return(nil, errors.New("db error"))

		profiles, err := service.SearchProfiles(ctx, query)

		require.Error(t, err)
		assert.Nil(t, profiles)
		assert.Contains(t, err.Error(), "db error")
		mockRepo.AssertExpectations(t)
	})
}
