package repo

import (
	"context"
	"errors"
	"fmt"
	"social_service/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresSocialRepository реализует SocialRepository с использованием PostgreSQL.
type PostgresSocialRepository struct {
	db *pgxpool.Pool
}

// NewPostgresSocialRepository создает новый экземпляр PostgresSocialRepository
func NewPostgresSocialRepository(db *pgxpool.Pool) *PostgresSocialRepository {
	return &PostgresSocialRepository{db: db}
}

// Close закрывает соединение с базой данных
func (r *PostgresSocialRepository) Close() {
	r.db.Close()
}

// CreateProfile создает новый пустой профиль
func (r *PostgresSocialRepository) CreateProfile(ctx context.Context, userID uuid.UUID) error {
	const op = "repositories.PostgresSocialRepository.CreateProfile"

	query := `
        INSERT INTO social_profiles (user_id, total_friends, total_ratings, last_activity_at, created_at, updated_at)
        VALUES ($1, 0, 0, NOW(), NOW(), NOW())
        ON CONFLICT (user_id) DO NOTHING
    `
	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// GetProfile возвращает профиль со списками ID друзей и оценок
func (r *PostgresSocialRepository) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.SocialProfile, error) {
	const op = "repositories.PostgresSocialRepository.GetProfile"

	profile := &domain.SocialProfile{
		UserID:    userID,
		Friends:   []uuid.UUID{},
		RateFilms: []uuid.UUID{},
	}

	queryProfile := `
        SELECT total_friends, total_ratings, last_activity_at 
        FROM social_profiles 
        WHERE user_id = $1
    `

	err := r.db.QueryRow(ctx, queryProfile, userID).Scan(
		&profile.TotalFriends,
		&profile.TotalRatings,
		&profile.LastActivityAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: scan profile: %w", op, err)
	}

	queryFriends := `SELECT friend_id FROM friends WHERE user_id = $1`
	rowsFriends, err := r.db.Query(ctx, queryFriends, userID)

	if err != nil {
		return nil, fmt.Errorf("%s: query friends: %w", op, err)
	}

	defer rowsFriends.Close()

	for rowsFriends.Next() {
		var friendID uuid.UUID
		if err := rowsFriends.Scan(&friendID); err != nil {
			return nil, fmt.Errorf("%s: scan friend id: %w", op, err)
		}
		profile.Friends = append(profile.Friends, friendID)
	}

	queryRatings := `SELECT grade_id FROM film_ratings WHERE user_id = $1`
	rowsRatings, err := r.db.Query(ctx, queryRatings, userID)

	if err != nil {
		return nil, fmt.Errorf("%s: query ratings: %w", op, err)
	}

	defer rowsRatings.Close()

	for rowsRatings.Next() {
		var gradeID uuid.UUID
		if err := rowsRatings.Scan(&gradeID); err != nil {
			return nil, fmt.Errorf("%s: scan rating id: %w", op, err)
		}
		profile.RateFilms = append(profile.RateFilms, gradeID)
	}

	return profile, nil
}

// AddFriend добавляет друга и обновляет счетчик
func (r *PostgresSocialRepository) AddFriend(ctx context.Context, userID, friendID uuid.UUID) error {
	const op = "repositories.PostgresSocialRepository.AddFriend"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer tx.Rollback(ctx)

	queryAdd := `
        INSERT INTO friends (user_id, friend_id, added_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT (user_id, friend_id) DO NOTHING
    `

	tag, err := tx.Exec(ctx, queryAdd, userID, friendID)
	if err != nil {
		return fmt.Errorf("%s: insert friend: %w", op, err)
	}

	if tag.RowsAffected() > 0 {
		queryUpdate := `
            UPDATE social_profiles 
            SET total_friends = total_friends + 1, last_activity_at = NOW(), updated_at = NOW()
            WHERE user_id = $1
        `
		if _, err := tx.Exec(ctx, queryUpdate, userID); err != nil {
			return fmt.Errorf("%s: update stats: %w", op, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit transaction: %w", op, err)
	}
	return nil
}

// RemoveFriend удаляет друга и обновляет счетчик
func (r *PostgresSocialRepository) RemoveFriend(ctx context.Context, userID, friendID uuid.UUID) error {
	const op = "repositories.PostgresSocialRepository.RemoveFriend"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer tx.Rollback(ctx)

	queryRemove := `DELETE FROM friends WHERE user_id = $1 AND friend_id = $2`
	tag, err := tx.Exec(ctx, queryRemove, userID, friendID)
	if err != nil {
		return fmt.Errorf("%s: delete friend: %w", op, err)
	}

	if tag.RowsAffected() > 0 {
		queryUpdate := `
            UPDATE social_profiles 
            SET total_friends = total_friends - 1, last_activity_at = NOW(), updated_at = NOW()
            WHERE user_id = $1
        `
		if _, err := tx.Exec(ctx, queryUpdate, userID); err != nil {
			return fmt.Errorf("%s: update stats: %w", op, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit transaction: %w", op, err)
	}
	return nil
}

// AddRating добавляет оценку и обновляет счетчик
func (r *PostgresSocialRepository) AddRating(ctx context.Context, userID uuid.UUID, grade *domain.Grade) error {
	const op = "repositories.PostgresSocialRepository.AddRating"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer tx.Rollback(ctx)

	queryAdd := `
        INSERT INTO film_ratings (grade_id, user_id, film_id, grade, review, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `
	_, err = tx.Exec(ctx, queryAdd, grade.GradeID, userID, grade.FilmID, grade.Grade, grade.Review, grade.CreatedAt, grade.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%s: insert rating: %w", op, err)
	}

	queryUpdate := `
        UPDATE social_profiles 
        SET total_ratings = total_ratings + 1, last_activity_at = NOW(), updated_at = NOW()
        WHERE user_id = $1
    `
	if _, err := tx.Exec(ctx, queryUpdate, userID); err != nil {
		return fmt.Errorf("%s: update stats: %w", op, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit transaction: %w", op, err)
	}
	return nil
}

// DeleteRating удаляет оценку по ID и обновляет счетчик
func (r *PostgresSocialRepository) DeleteRating(ctx context.Context, userID, gradeID uuid.UUID) error {
	const op = "repositories.PostgresSocialRepository.DeleteRating"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer tx.Rollback(ctx)

	queryRemove := `DELETE FROM film_ratings WHERE grade_id = $1 AND user_id = $2`
	tag, err := tx.Exec(ctx, queryRemove, gradeID, userID)
	if err != nil {
		return fmt.Errorf("%s: delete rating: %w", op, err)
	}

	if tag.RowsAffected() > 0 {
		queryUpdate := `
            UPDATE social_profiles 
            SET total_ratings = total_ratings - 1, last_activity_at = NOW(), updated_at = NOW()
            WHERE user_id = $1
        `
		if _, err := tx.Exec(ctx, queryUpdate, userID); err != nil {
			return fmt.Errorf("%s: update stats: %w", op, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit transaction: %w", op, err)
	}
	return nil
}

// GetGradeByID получает полную структуру оценки по ID
func (r *PostgresSocialRepository) GetGradeByID(ctx context.Context, gradeID uuid.UUID) (*domain.Grade, error) {
	const op = "repositories.PostgresSocialRepository.GetGradeByID"

	grade := &domain.Grade{}
	query := `
        SELECT grade_id, film_id, grade, review, created_at, updated_at
        FROM film_ratings
        WHERE grade_id = $1
    `
	err := r.db.QueryRow(ctx, query, gradeID).Scan(
		&grade.GradeID,
		&grade.FilmID,
		&grade.Grade,
		&grade.Review,
		&grade.CreatedAt,
		&grade.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Не найдено - не ошибка
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return grade, nil
}

// CreateFriendRequest создает запись о запросе на дружбу в таблице friend_requests.
func (r *PostgresSocialRepository) CreateFriendRequest(ctx context.Context, fromUserID, toUserID uuid.UUID, fromUsername string) error {
	const op = "repositories.PostgresSocialRepository.CreateFriendRequest"

	query := `
        INSERT INTO friend_requests (from_user_id, to_user_id, from_username)
        VALUES ($1, $2, $3)
        ON CONFLICT (from_user_id, to_user_id) DO NOTHING
    `

	_, err := r.db.Exec(ctx, query, fromUserID, toUserID, fromUsername)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// GetFriendRequest находит активный (pending) запрос на дружбу.
func (r *PostgresSocialRepository) GetFriendRequest(ctx context.Context, fromUserID, toUserID uuid.UUID) (*domain.FriendRequest, error) {
	const op = "repositories.PostgresSocialRepository.GetFriendRequest"

	req := &domain.FriendRequest{}

	query := `
        SELECT request_id, from_user_id, to_user_id, from_username, status, created_at
        FROM friend_requests
        WHERE from_user_id = $1 AND to_user_id = $2 AND status = 'pending'
    `

	err := r.db.QueryRow(ctx, query, fromUserID, toUserID).Scan(
		&req.RequestID,
		&req.FromUserID,
		&req.ToUserID,
		&req.FromUsername,
		&req.Status,
		&req.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return req, nil
}

// GetFriendRequestByID находит запрос на дружбу по его ID.
func (r *PostgresSocialRepository) GetFriendRequestByID(ctx context.Context, requestID uuid.UUID) (*domain.FriendRequest, error) {
	const op = "repositories.PostgresSocialRepository.GetFriendRequestByID"

	req := &domain.FriendRequest{}

	query := `
        SELECT request_id, from_user_id, to_user_id, from_username, status, created_at
        FROM friend_requests
        WHERE request_id = $1
    `

	err := r.db.QueryRow(ctx, query, requestID).Scan(
		&req.RequestID,
		&req.FromUserID,
		&req.ToUserID,
		&req.FromUsername,
		&req.Status,
		&req.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return req, nil
}

// UpdateFriendRequestStatus обновляет статус запроса (например, на 'accepted' или 'rejected').
func (r *PostgresSocialRepository) UpdateFriendRequestStatus(ctx context.Context, requestID uuid.UUID, status string) error {
	const op = "repositories.PostgresSocialRepository.UpdateFriendRequestStatus"

	query := `
        UPDATE friend_requests
        SET status = $1
        WHERE request_id = $2
    `
	tag, err := r.db.Exec(ctx, query, status, requestID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: friend request with id %s not found", op, requestID)
	}

	return nil
}

// AreUsersFriends проверяет, являются ли два пользователя друзьями.
func (r *PostgresSocialRepository) AreUsersFriends(ctx context.Context, userID1, userID2 uuid.UUID) (bool, error) {
	const op = "repositories.PostgresSocialRepository.AreUsersFriends"

	var exists bool
	query := `
        SELECT EXISTS (
            SELECT 1 FROM friends WHERE user_id = $1 AND friend_id = $2
        )
    `
	err := r.db.QueryRow(ctx, query, userID1, userID2).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return exists, nil
}

// GetUserRatingsWithDetails получает все оценки пользователя с деталями.
func (r *PostgresSocialRepository) GetUserRatingsWithDetails(ctx context.Context, userID uuid.UUID) ([]domain.Grade, error) {
	const op = "repositories.PostgresSocialRepository.GetUserRatingsWithDetails"

	query := `
        SELECT grade_id, film_id, grade, review, created_at, updated_at
        FROM film_ratings
        WHERE user_id = $1
        ORDER BY created_at DESC
    `
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var ratings []domain.Grade
	for rows.Next() {
		var rtg domain.Grade
		if err := rows.Scan(&rtg.GradeID, &rtg.FilmID, &rtg.Grade, &rtg.Review, &rtg.CreatedAt, &rtg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		ratings = append(ratings, rtg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows iteration: %w", op, err)
	}

	return ratings, nil
}

// GetIncomingFriendRequests возвращает список активных запросов в друзья для пользователя.
func (r *PostgresSocialRepository) GetIncomingFriendRequests(ctx context.Context, userID uuid.UUID) ([]domain.FriendRequest, error) {
	const op = "repositories.PostgresSocialRepository.GetIncomingFriendRequests"

	query := `
        SELECT request_id, from_user_id, to_user_id, from_username, status, created_at
        FROM friend_requests
        WHERE to_user_id = $1 AND status = 'pending'
        ORDER BY created_at DESC
    `

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	requests := []domain.FriendRequest{}
	for rows.Next() {
		var req domain.FriendRequest
		err := rows.Scan(
			&req.RequestID,
			&req.FromUserID,
			&req.ToUserID,
			&req.FromUsername,
			&req.Status,
			&req.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		requests = append(requests, req)
	}

	return requests, nil
}

// DeleteFriendRequest удаляет запрос на дружбу из базы данных.
func (r *PostgresSocialRepository) DeleteFriendRequest(ctx context.Context, requestID uuid.UUID) error {
	const op = "repositories.PostgresSocialRepository.DeleteFriendRequest"

	query := `DELETE FROM friend_requests WHERE request_id = $1`
	tag, err := r.db.Exec(ctx, query, requestID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: friend request with id %s not found", op, requestID)
	}

	return nil
}
