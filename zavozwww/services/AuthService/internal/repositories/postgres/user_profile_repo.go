package repositories

import (
	"authServ/internal/domain/entities"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserProfileRepository предоставляет методы для работы с профилями пользователей.
type UserProfileRepository struct {
	db *pgxpool.Pool
}

// NewUserProfileRepository создает новый экземпляр UserProfileRepository.
func NewUserProfileRepository(db *pgxpool.Pool) *UserProfileRepository {
	return &UserProfileRepository{db: db}
}

// Проверка на соответствие интерфейсу во время компиляции.
var _ UserProfilesRepository = (*UserProfileRepository)(nil)

// SaveProfile сохраняет новый профиль пользователя.
func (r *UserProfileRepository) SaveProfile(ctx context.Context, profile *entities.UserProfile) error {
	const op = "repositories.postgres.UserProfileRepository.SaveProfile"
	query := `INSERT INTO user_profiles (user_id, username, first_name, last_name, age, info, city) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.Exec(ctx, query, profile.UserId, profile.Username, profile.FirstName, profile.LastName, profile.Age, profile.Info, profile.City)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%s: %w", op, ErrProfileAlreadyExists)
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// GetProfileByUserID получает профиль пользователя по его ID.
func (r *UserProfileRepository) GetProfileByUserID(ctx context.Context, userID uuid.UUID) (*entities.UserProfile, error) {
	const op = "repositories.postgres.UserProfileRepository.GetProfileByUserID"
	query := `SELECT user_id, username, first_name, last_name, age, info, city FROM user_profiles WHERE user_id = $1`

	row := r.db.QueryRow(ctx, query, userID)
	var profile entities.UserProfile
	err := row.Scan(&profile.UserId, &profile.Username, &profile.FirstName, &profile.LastName, &profile.Age, &profile.Info, &profile.City)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, ErrUserProfileNotFound)
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &profile, nil
}

// UpdateProfile обновляет существующий профиль пользователя.
func (r *UserProfileRepository) UpdateProfile(ctx context.Context, profile *entities.UserProfile) error {
	const op = "repositories.postgres.UserProfileRepository.UpdateProfile"
	query := `UPDATE user_profiles SET username = $1, first_name = $2, last_name = $3, age = $4, info = $5, city = $6 WHERE user_id = $7`

	cmdTag, err := r.db.Exec(ctx, query, profile.Username, profile.FirstName, profile.LastName, profile.Age, profile.Info, profile.City, profile.UserId)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, ErrUserProfileNotFound)
	}

	return nil
}

// DeleteProfile удаляет профиль пользователя по его ID.
func (r *UserProfileRepository) DeleteProfile(ctx context.Context, userID uuid.UUID) error {
	const op = "repositories.postgres.UserProfileRepository.DeleteProfile"
	query := `DELETE FROM user_profiles WHERE user_id = $1`

	cmdTag, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, ErrUserProfileNotFound)
	}

	return nil
}

// SearchProfilesByUsername ищет профили по частичному совпадению никнейма.
func (r *UserProfileRepository) SearchProfilesByUsername(ctx context.Context, query string) ([]entities.UserProfile, error) {
	const op = "repositories.postgres.UserProfileRepository.SearchProfilesByUsername"
	sqlQuery := `SELECT user_id, username, first_name, last_name, age, info, city FROM user_profiles WHERE username ILIKE $1`

	searchPattern := "%" + query + "%"

	rows, err := r.db.Query(ctx, sqlQuery, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	profiles := make([]entities.UserProfile, 0)
	for rows.Next() {
		var profile entities.UserProfile
		if err := rows.Scan(&profile.UserId, &profile.Username, &profile.FirstName, &profile.LastName, &profile.Age, &profile.Info, &profile.City); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		profiles = append(profiles, profile)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return profiles, nil
}
