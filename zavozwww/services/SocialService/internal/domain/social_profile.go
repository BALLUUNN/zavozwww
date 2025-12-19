package domain

import (
	"time"

	"github.com/google/uuid"
)

// Grade - структура, описывающая оценку пользователя на фильм.
type Grade struct {
	GradeID   uuid.UUID `json:"grade_id"`
	FilmID    int       `json:"film_id"`
	Grade     int       `json:"grade"`
	Review    string    `json:"review"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SocialPorfile - структура, описывающая профиль пользователя в сети.
type SocialProfile struct {
	UserID         uuid.UUID   `json:"user_id"`
	Friends        []uuid.UUID `json:"friends"`
	RateFilms      []uuid.UUID `json:"rate_films"`
	TotalFriends   int         `json:"total_friends"`
	TotalRatings   int         `json:"total_ratings"`
	LastActivityAt time.Time   `json:"last_activity_at"`
}

// NewGradeForFilm создает новую оценку фильма с временными метками и ID
func NewGradeForFilm(filmID int, grade int, review string) *Grade {
	now := time.Now().UTC()
	return &Grade{
		GradeID:   uuid.New(),
		FilmID:    filmID,
		Grade:     grade,
		Review:    review,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewSocialProfile создает новый социальный профиль с инициализацией
func NewSocialProfile(userID uuid.UUID) *SocialProfile {
	now := time.Now().UTC()
	return &SocialProfile{
		UserID:         userID,
		Friends:        []uuid.UUID{},
		RateFilms:      []uuid.UUID{},
		TotalFriends:   0,
		TotalRatings:   0,
		LastActivityAt: now,
	}
}

// AddFriend добавляет ID друга в профиль
func (sp *SocialProfile) AddFriend(friendID uuid.UUID) {
	if sp.HasFriend(friendID) {
		return
	}
	sp.Friends = append(sp.Friends, friendID)
	sp.TotalFriends++
	sp.LastActivityAt = time.Now().UTC()
}

// RemoveFriend удаляет ID друга из профиля
func (sp *SocialProfile) RemoveFriend(friendID uuid.UUID) bool {
	for i, id := range sp.Friends {
		if id == friendID {
			sp.Friends = append(sp.Friends[:i], sp.Friends[i+1:]...)
			sp.TotalFriends--
			sp.LastActivityAt = time.Now().UTC()
			return true
		}
	}
	return false
}

// HasFriend проверяет, является ли пользователь другом
func (sp *SocialProfile) HasFriend(friendID uuid.UUID) bool {
	for _, id := range sp.Friends {
		if id == friendID {
			return true
		}
	}
	return false
}

// AddRatingID добавляет ID оценки в профиль
func (sp *SocialProfile) AddRatingID(gradeID uuid.UUID) {
	if sp.HasRating(gradeID) {
		return
	}
	sp.RateFilms = append(sp.RateFilms, gradeID)
	sp.TotalRatings++
	sp.LastActivityAt = time.Now().UTC()
}

// RemoveRatingID удаляет ID оценки из профиля
func (sp *SocialProfile) RemoveRatingID(gradeID uuid.UUID) bool {
	for i, id := range sp.RateFilms {
		if id == gradeID {
			sp.RateFilms = append(sp.RateFilms[:i], sp.RateFilms[i+1:]...)
			sp.TotalRatings--
			sp.LastActivityAt = time.Now().UTC()
			return true
		}
	}
	return false
}

// HasRating проверяет наличие оценки по ID
func (sp *SocialProfile) HasRating(gradeID uuid.UUID) bool {
	for _, id := range sp.RateFilms {
		if id == gradeID {
			return true
		}
	}
	return false
}

// FriendRequest представляет собой запись о запросе на дружбу в базе данных.
type FriendRequest struct {
	RequestID    uuid.UUID `json:"request_id"`
	FromUserID   uuid.UUID `json:"from_user_id"`
	ToUserID     uuid.UUID `json:"to_user_id"`
	FromUsername string    `json:"from_username"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserProfile представляет профиль пользователя.
type UserProfile struct {
	UserId    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Age       int       `json:"age"`
	Info      string    `json:"info"`
	City      string    `json:"city"`
}
