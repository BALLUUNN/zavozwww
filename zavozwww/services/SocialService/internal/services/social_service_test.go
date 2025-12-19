package services_test

import (
	"context"
	"errors"
	"testing"

	"social_service/internal/domain"
	"social_service/internal/services"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockSocialRepository struct {
	mock.Mock
}

func (m *MockSocialRepository) CreateProfile(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockSocialRepository) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.SocialProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SocialProfile), args.Error(1)
}

func (m *MockSocialRepository) AddFriend(ctx context.Context, userID, friendID uuid.UUID) error {
	args := m.Called(ctx, userID, friendID)
	return args.Error(0)
}

func (m *MockSocialRepository) RemoveFriend(ctx context.Context, userID, friendID uuid.UUID) error {
	args := m.Called(ctx, userID, friendID)
	return args.Error(0)
}

func (m *MockSocialRepository) AddRating(ctx context.Context, userID uuid.UUID, grade *domain.Grade) error {
	args := m.Called(ctx, userID, grade)
	return args.Error(0)
}

func (m *MockSocialRepository) CreateFriendRequest(ctx context.Context, fromUserID, toUserID uuid.UUID, fromUsername string) error {
	args := m.Called(ctx, fromUserID, toUserID, fromUsername)
	return args.Error(0)
}

func (m *MockSocialRepository) GetFriendRequest(ctx context.Context, requesterID, targetID uuid.UUID) (*domain.FriendRequest, error) {
	args := m.Called(ctx, requesterID, targetID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FriendRequest), args.Error(1)
}

func (m *MockSocialRepository) GetFriendRequestByID(ctx context.Context, requestID uuid.UUID) (*domain.FriendRequest, error) {
	args := m.Called(ctx, requestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FriendRequest), args.Error(1)
}

func (m *MockSocialRepository) UpdateFriendRequestStatus(ctx context.Context, requestID uuid.UUID, status string) error {
	args := m.Called(ctx, requestID, status)
	return args.Error(0)
}

func (m *MockSocialRepository) DeleteFriendRequest(ctx context.Context, requestID uuid.UUID) error {
	args := m.Called(ctx, requestID)
	return args.Error(0)
}

func (m *MockSocialRepository) AreUsersFriends(ctx context.Context, userID1, userID2 uuid.UUID) (bool, error) {
	args := m.Called(ctx, userID1, userID2)
	return args.Bool(0), args.Error(1)
}

func (m *MockSocialRepository) GetIncomingFriendRequests(ctx context.Context, userID uuid.UUID) ([]domain.FriendRequest, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.FriendRequest), args.Error(1)
}

func (m *MockSocialRepository) GetUserRatingsWithDetails(ctx context.Context, userID uuid.UUID) ([]domain.Grade, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Grade), args.Error(1)
}

type MockUserServiceClient struct {
	mock.Mock
}

func (m *MockUserServiceClient) UserExists(ctx context.Context, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserServiceClient) GetUserByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	args := m.Called(ctx, username)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockUserServiceClient) GetUserByID(ctx context.Context, userID uuid.UUID) (*domain.UserProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserProfile), args.Error(1)
}

func TestSocialService_GetUserProfile(t *testing.T) {
	mockRepo := new(MockSocialRepository)
	mockUserClient := new(MockUserServiceClient)
	service := services.NewSocialService(mockRepo, mockUserClient, "test_secret")
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Profile exists", func(t *testing.T) {
		expectedProfile := &domain.SocialProfile{UserID: userID}
		mockRepo.On("GetProfile", ctx, userID).Return(expectedProfile, nil).Once()

		profile, err := service.GetUserProfile(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedProfile, profile)
	})

	t.Run("Profile does not exist, create new", func(t *testing.T) {
		mockRepo.On("GetProfile", ctx, userID).Return(nil, nil).Once()
		mockRepo.On("CreateProfile", ctx, userID).Return(nil).Once()
		expectedProfile := &domain.SocialProfile{UserID: userID}
		mockRepo.On("GetProfile", ctx, userID).Return(expectedProfile, nil).Once()

		profile, err := service.GetUserProfile(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedProfile, profile)
	})

	t.Run("Error getting profile", func(t *testing.T) {
		mockRepo.On("GetProfile", ctx, userID).Return(nil, errors.New("db error")).Once()

		profile, err := service.GetUserProfile(ctx, userID)
		assert.Error(t, err)
		assert.Nil(t, profile)
	})
}

func TestSocialService_SendFriendRequest(t *testing.T) {
	mockRepo := new(MockSocialRepository)
	mockUserClient := new(MockUserServiceClient)
	service := services.NewSocialService(mockRepo, mockUserClient, "test_secret")
	ctx := context.Background()

	fromUserID := uuid.New()
	targetUserID := uuid.New()
	reqDTO := services.FriendRequestDTO{
		TargetUsername: "target_user",
		FromUsername:   "from_user",
	}

	t.Run("Success", func(t *testing.T) {
		mockUserClient.On("GetUserByUsername", ctx, "target_user").Return(targetUserID, nil).Once()

		// Mock GetUserProfile calls (which call GetProfile)
		mockRepo.On("GetProfile", ctx, fromUserID).Return(&domain.SocialProfile{UserID: fromUserID}, nil).Once()
		mockRepo.On("GetProfile", ctx, targetUserID).Return(&domain.SocialProfile{UserID: targetUserID}, nil).Once()

		mockRepo.On("AreUsersFriends", ctx, fromUserID, targetUserID).Return(false, nil).Once()
		mockRepo.On("CreateFriendRequest", ctx, fromUserID, targetUserID, "from_user").Return(nil).Once()

		err := service.SendFriendRequest(ctx, fromUserID, reqDTO)
		require.NoError(t, err)
	})

	t.Run("Self friend request", func(t *testing.T) {
		// Use a DTO with same usernames to trigger the validation error
		selfReqDTO := services.FriendRequestDTO{
			TargetUsername: "same_user",
			FromUsername:   "same_user",
		}
		mockUserClient.On("GetUserByUsername", ctx, "same_user").Return(fromUserID, nil).Once()

		err := service.SendFriendRequest(ctx, fromUserID, selfReqDTO)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot add themselves")
	})

	t.Run("Already friends", func(t *testing.T) {
		mockUserClient.On("GetUserByUsername", ctx, "target_user").Return(targetUserID, nil).Once()

		// Mock GetUserProfile calls
		mockRepo.On("GetProfile", ctx, fromUserID).Return(&domain.SocialProfile{UserID: fromUserID}, nil).Once()
		mockRepo.On("GetProfile", ctx, targetUserID).Return(&domain.SocialProfile{UserID: targetUserID}, nil).Once()

		mockRepo.On("AreUsersFriends", ctx, fromUserID, targetUserID).Return(true, nil).Once()

		err := service.SendFriendRequest(ctx, fromUserID, reqDTO)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already friends")
	})
}

func TestSocialService_AcceptFriendRequest(t *testing.T) {
	mockRepo := new(MockSocialRepository)
	mockUserClient := new(MockUserServiceClient)
	service := services.NewSocialService(mockRepo, mockUserClient, "test_secret")
	ctx := context.Background()

	currentUserID := uuid.New()
	requesterID := uuid.New()
	requestID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		req := &domain.FriendRequest{
			RequestID:  requestID,
			FromUserID: requesterID,
			ToUserID:   currentUserID,
			Status:     "pending",
		}
		mockRepo.On("GetFriendRequest", ctx, requesterID, currentUserID).Return(req, nil).Once()
		mockRepo.On("AddFriend", ctx, currentUserID, requesterID).Return(nil).Once()
		mockRepo.On("AddFriend", ctx, requesterID, currentUserID).Return(nil).Once()
		mockRepo.On("UpdateFriendRequestStatus", ctx, requestID, "accepted").Return(nil).Once()

		err := service.AcceptFriendRequest(ctx, currentUserID, requesterID)
		require.NoError(t, err)
	})

	t.Run("No pending request", func(t *testing.T) {
		mockRepo.On("GetFriendRequest", ctx, requesterID, currentUserID).Return(nil, nil).Once()

		err := service.AcceptFriendRequest(ctx, currentUserID, requesterID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no pending friend request")
	})
}

func TestSocialService_AddRating(t *testing.T) {
	mockRepo := new(MockSocialRepository)
	mockUserClient := new(MockUserServiceClient)
	service := services.NewSocialService(mockRepo, mockUserClient, "test_secret")
	ctx := context.Background()

	userID := uuid.New()
	reqDTO := services.RatingRequestDTO{
		FilmID: 123,
		Grade:  5,
		Review: "Nice",
	}

	t.Run("Success", func(t *testing.T) {
		mockRepo.On("GetProfile", ctx, userID).Return(&domain.SocialProfile{UserID: userID}, nil).Once()
		mockRepo.On("AddRating", ctx, userID, mock.AnythingOfType("*domain.Grade")).Return(nil).Once()

		err := service.AddRating(ctx, userID, reqDTO)
		require.NoError(t, err)
	})

	t.Run("Profile check fails", func(t *testing.T) {
		mockRepo.On("GetProfile", ctx, userID).Return(nil, errors.New("db error")).Once()

		err := service.AddRating(ctx, userID, reqDTO)
		assert.Error(t, err)
	})
}

func TestSocialService_GetUserFriends(t *testing.T) {
	mockRepo := new(MockSocialRepository)
	mockUserClient := new(MockUserServiceClient)
	service := services.NewSocialService(mockRepo, mockUserClient, "test_secret")
	ctx := context.Background()

	userID := uuid.New()
	friendID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		profile := &domain.SocialProfile{
			UserID:  userID,
			Friends: []uuid.UUID{friendID},
		}
		mockRepo.On("GetProfile", ctx, userID).Return(profile, nil).Once()

		friendProfile := &domain.UserProfile{UserId: friendID, FirstName: "Friend"}
		mockUserClient.On("GetUserByID", ctx, friendID).Return(friendProfile, nil).Once()

		friends, err := service.GetUserFriends(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, friends, 1)
		assert.Equal(t, friendID, friends[0].UserId)
	})

	t.Run("Profile not found", func(t *testing.T) {
		mockRepo.On("GetProfile", ctx, userID).Return(nil, nil).Once()

		friends, err := service.GetUserFriends(ctx, userID)
		require.NoError(t, err)
		assert.Empty(t, friends)
	})
}

func TestSocialService_GetIncomingRequests(t *testing.T) {
	mockRepo := new(MockSocialRepository)
	mockUserClient := new(MockUserServiceClient)
	service := services.NewSocialService(mockRepo, mockUserClient, "test_secret")
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		reqs := []domain.FriendRequest{{RequestID: uuid.New()}}
		mockRepo.On("GetIncomingFriendRequests", ctx, userID).Return(reqs, nil).Once()

		res, err := service.GetIncomingRequests(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, reqs, res)
	})
}

func TestSocialService_RejectFriendRequest(t *testing.T) {
	mockRepo := new(MockSocialRepository)
	mockUserClient := new(MockUserServiceClient)
	service := services.NewSocialService(mockRepo, mockUserClient, "test_secret")
	ctx := context.Background()

	currentUserID := uuid.New()
	requesterID := uuid.New()
	requestID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		req := &domain.FriendRequest{
			RequestID:  requestID,
			FromUserID: requesterID,
			ToUserID:   currentUserID,
			Status:     "pending",
		}
		mockRepo.On("GetFriendRequestByID", ctx, requestID).Return(req, nil).Once()
		mockRepo.On("DeleteFriendRequest", ctx, requestID).Return(nil).Once()

		err := service.RejectFriendRequest(ctx, currentUserID, requestID)
		require.NoError(t, err)
	})
}

func TestSocialService_GetUserRatings(t *testing.T) {
	mockRepo := new(MockSocialRepository)
	mockUserClient := new(MockUserServiceClient)
	service := services.NewSocialService(mockRepo, mockUserClient, "test_secret")
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		ratings := []domain.Grade{{GradeID: uuid.New()}}
		mockRepo.On("GetUserRatingsWithDetails", ctx, userID).Return(ratings, nil).Once()

		res, err := service.GetUserRatings(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, ratings, res)
	})
}
