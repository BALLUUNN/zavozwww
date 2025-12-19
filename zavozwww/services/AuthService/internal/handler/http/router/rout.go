package rout

import (
	repositories "authServ/internal/repositories/postgres"
	"authServ/internal/services"
	"authServ/pkg/logger"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/google/uuid"
)

// Handler обрабатывает HTTP-запросы, связанные с аутентификацией и управлением пользователями.
type Handler struct {
	userService    services.UserService
	log            *logger.Logger
	profileService services.ProfileService
}

// NewHandler создает новый экземпляр Handler с заданными сервисами и логгером.
func NewHandler(userService services.UserService, profileService services.ProfileService, log *logger.Logger) *Handler {
	return &Handler{
		userService:    userService,
		profileService: profileService,
		log:            log,
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
	lmt := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	lmt.SetMessage("You have reached maximum request limit.")
	lmt.SetMessageContentType("application/json; charset=utf-8")
	lmt.SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	return func(next http.Handler) http.Handler {
		return tollbooth.LimitFuncHandler(lmt, next.ServeHTTP)
	}
}

func (h *Handler) InitRoutes() *chi.Mux {
	router := chi.NewRouter()

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://127.0.0.1:5500", "http://localhost:5500", "http://127.0.0.1:5501", "http://localhost:5501"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
		Debug:            true,
	}))

	router.Use(middleware.RequestID)
	router.Use(h.logRequestMiddleware)
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)
	router.Use(render.SetContentType(render.ContentTypeJSON))

	// csrfMiddleware := csrf.Protect(
	// 	[]byte("32-byte-long-auth-key"),
	// 	csrf.Secure(false), // Установить в true для продакшена
	// 	csrf.HttpOnly(true),
	// 	csrf.Path("/"),
	// )
	// router.Use(csrfMiddleware)

	router.Group(func(r chi.Router) {
		r.Use(newRateLimiter())
		r.Post("/filmbuddy/register", h.register)
		r.Post("/filmbuddy/login", h.login)
		r.Post("/filmbuddy/resend-verification", h.resendVerificationEmail)
	})

	router.Post("/filmbuddy/verify", h.verifyEmail)
	router.Post("/filmbuddy/refresh", h.refreshTokens)
	router.Post("/filmbuddy/logout", h.logout)
	router.Post("/filmbuddy/profile", h.saveProfile)

	router.Get("/filmbuddy/profile", h.getUserProfileByID)
	router.Get("/filmbuddy/exists", h.userExists)
	router.Get("/filmbuddy/username", h.getUserByUsername)
	router.Post("/filmbuddy/friends/searchFriends", h.searchProfiles)

	return router
}

// register обрабатывает запросы на регистрацию новых пользователей.
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input services.RegisterUser
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.log.Error("failed to decode request body", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	err := h.userService.RegisterUser(r.Context(), input)
	if err != nil {
		h.log.Error("failed to register user", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]string{"message": "verification email sent"})
}

// login обрабатывает запросы на аутентификацию пользователей.
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input services.LoginUser
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.log.Error("failed to decode request body", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	tokens, err := h.userService.LoginUser(r.Context(), input)
	if err != nil {
		h.log.Error("failed to login user", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, tokens)
}

// verifyEmail обрабатывает запросы на верификацию электронной почты пользователей.
func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var input services.CheckTruthEmail
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.log.Error("failed to decode request body", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	tokens, err := h.userService.CheckTruthEmail(r.Context(), input)
	if err != nil {
		h.log.Error("failed to verify email", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": err.Error()})

		return
	}

	render.JSON(w, r, tokens)
}

// refreshTokens обрабатывает запросы на обновление токенов аутентификации.
func (h *Handler) refreshTokens(w http.ResponseWriter, r *http.Request) {
	var input services.RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.log.Error("failed to decode request body", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	tokens, err := h.userService.RefreshTokens(r.Context(), input)
	if err != nil {
		h.log.Error("failed to refresh tokens", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, tokens)
}

// logout обрабатывает запросы на выход пользователей из системы.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var input services.RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.log.Error("failed to decode request body", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.userService.Logout(r.Context(), input); err != nil {
		h.log.Error("failed to logout", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{"message": "logged out successfully"})
}

// resendVerificationEmail обрабатывает запрос на повторную отправку письма верификации.
func (h *Handler) resendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	var input services.ResendVerificationEmailInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.log.Error("failed to decode request body", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	err := h.userService.ResendVerificationEmail(r.Context(), input)
	if err != nil {
		h.log.Error("failed to resend verification email", "id", middleware.GetReqID(r.Context()), "error", err)
		if strings.Contains(err.Error(), "please wait") {
			render.Status(r, http.StatusTooManyRequests)
		} else {
			render.Status(r, http.StatusInternalServerError)
		}
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{"message": "a new verification email has been sent"})
}

func (h *Handler) saveProfile(w http.ResponseWriter, r *http.Request) {

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "authorization header is required"})
		return
	}

	headerParts := strings.Split(authHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "invalid authorization header format"})
		return
	}
	tokenString := headerParts[1]

	userID, err := h.userService.ParseAccessToken(r.Context(), tokenString)
	if err != nil {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "invalid token"})
		return
	}

	var input services.ProfileReq
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.log.Error("failed to decode request body", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	profileEntity := &services.ProfileReq{
		Username:  input.Username,
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Age:       input.Age,
		Info:      input.Info,
		City:      input.City,
	}

	err = h.profileService.SaveProfile(r.Context(), *profileEntity, userID)
	if err != nil {
		h.log.Error("failed to save profile", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "failed to save profile"})
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{"message": "profile saved successfully"})
}

// userExists проверяет существование пользователя по ID.
func (h *Handler) userExists(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.Header.Get("userid")
	if userIDStr == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "userid header is required"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid user id"})
		return
	}

	exists, err := h.userService.UserExists(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to check user existence", "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "internal server error"})
		return
	}

	render.JSON(w, r, map[string]bool{"exists": exists})
}

// getUserByUsername получает ID пользователя по имени.
func (h *Handler) getUserByUsername(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("username")
	if username == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "username header is required"})
		return
	}

	userID, err := h.userService.GetUserByUsername(r.Context(), username)
	if err != nil {
		h.log.Error("failed to get user by username", "error", err)
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": "user not found"})
		return
	}

	render.JSON(w, r, map[string]string{"user_id": userID.String()})
}

// getUserProfileByID получает профиль пользователя по ID из JWT токена или заголовка userid.
func (h *Handler) getUserProfileByID(w http.ResponseWriter, r *http.Request) {
	var userID uuid.UUID
	var err error

	authHeader := r.Header.Get("Authorization")
	userIDHeader := r.Header.Get("userid")

	if authHeader != "" {
		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, map[string]string{"error": "invalid authorization header format"})
			return
		}
		tokenString := headerParts[1]

		userID, err = h.userService.ParseAccessToken(r.Context(), tokenString)
		if err != nil {
			h.log.Warn("invalid token", "error", err)
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, map[string]string{"error": "invalid token"})
			return
		}
	} else if userIDHeader != "" {
		userID, err = uuid.Parse(userIDHeader)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, map[string]string{"error": "invalid user id header"})
			return
		}
	} else {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "authorization header or userid header is required"})
		return
	}

	profile, err := h.profileService.GetUserProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserProfileNotFound) {
			h.log.Warn("user profile not found", "user_id", userID)
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{"error": "profile not found"})
			return
		}
		h.log.Error("failed to get user profile", "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "failed to get profile"})
		return
	}

	responseBody, _ := json.Marshal(profile)
	h.log.Info("sending user profile response", "user_id", userID, "body", string(responseBody))

	render.JSON(w, r, profile)
}

// searchProfiles ищет профили пользователей по частичному совпадению никнейма.
func (h *Handler) searchProfiles(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.log.Error("failed to decode request body", "id", middleware.GetReqID(r.Context()), "error", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}

	if input.Username == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "username is required"})
		return
	}

	profiles, err := h.profileService.SearchProfiles(r.Context(), input.Username)
	if err != nil {
		h.log.Error("failed to search profiles", "error", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "failed to search profiles"})
		return
	}

	responseBody, _ := json.Marshal(profiles)
	h.log.Info("sending search profiles response", "body", string(responseBody))

	render.JSON(w, r, profiles)
}
