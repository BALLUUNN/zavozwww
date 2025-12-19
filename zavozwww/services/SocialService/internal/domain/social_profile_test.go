package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewGradeForFilm(t *testing.T) {
	t.Run("Успешное создание оценки", func(t *testing.T) {
		filmID := 123
		gradeVal := 5
		review := "Great movie!"

		gradeObj := NewGradeForFilm(filmID, gradeVal, review)

		assert.NotNil(t, gradeObj)
		assert.NotEqual(t, uuid.Nil, gradeObj.GradeID, "GradeID должен быть сгенерирован")
		assert.Equal(t, filmID, gradeObj.FilmID)
		assert.Equal(t, gradeVal, gradeObj.Grade)
		assert.Equal(t, review, gradeObj.Review)

		assert.False(t, gradeObj.CreatedAt.IsZero())
		assert.False(t, gradeObj.UpdatedAt.IsZero())
		assert.Equal(t, gradeObj.CreatedAt, gradeObj.UpdatedAt)
	})
}

func TestNewSocialProfile(t *testing.T) {
	t.Run("Успешное создание профиля", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)

		assert.NotNil(t, profile)
		assert.Equal(t, userID, profile.UserID)

		assert.NotNil(t, profile.Friends)
		assert.NotNil(t, profile.RateFilms)
		assert.Empty(t, profile.Friends)
		assert.Empty(t, profile.RateFilms)

		assert.Equal(t, 0, profile.TotalFriends)
		assert.Equal(t, 0, profile.TotalRatings)

		assert.False(t, profile.LastActivityAt.IsZero())
	})
}

func TestSocialProfile_FriendsLogic(t *testing.T) {
	t.Run("Добавление друга", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		friendID := uuid.New()

		beforeActivity := profile.LastActivityAt
		time.Sleep(time.Millisecond)

		profile.AddFriend(friendID)

		assert.Equal(t, 1, profile.TotalFriends)
		assert.Len(t, profile.Friends, 1)
		assert.Equal(t, friendID, profile.Friends[0])
		assert.True(t, profile.LastActivityAt.After(beforeActivity), "LastActivityAt должно обновиться после добавления")
	})

	t.Run("Проверка наличия друга (HasFriend)", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		friendID := uuid.New()

		assert.False(t, profile.HasFriend(friendID))
		profile.AddFriend(friendID)
		assert.True(t, profile.HasFriend(friendID))
	})

	t.Run("Защита от дубликатов друзей", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		friendID := uuid.New()

		profile.AddFriend(friendID)

		lastActivity := profile.LastActivityAt
		time.Sleep(time.Millisecond)

		profile.AddFriend(friendID)

		assert.Equal(t, 1, profile.TotalFriends)
		assert.Len(t, profile.Friends, 1)
		assert.Equal(t, lastActivity, profile.LastActivityAt)
	})

	t.Run("Удаление друга", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		friendID := uuid.New()
		profile.AddFriend(friendID)

		beforeActivity := profile.LastActivityAt
		time.Sleep(time.Millisecond)

		removed := profile.RemoveFriend(friendID)

		assert.True(t, removed)
		assert.Equal(t, 0, profile.TotalFriends)
		assert.Empty(t, profile.Friends)
		assert.False(t, profile.HasFriend(friendID))
		assert.True(t, profile.LastActivityAt.After(beforeActivity), "LastActivityAt должно обновиться после удаления")
	})

	t.Run("Удаление несуществующего друга", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		friendID := uuid.New()
		profile.AddFriend(friendID)

		otherID := uuid.New()
		removed := profile.RemoveFriend(otherID)

		assert.False(t, removed)
		assert.Equal(t, 1, profile.TotalFriends)
	})
}

func TestSocialProfile_RatingsLogic(t *testing.T) {
	t.Run("Добавление ID оценки", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		gradeID := uuid.New()

		beforeActivity := profile.LastActivityAt
		time.Sleep(time.Millisecond)

		profile.AddRatingID(gradeID)

		assert.Equal(t, 1, profile.TotalRatings)
		assert.Len(t, profile.RateFilms, 1)
		assert.Equal(t, gradeID, profile.RateFilms[0])
		assert.True(t, profile.LastActivityAt.After(beforeActivity), "LastActivityAt должно обновиться")
	})

	t.Run("Проверка наличия оценки (HasRating)", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		gradeID := uuid.New()

		assert.False(t, profile.HasRating(gradeID))
		profile.AddRatingID(gradeID)
		assert.True(t, profile.HasRating(gradeID))
	})

	t.Run("Защита от дубликатов оценок", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		gradeID := uuid.New()

		profile.AddRatingID(gradeID)

		lastActivity := profile.LastActivityAt
		time.Sleep(time.Millisecond)

		profile.AddRatingID(gradeID) // Повторное добавление

		assert.Equal(t, 1, profile.TotalRatings)
		assert.Len(t, profile.RateFilms, 1)
		assert.Equal(t, lastActivity, profile.LastActivityAt)
	})

	t.Run("Удаление ID оценки", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		gradeID := uuid.New()
		profile.AddRatingID(gradeID)

		beforeActivity := profile.LastActivityAt
		time.Sleep(time.Millisecond)

		removed := profile.RemoveRatingID(gradeID)

		assert.True(t, removed)
		assert.Equal(t, 0, profile.TotalRatings)
		assert.Empty(t, profile.RateFilms)
		assert.False(t, profile.HasRating(gradeID))
		assert.True(t, profile.LastActivityAt.After(beforeActivity))
	})

	t.Run("Удаление несуществующей оценки", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)
		gradeID := uuid.New()
		profile.AddRatingID(gradeID)

		otherID := uuid.New()
		removed := profile.RemoveRatingID(otherID)

		assert.False(t, removed)
		assert.Equal(t, 1, profile.TotalRatings)
	})
}

func TestSocialProfile_IntegrationScenarios(t *testing.T) {
	t.Run("Полный жизненный цикл профиля (только ID)", func(t *testing.T) {
		userID := uuid.New()
		profile := NewSocialProfile(userID)

		friend1ID := uuid.New()
		friend2ID := uuid.New()
		profile.AddFriend(friend1ID)
		profile.AddFriend(friend2ID)

		assert.Equal(t, 2, profile.TotalFriends)
		assert.True(t, profile.HasFriend(friend1ID))
		assert.True(t, profile.HasFriend(friend2ID))

		grade1ID := uuid.New()
		grade2ID := uuid.New()
		profile.AddRatingID(grade1ID)
		profile.AddRatingID(grade2ID)

		assert.Equal(t, 2, profile.TotalRatings)
		assert.True(t, profile.HasRating(grade1ID))

		profile.RemoveFriend(friend1ID)
		assert.Equal(t, 1, profile.TotalFriends)
		assert.False(t, profile.HasFriend(friend1ID))
		assert.True(t, profile.HasFriend(friend2ID))

		profile.RemoveRatingID(grade2ID)
		assert.Equal(t, 1, profile.TotalRatings)
		assert.True(t, profile.HasRating(grade1ID))
	})
}
