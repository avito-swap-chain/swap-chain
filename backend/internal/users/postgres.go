package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// PostgresService stores users in PostgreSQL.
type PostgresService struct {
	database *sql.DB
}

// NewPostgresService creates a PostgreSQL-backed user service.
func NewPostgresService(database *sql.DB) *PostgresService {
	return &PostgresService{database: database}
}

// Create validates and persists a user.
func (s *PostgresService) Create(ctx context.Context, input CreateInput) (User, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return User{}, err
	}

	user, err := scanUser(s.database.QueryRowContext(ctx, `
		INSERT INTO users (username, phone)
		VALUES ($1, $2)
		RETURNING id, username, phone, role::text, created_at`, normalized.Username, normalized.Phone))
	if err != nil {
		var postgresError *pq.Error
		if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.Constraint == "users_phone_key" {
			return User{}, ErrPhoneExists
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// Get returns a user by ID.
func (s *PostgresService) Get(ctx context.Context, userID int64) (User, error) {
	user, err := scanUser(s.database.QueryRowContext(ctx, `
		SELECT id, username, phone, role::text, created_at
		FROM users WHERE id = $1`, userID))
	return mapLookupError(user, err, "get user")
}

// FindByPhone returns a user by canonicalized phone.
func (s *PostgresService) FindByPhone(ctx context.Context, phone string) (User, error) {
	normalized, err := NormalizePhone(phone)
	if err != nil {
		return User{}, err
	}
	user, err := scanUser(s.database.QueryRowContext(ctx, `
		SELECT id, username, phone, role::text, created_at
		FROM users WHERE phone = $1`, normalized))
	return mapLookupError(user, err, "find user by phone")
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.Username, &user.Phone, &user.Role, &user.CreatedAt)
	return user, err
}

func mapLookupError(user User, err error, operation string) (User, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("%s: %w", operation, err)
	}
	return user, nil
}
