package rout

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"social_service/internal/domain"
	"social_service/internal/services"
	"social_service/pkg/logger"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/google/uuid"
)

type ctxKey string

const userIDKey ctxKey = "userID"

// Handler обрабатывает HTTP-запросы социального графа.
type Handler struct {
	service *services.SocialService
	log     *logger.Logger
}

// UserServiceClient определяет методы для общения с микросервисом пользователей.
type UserServiceClient interface {
	UserExists(ctx context.Context, userID uuid.UUID) (bool, error)
	GetUserByUsername(ctx context.Context, username string) (uuid.UUID, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*domain.UserProfile, error)
}

type AuthServiceClient struct {
	IsExist bool               `json:"is_exist"`
	UserId  uuid.UUID          `json:"user_id"`
	Profile domain.UserProfile `json:"profile"`
}

// NewHandler создает новый экземпляр Handler.
func NewHandler(service *services.SocialService, log *logger.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

// logRequestMiddleware логирует детали каждого HTTP-запроса.
func (h *Handler) logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		defer func() {
			reqID := middleware.GetReqID(r.Context())
			h.log.Info("request completed",
				"request_id", reqID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start),
				"bytes_written", ww.BytesWritten(),
			)
		}()
		next.ServeHTTP(ww, r)
	})
}

// newRateLimiter создает middleware для ограничения частоты запросов.
func newRateLimiter() func(http.Handler) http.Handler {
	lmt := tollbooth.NewLimiter(5, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	lmt.SetMessage("You have reached maximum request limit.")
	lmt.SetMessageContentType("application/json; charset=utf-8")
	lmt.SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	return func(next http.Handler) http.Handler {
		return tollbooth.LimitFuncHandler(lmt, next.ServeHTTP)
	}
}

// authMiddleware проверяет авторизацию и извлекает UserID.
func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.Header.Get("X-User-ID")

		if userIDStr == "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				h.log.Warn("missing authorization header", "request_id", middleware.GetReqID(r.Context()))
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{"error": "authorization header is required"})
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				h.log.Warn("invalid authorization format", "request_id", middleware.GetReqID(r.Context()))
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{"error": "invalid authorization header format"})
				return
			}

			userID, err := h.service.ParseToken(parts[1])
			if err != nil {
				h.log.Warn("invalid token", "request_id", middleware.GetReqID(r.Context()), "error", err)
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{"error": "invalid token"})
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			h.log.Warn("invalid user id", "request_id", middleware.GetReqID(r.Context()), "error", err)
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, map[string]string{"error": "invalid user id in token"})
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// InitRoutes инициализирует маршруты.
func (h *Handler) InitRoutes() *chi.Mux {
	router := chi.NewRouter()

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://127.0.0.1:5500", "http://localhost:5500", "http://127.0.0.1:5501", "http://localhost:5501"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-User-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	router.Use(middleware.RequestID)
	router.Use(h.logRequestMiddleware)
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/social", func(r chi.Router) {
		r.Use(newRateLimiter())
		r.Use(h.authMiddleware)

		r.Get("/ratings", h.getRatings)
		r.Post("/ratings", h.addRating)
		r.Get("/friends", h.getFriends)
		r.Get("/friends/requests", h.getIncomingRequests)
		r.Post("/friends/requests", h.sendFriendRequest)
		r.Post("/friends/requests/accept", h.acceptFriendRequest)
		r.Post("/friends/requests/reject", h.rejectFriendRequest)
		r.Get("/profile", h.getProfile)
	})

	return router
}

func (h *Handler) getRatings(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(uuid.UUID)

	ratings, err := h.service.GetUserRatings(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get ratings", "request_id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "failed to fetch ratings"})
		return
	}

	render.JSON(w, r, ratings)
}

func (h *Handler) getFriends(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(uuid.UUID)

	friends, err := h.service.GetUserFriends(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get friends", "request_id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "failed to fetch friends"})
		return
	}

	render.JSON(w, r, friends)
}

func (h *Handler) getIncomingRequests(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(uuid.UUID)
	h.log.Info("DEBUG: fetching requests for user", "user_id", userID.String())
	requests, err := h.service.GetIncomingRequests(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get requests", "request_id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "failed to fetch requests"})
		return
	}

	if requests == nil {
		requests = []domain.FriendRequest{}
	}

	render.JSON(w, r, requests)
}

func (h *Handler) sendFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(uuid.UUID)

	var req services.FriendRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.service.SendFriendRequest(r.Context(), userID, req); err != nil {
		h.log.Error("failed to send friend request", "request_id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{"message": "friend request sent"})
}

func (h *Handler) acceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(uuid.UUID)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error("failed to read request body", "error", err)
		render.Status(r, http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	h.log.Info("DEBUG: acceptFriendRequest body", "body", string(bodyBytes))

	var input struct {
		FromUsername string `json:"from_username"`
		ToUsername   string `json:"to_username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	if input.FromUsername == "" || input.ToUsername == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "from_username and to_username are required"})
		return
	}

	requesterID, err := h.service.GetUserIDByUsername(r.Context(), input.FromUsername)
	if err != nil {
		h.log.Error("failed to get requester id", "username", input.FromUsername, "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "requester not found"})
		return
	}

	targetID, err := h.service.GetUserIDByUsername(r.Context(), input.ToUsername)
	if err != nil {
		h.log.Error("failed to get target id", "username", input.ToUsername, "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "target user not found"})
		return
	}

	if targetID != userID {
		h.log.Warn("user tried to accept request for another user", "user_id", userID, "target_id", targetID)
		render.Status(r, http.StatusForbidden)
		render.JSON(w, r, map[string]string{"error": "you can only accept requests addressed to you"})
		return
	}

	if err := h.service.AcceptFriendRequest(r.Context(), targetID, requesterID); err != nil {
		h.log.Error("failed to accept request", "request_id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{"message": "friend request accepted"})
}

func (h *Handler) rejectFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(uuid.UUID)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error("failed to read request body", "error", err)
		render.Status(r, http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	h.log.Info("DEBUG: rejectFriendRequest body", "body", string(bodyBytes))

	var input struct {
		RequestID uuid.UUID `json:"request_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.log.Error("failed to decode request body", "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.service.RejectFriendRequest(r.Context(), userID, input.RequestID); err != nil {
		h.log.Error("failed to reject request", "request_id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{"message": "friend request rejected"})
}

func (h *Handler) addRating(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(uuid.UUID)

	var req services.RatingRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.service.AddRating(r.Context(), userID, req); err != nil {
		h.log.Error("failed to add rating", "request_id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]string{"message": "rating added"})
}

func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(uuid.UUID)

	profile, err := h.service.GetUserProfile(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get profile", "request_id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "failed to fetch profile"})
		return
	}

	render.JSON(w, r, profile)
}
