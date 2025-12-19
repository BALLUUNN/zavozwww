package rout_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social_service/internal/domain"
	"social_service/internal/handler/rout"
	"social_service/internal/services"
	"social_service/pkg/logger"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

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

// --- Setup ---

func setupTest(t *testing.T) (*rout.Handler, *MockSocialRepository, *MockUserServiceClient) {
	mockRepo := new(MockSocialRepository)
	mockUserClient := new(MockUserServiceClient)
	service := services.NewSocialService(mockRepo, mockUserClient, "test_secret")
	log, err := logger.NewLogger()
	require.NoError(t, err)
	h := rout.NewHandler(service, log)
	return h, mockRepo, mockUserClient
}

func TestHandler_GetProfile(t *testing.T) {
	h, mockRepo, _ := setupTest(t)
	router := h.InitRoutes()
	userID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		mockRepo.On("GetProfile", mock.Anything, userID).Return(&domain.SocialProfile{UserID: userID}, nil).Once()

		req := httptest.NewRequest("GET", "/social/profile", nil)
		req.Header.Set("X-User-ID", userID.String())

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var p domain.SocialProfile
		err := json.NewDecoder(rr.Body).Decode(&p)
		require.NoError(t, err)
		assert.Equal(t, userID, p.UserID)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/social/profile", nil)
		// No X-User-ID header

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestHandler_SendFriendRequest(t *testing.T) {
	h, mockRepo, mockUserClient := setupTest(t)
	router := h.InitRoutes()
	fromUserID := uuid.New()
	targetUserID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		mockUserClient.On("GetUserByUsername", mock.Anything, "target_user").Return(targetUserID, nil).Once()

		// Mock GetUserProfile calls
		mockRepo.On("GetProfile", mock.Anything, fromUserID).Return(&domain.SocialProfile{UserID: fromUserID}, nil).Once()
		mockRepo.On("GetProfile", mock.Anything, targetUserID).Return(&domain.SocialProfile{UserID: targetUserID}, nil).Once()

		mockRepo.On("AreUsersFriends", mock.Anything, fromUserID, targetUserID).Return(false, nil).Once()
		mockRepo.On("CreateFriendRequest", mock.Anything, fromUserID, targetUserID, "from_user").Return(nil).Once()

		body := services.FriendRequestDTO{
			TargetUsername: "target_user",
			FromUsername:   "from_user",
		}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/social/friends/requests", bytes.NewBuffer(jsonBody))
		req.Header.Set("X-User-ID", fromUserID.String())
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("User Not Found", func(t *testing.T) {
		mockUserClient.On("GetUserByUsername", mock.Anything, "unknown_user").Return(uuid.Nil, errors.New("not found")).Once()

		body := services.FriendRequestDTO{
			TargetUsername: "unknown_user",
			FromUsername:   "from_user",
		}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/social/friends/requests", bytes.NewBuffer(jsonBody))
		req.Header.Set("X-User-ID", fromUserID.String())
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestHandler_GetIncomingRequests(t *testing.T) {
	h, mockRepo, _ := setupTest(t)
	router := h.InitRoutes()
	userID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		reqs := []domain.FriendRequest{{RequestID: uuid.New()}}
		mockRepo.On("GetIncomingFriendRequests", mock.Anything, userID).Return(reqs, nil).Once()

		req := httptest.NewRequest("GET", "/social/friends/requests", nil)
		req.Header.Set("X-User-ID", userID.String())

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var res []domain.FriendRequest
		err := json.NewDecoder(rr.Body).Decode(&res)
		require.NoError(t, err)
		assert.Len(t, res, 1)
	})
}

func TestHandler_AcceptFriendRequest(t *testing.T) {
	h, mockRepo, mockUserClient := setupTest(t)
	router := h.InitRoutes()
	userID := uuid.New()
	requesterID := uuid.New()
	requestID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		fr := &domain.FriendRequest{
			RequestID:  requestID,
			FromUserID: requesterID,
			ToUserID:   userID,
			Status:     "pending",
		}
		mockUserClient.On("GetUserByUsername", mock.Anything, "requester").Return(requesterID, nil).Once()
		mockUserClient.On("GetUserByUsername", mock.Anything, "target").Return(userID, nil).Once()
		mockRepo.On("GetFriendRequest", mock.Anything, requesterID, userID).Return(fr, nil).Once()
		mockRepo.On("AddFriend", mock.Anything, userID, requesterID).Return(nil).Once()
		mockRepo.On("AddFriend", mock.Anything, requesterID, userID).Return(nil).Once()
		mockRepo.On("UpdateFriendRequestStatus", mock.Anything, requestID, "accepted").Return(nil).Once()

		body := map[string]string{
			"from_username": "requester",
			"to_username":   "target",
		}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/social/friends/requests/accept", bytes.NewBuffer(jsonBody))
		req.Header.Set("X-User-ID", userID.String())
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestHandler_RejectFriendRequest(t *testing.T) {
	h, mockRepo, _ := setupTest(t)
	router := h.InitRoutes()
	userID := uuid.New()
	requesterID := uuid.New()
	requestID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		fr := &domain.FriendRequest{
			RequestID:  requestID,
			FromUserID: requesterID,
			ToUserID:   userID,
			Status:     "pending",
		}
		mockRepo.On("GetFriendRequestByID", mock.Anything, requestID).Return(fr, nil).Once()
		mockRepo.On("DeleteFriendRequest", mock.Anything, requestID).Return(nil).Once()

		body := map[string]string{"request_id": requestID.String()}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/social/friends/requests/reject", bytes.NewBuffer(jsonBody))
		req.Header.Set("X-User-ID", userID.String())
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestHandler_AddRating(t *testing.T) {
	h, mockRepo, _ := setupTest(t)
	router := h.InitRoutes()
	userID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		mockRepo.On("GetProfile", mock.Anything, userID).Return(&domain.SocialProfile{UserID: userID}, nil).Once()
		mockRepo.On("AddRating", mock.Anything, userID, mock.AnythingOfType("*domain.Grade")).Return(nil).Once()

		body := services.RatingRequestDTO{
			FilmID: 101,
			Grade:  5,
			Review: "Great",
		}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/social/ratings", bytes.NewBuffer(jsonBody))
		req.Header.Set("X-User-ID", userID.String())
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}

func TestHandler_GetRatings(t *testing.T) {
	h, mockRepo, _ := setupTest(t)
	router := h.InitRoutes()
	userID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		ratings := []domain.Grade{{FilmID: 1, Grade: 5}}
		mockRepo.On("GetUserRatingsWithDetails", mock.Anything, userID).Return(ratings, nil).Once()

		req := httptest.NewRequest("GET", "/social/ratings", nil)
		req.Header.Set("X-User-ID", userID.String())

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Error", func(t *testing.T) {
		mockRepo.On("GetUserRatingsWithDetails", mock.Anything, userID).Return(nil, errors.New("db error")).Once()

		req := httptest.NewRequest("GET", "/social/ratings", nil)
		req.Header.Set("X-User-ID", userID.String())

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestHandler_GetFriends(t *testing.T) {
	h, mockRepo, mockUserClient := setupTest(t)
	router := h.InitRoutes()
	userID := uuid.New()
	friendID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		profile := &domain.SocialProfile{
			UserID:  userID,
			Friends: []uuid.UUID{friendID},
		}
		mockRepo.On("GetProfile", mock.Anything, userID).Return(profile, nil).Once()

		friendProfile := &domain.UserProfile{UserId: friendID, Username: "friend"}
		mockUserClient.On("GetUserByID", mock.Anything, friendID).Return(friendProfile, nil).Once()

		req := httptest.NewRequest("GET", "/social/friends", nil)
		req.Header.Set("X-User-ID", userID.String())

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var res []domain.UserProfile
		err := json.NewDecoder(rr.Body).Decode(&res)
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, friendID, res[0].UserId)
	})

	t.Run("Error", func(t *testing.T) {
		mockRepo.On("GetProfile", mock.Anything, userID).Return(nil, errors.New("db error")).Once()

		req := httptest.NewRequest("GET", "/social/friends", nil)
		req.Header.Set("X-User-ID", userID.String())

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestHandler_AuthMiddleware_Bearer(t *testing.T) {
	h, mockRepo, _ := setupTest(t)
	router := h.InitRoutes()
	userID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": userID.String(),
		})
		tokenString, _ := token.SignedString([]byte("test_secret"))

		mockRepo.On("GetProfile", mock.Anything, userID).Return(&domain.SocialProfile{UserID: userID}, nil).Once()

		req := httptest.NewRequest("GET", "/social/profile", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Invalid Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/social/profile", nil)
		req.Header.Set("Authorization", "Bearer invalid_token")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Invalid Header Format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/social/profile", nil)
		req.Header.Set("Authorization", "InvalidFormat")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}
