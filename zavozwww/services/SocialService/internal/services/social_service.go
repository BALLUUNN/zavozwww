package services

import (
	"context"
	"errors"
	"fmt"
	"social_service/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// tokenClaims определяет структуру данных (полезной нагрузки) для JWT токена.
type tokenClaims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
}

// FriendRequestDTO описывает запрос на добавление в друзья от клиента.
type FriendRequestDTO struct {
	TargetUsername string `json:"to_username"`
	FromUsername   string `json:"from_username"`
}

// RatingRequestDTO описывает запрос на добавление/обновление оценки.
type RatingRequestDTO struct {
	Username string `json:"username"`
	FilmID   int    `json:"film_id"`
	Grade    int    `json:"grade"`
	Review   string `json:"review"`
}

// SocialRepository определяет методы, которые мы ожидаем от слоя репозитория.
type SocialRepository interface {
	CreateProfile(ctx context.Context, userID uuid.UUID) error
	GetProfile(ctx context.Context, userID uuid.UUID) (*domain.SocialProfile, error)
	AddFriend(ctx context.Context, userID, friendID uuid.UUID) error
	RemoveFriend(ctx context.Context, userID, friendID uuid.UUID) error
	AddRating(ctx context.Context, userID uuid.UUID, grade *domain.Grade) error
	CreateFriendRequest(ctx context.Context, fromUserID, toUserID uuid.UUID, fromUsername string) error
	GetFriendRequest(ctx context.Context, requesterID, targetID uuid.UUID) (*domain.FriendRequest, error)
	GetFriendRequestByID(ctx context.Context, requestID uuid.UUID) (*domain.FriendRequest, error)
	UpdateFriendRequestStatus(ctx context.Context, requestID uuid.UUID, status string) error
	DeleteFriendRequest(ctx context.Context, requestID uuid.UUID) error
	AreUsersFriends(ctx context.Context, userID1, userID2 uuid.UUID) (bool, error)
	GetIncomingFriendRequests(ctx context.Context, userID uuid.UUID) ([]domain.FriendRequest, error)
	GetUserRatingsWithDetails(ctx context.Context, userID uuid.UUID) ([]domain.Grade, error) // НОВОЕ
}

// UserServiceClient определяет методы для общения с микросервисом пользователей.
type UserServiceClient interface {
	UserExists(ctx context.Context, userID uuid.UUID) (bool, error)
	GetUserByUsername(ctx context.Context, username string) (uuid.UUID, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*domain.UserProfile, error)
}

// SocialService - это основная структура, содержащая бизнес-логику.
type SocialService struct {
	repo       SocialRepository
	userClient UserServiceClient
	secretKey  string
}

// NewSocialService создает новый экземпляр SocialService.
func NewSocialService(repo SocialRepository, userClient UserServiceClient, secretKey string) *SocialService {
	return &SocialService{
		repo:       repo,
		userClient: userClient,
		secretKey:  secretKey,
	}
}

// ParseToken проверяет access токен и возвращает ID пользователя из него.
func (s *SocialService) ParseToken(accessToken string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(accessToken, &tokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secretKey), nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	if claims, ok := token.Claims.(*tokenClaims); ok && token.Valid {
		return claims.UserID, nil
	}

	return uuid.Nil, errors.New("invalid token")
}

// GetUserProfile возвращает профиль пользователя.
func (s *SocialService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*domain.SocialProfile, error) {
	profile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile from repository: %w", err)
	}
	if profile == nil {
		if err := s.repo.CreateProfile(ctx, userID); err != nil {
			return nil, fmt.Errorf("failed to auto-create profile: %w", err)
		}
		return s.repo.GetProfile(ctx, userID)
	}
	return profile, nil
}

// SendFriendRequest отправляет запрос на добавление в друзья.
func (s *SocialService) SendFriendRequest(ctx context.Context, fromUserID uuid.UUID, req FriendRequestDTO) error {
	targetUserID, err := s.userClient.GetUserByUsername(ctx, req.TargetUsername)
	if err != nil {
		return fmt.Errorf("failed to find target user '%s': %w", req.TargetUsername, err)
	}

	if req.FromUsername == req.TargetUsername {
		return errors.New("user cannot add themselves as a friend")
	}

	if _, err := s.GetUserProfile(ctx, fromUserID); err != nil {
		return fmt.Errorf("failed to ensure sender profile exists: %w", err)
	}

	if _, err := s.GetUserProfile(ctx, targetUserID); err != nil {
		return fmt.Errorf("failed to ensure target profile exists: %w", err)
	}

	areFriends, err := s.repo.AreUsersFriends(ctx, fromUserID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to check friendship status: %w", err)
	}
	if areFriends {
		return errors.New("users are already friends")
	}

	if err := s.repo.CreateFriendRequest(ctx, fromUserID, targetUserID, req.FromUsername); err != nil {
		return fmt.Errorf("failed to create friend request: %w", err)
	}

	return nil
}

func (s *SocialService) AcceptFriendRequest(ctx context.Context, currentUserID, requesterID uuid.UUID) error {
	request, err := s.repo.GetFriendRequest(ctx, requesterID, currentUserID)

	if err != nil {
		return fmt.Errorf("failed to get friend request: %w", err)
	}

	if request == nil {
		return errors.New("no pending friend request found")
	}
	if request.Status != "pending" {
		return errors.New("no pending friend request found")
	}

	if err := s.repo.AddFriend(ctx, currentUserID, requesterID); err != nil {
		return err
	}

	if err := s.repo.AddFriend(ctx, requesterID, currentUserID); err != nil {
		return err
	}

	if err := s.repo.UpdateFriendRequestStatus(ctx, request.RequestID, "accepted"); err != nil {
		// Логируем ошибку, но не возвращаем её клиенту, так как основное действие выполнено
		// В реальном приложении здесь лучше использовать logger
		fmt.Printf("WARN: failed to update request status for %s: %v\n", request.RequestID, err)
	}

	return nil
}

func (s *SocialService) AddRating(ctx context.Context, userID uuid.UUID, req RatingRequestDTO) error {
	if _, err := s.GetUserProfile(ctx, userID); err != nil {
		return fmt.Errorf("failed to ensure profile exists: %w", err)
	}

	grade := domain.NewGradeForFilm(req.FilmID, req.Grade, req.Review)
	if err := s.repo.AddRating(ctx, userID, grade); err != nil {
		return fmt.Errorf("failed to add rating via repository: %w", err)
	}

	return nil
}

// GetUserFriends возвращает список друзей с их полными данными.
func (s *SocialService) GetUserFriends(ctx context.Context, userID uuid.UUID) ([]domain.UserProfile, error) {
	const op = "services.SocialService.GetUserFriends"

	profile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get profile: %w", op, err)
	}
	if profile == nil {
		return []domain.UserProfile{}, nil // Профиля нет — друзей тоже нет
	}

	friends := make([]domain.UserProfile, 0, len(profile.Friends))
	for _, friendID := range profile.Friends {
		userInfo, err := s.userClient.GetUserByID(ctx, friendID)
		if err != nil {
			// Логируем, но не прерываем весь процесс
			fmt.Printf("WARN: %s: failed to get friend %s info: %v\n", op, friendID, err)
			continue
		}
		friends = append(friends, *userInfo)
	}

	return friends, nil
}

// GetIncomingRequests возвращает список запросов в друзья для текущего пользователя.
func (s *SocialService) GetIncomingRequests(ctx context.Context, userID uuid.UUID) ([]domain.FriendRequest, error) {
	const op = "services.SocialService.GetIncomingRequests"

	requests, err := s.repo.GetIncomingFriendRequests(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get incoming requests: %w", op, err)
	}

	// Если запросов нет, возвращаем пустой массив
	if requests == nil {
		requests = []domain.FriendRequest{}
	}

	return requests, nil
}

// RejectFriendRequest отклоняет запрос на дружбу.
func (s *SocialService) RejectFriendRequest(ctx context.Context, currentUserID, requestID uuid.UUID) error {
	const op = "services.SocialService.RejectFriendRequest"

	request, err := s.repo.GetFriendRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if request == nil {
		return errors.New("friend request not found")
	}

	// Проверяем, что запрос адресован текущему пользователю
	if request.ToUserID != currentUserID {
		return errors.New("you can only reject requests addressed to you")
	}

	// Удаляем запрос из базы данных, чтобы можно было создать новый
	if err := s.repo.DeleteFriendRequest(ctx, request.RequestID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// GetUserRatings возвращает список оценок пользователя.
func (s *SocialService) GetUserRatings(ctx context.Context, userID uuid.UUID) ([]domain.Grade, error) {
	const op = "services.SocialService.GetUserRatings"

	ratings, err := s.repo.GetUserRatingsWithDetails(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get ratings: %w", op, err)
	}

	if ratings == nil {
		return []domain.Grade{}, nil
	}

	return ratings, nil
}

// GetUserIDByUsername возвращает ID пользователя по его никнейму.
func (s *SocialService) GetUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	return s.userClient.GetUserByUsername(ctx, username)
}
