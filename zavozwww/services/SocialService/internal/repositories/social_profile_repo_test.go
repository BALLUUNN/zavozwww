package repo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"social_service/internal/domain"
	repo "social_service/internal/repositories"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSocialTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	ctx := context.Background()

	defaultConnStr := "user=postgres password=6324eB6324 host=localhost port=5432 dbname=postgres sslmode=disable"
	tempPool, err := pgxpool.New(ctx, defaultConnStr)
	if err == nil {
		_, _ = tempPool.Exec(ctx, "CREATE DATABASE social_service_test")
		tempPool.Close()
	}

	targetConnStr := os.Getenv("TEST_DATABASE_URL")
	if targetConnStr == "" {
		targetConnStr = "user=postgres password=6324eB6324 host=localhost port=5432 dbname=social_service_test sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, targetConnStr)
	require.NoError(t, err, "Не удалось подключиться к БД social_service_test")

	schema := `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

		CREATE TABLE IF NOT EXISTS social_profiles (
			user_id UUID PRIMARY KEY,
			total_friends INTEGER DEFAULT 0 NOT NULL CHECK (total_friends >= 0),
			total_ratings INTEGER DEFAULT 0 NOT NULL CHECK (total_ratings >= 0),
			last_activity_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
		);

		CREATE TABLE IF NOT EXISTS friends (
			user_id UUID NOT NULL REFERENCES social_profiles(user_id) ON DELETE CASCADE,
			friend_id UUID NOT NULL, 
			added_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
			PRIMARY KEY (user_id, friend_id)
		);

		CREATE TABLE IF NOT EXISTS film_ratings (
			grade_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES social_profiles(user_id) ON DELETE CASCADE,
			film_id INTEGER NOT NULL,
			grade INTEGER NOT NULL CHECK (grade >= 0 AND grade <= 5),
			review TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
			CONSTRAINT unique_user_film_rating UNIQUE (user_id, film_id)
		);

		CREATE INDEX IF NOT EXISTS idx_ratings_film_id ON film_ratings(film_id);
		CREATE INDEX IF NOT EXISTS idx_ratings_user_id ON film_ratings(user_id);
		CREATE INDEX IF NOT EXISTS idx_friends_friend_id ON friends(friend_id);

		CREATE TABLE IF NOT EXISTS friend_requests (
			request_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			from_user_id UUID NOT NULL REFERENCES social_profiles(user_id) ON DELETE CASCADE,
			to_user_id UUID NOT NULL REFERENCES social_profiles(user_id) ON DELETE CASCADE,
			from_username VARCHAR(255) NOT NULL, 
			status VARCHAR(20) NOT NULL DEFAULT 'pending', 
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
			CONSTRAINT unique_pending_request UNIQUE (from_user_id, to_user_id)
		);
	`
	_, err = pool.Exec(ctx, schema)
	require.NoError(t, err, "Не удалось создать схему БД")

	return pool, func() {
		_, err := pool.Exec(ctx, "TRUNCATE TABLE friend_requests, film_ratings, friends, social_profiles CASCADE")
		if err != nil {
			t.Logf("Ошибка при очистке таблиц: %v", err)
		}
		pool.Close()
	}
}

func TestPostgresSocialRepository_CreateProfile(t *testing.T) {
	pool, teardown := setupSocialTestDB(t)
	defer teardown()

	r := repo.NewPostgresSocialRepository(pool)
	ctx := context.Background()
	userID := uuid.New()

	err := r.CreateProfile(ctx, userID)
	require.NoError(t, err)

	// Проверка
	p, err := r.GetProfile(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, userID, p.UserID)
	assert.Equal(t, 0, p.TotalFriends)
	assert.Equal(t, 0, p.TotalRatings)
}

func TestPostgresSocialRepository_AddFriend(t *testing.T) {
	pool, teardown := setupSocialTestDB(t)
	defer teardown()

	r := repo.NewPostgresSocialRepository(pool)
	ctx := context.Background()

	user1 := uuid.New()
	user2 := uuid.New()

	require.NoError(t, r.CreateProfile(ctx, user1))
	// user2 не обязательно должен иметь профиль, так как friend_id не ссылается на social_profiles

	err := r.AddFriend(ctx, user1, user2)
	require.NoError(t, err)

	// Проверка добавления друга
	isFriend, err := r.AreUsersFriends(ctx, user1, user2)
	require.NoError(t, err)
	assert.True(t, isFriend)

	// Проверка обновления статистики
	p, err := r.GetProfile(ctx, user1)
	require.NoError(t, err)
	assert.Equal(t, 1, p.TotalFriends)
	assert.Contains(t, p.Friends, user2)
}

func TestPostgresSocialRepository_RemoveFriend(t *testing.T) {
	pool, teardown := setupSocialTestDB(t)
	defer teardown()

	r := repo.NewPostgresSocialRepository(pool)
	ctx := context.Background()

	user1 := uuid.New()
	user2 := uuid.New()

	require.NoError(t, r.CreateProfile(ctx, user1))
	require.NoError(t, r.AddFriend(ctx, user1, user2))

	err := r.RemoveFriend(ctx, user1, user2)
	require.NoError(t, err)

	// Проверка удаления друга
	isFriend, err := r.AreUsersFriends(ctx, user1, user2)
	require.NoError(t, err)
	assert.False(t, isFriend)

	// Проверка обновления статистики
	p, err := r.GetProfile(ctx, user1)
	require.NoError(t, err)
	assert.Equal(t, 0, p.TotalFriends)
	assert.NotContains(t, p.Friends, user2)
}

func TestPostgresSocialRepository_AddRating(t *testing.T) {
	pool, teardown := setupSocialTestDB(t)
	defer teardown()

	r := repo.NewPostgresSocialRepository(pool)
	ctx := context.Background()

	userID := uuid.New()
	require.NoError(t, r.CreateProfile(ctx, userID))

	gradeID := uuid.New()
	grade := &domain.Grade{
		GradeID:   gradeID,
		FilmID:    101,
		Grade:     5,
		Review:    "Great movie!",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := r.AddRating(ctx, userID, grade)
	require.NoError(t, err)

	// Проверка добавления оценки
	savedGrade, err := r.GetGradeByID(ctx, gradeID)
	require.NoError(t, err)
	assert.NotNil(t, savedGrade)
	assert.Equal(t, grade.FilmID, savedGrade.FilmID)
	assert.Equal(t, grade.Grade, savedGrade.Grade)
	assert.Equal(t, grade.Review, savedGrade.Review)

	// Проверка обновления статистики
	p, err := r.GetProfile(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, p.TotalRatings)
	assert.Contains(t, p.RateFilms, gradeID)
}

func TestPostgresSocialRepository_DeleteRating(t *testing.T) {
	pool, teardown := setupSocialTestDB(t)
	defer teardown()

	r := repo.NewPostgresSocialRepository(pool)
	ctx := context.Background()

	userID := uuid.New()
	require.NoError(t, r.CreateProfile(ctx, userID))

	gradeID := uuid.New()
	grade := &domain.Grade{
		GradeID:   gradeID,
		FilmID:    101,
		Grade:     5,
		Review:    "Great movie!",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	require.NoError(t, r.AddRating(ctx, userID, grade))

	err := r.DeleteRating(ctx, userID, gradeID)
	require.NoError(t, err)

	// Проверка удаления оценки
	savedGrade, err := r.GetGradeByID(ctx, gradeID)
	require.NoError(t, err)
	assert.Nil(t, savedGrade)

	// Проверка обновления статистики
	p, err := r.GetProfile(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0, p.TotalRatings)
	assert.NotContains(t, p.RateFilms, gradeID)
}

func TestPostgresSocialRepository_FriendRequests(t *testing.T) {
	pool, teardown := setupSocialTestDB(t)
	defer teardown()

	r := repo.NewPostgresSocialRepository(pool)
	ctx := context.Background()

	user1 := uuid.New()
	user2 := uuid.New()
	require.NoError(t, r.CreateProfile(ctx, user1))
	require.NoError(t, r.CreateProfile(ctx, user2))

	// 1. Создание запроса
	err := r.CreateFriendRequest(ctx, user1, user2, "user1_name")
	require.NoError(t, err)

	// 2. Получение запроса
	req, err := r.GetFriendRequest(ctx, user1, user2)
	require.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, user1, req.FromUserID)
	assert.Equal(t, user2, req.ToUserID)
	assert.Equal(t, "pending", req.Status)

	// 3. Получение входящих запросов
	incoming, err := r.GetIncomingFriendRequests(ctx, user2)
	require.NoError(t, err)
	assert.Len(t, incoming, 1)
	assert.Equal(t, req.RequestID, incoming[0].RequestID)

	// 4. Обновление статуса
	err = r.UpdateFriendRequestStatus(ctx, req.RequestID, "accepted")
	require.NoError(t, err)

	// Проверка обновления статуса
	// GetFriendRequest возвращает только 'pending', поэтому должен вернуть nil
	reqPending, err := r.GetFriendRequest(ctx, user1, user2)
	require.NoError(t, err)
	assert.Nil(t, reqPending)
}

func TestPostgresSocialRepository_GetUserRatingsWithDetails(t *testing.T) {
	pool, teardown := setupSocialTestDB(t)
	defer teardown()

	r := repo.NewPostgresSocialRepository(pool)
	ctx := context.Background()

	userID := uuid.New()
	require.NoError(t, r.CreateProfile(ctx, userID))

	grade1 := &domain.Grade{GradeID: uuid.New(), FilmID: 1, Grade: 5, Review: "Good", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	grade2 := &domain.Grade{GradeID: uuid.New(), FilmID: 2, Grade: 3, Review: "Ok", CreatedAt: time.Now().Add(time.Hour), UpdatedAt: time.Now()}

	require.NoError(t, r.AddRating(ctx, userID, grade1))
	require.NoError(t, r.AddRating(ctx, userID, grade2))

	ratings, err := r.GetUserRatingsWithDetails(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, ratings, 2)
	// Сортировка по created_at DESC
	assert.Equal(t, grade2.FilmID, ratings[0].FilmID)
	assert.Equal(t, grade1.FilmID, ratings[1].FilmID)
}
