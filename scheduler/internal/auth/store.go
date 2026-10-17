package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailTaken     = errors.New("email already registered")
	ErrInvalidLogin   = errors.New("invalid email or password")
	ErrInvalidRefresh = errors.New("invalid or expired refresh token")
	ErrNotFound       = errors.New("not found")
	ErrIncorrectPass  = errors.New("current password is incorrect")
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateUser(ctx context.Context, name, email, passwordHash, role string) (User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, email, role
	`, name, email, passwordHash, role))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key" {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, string, error) {
	var user User
	var passwordHash string
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, email, role, password_hash FROM users WHERE email = $1
	`, email).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrInvalidLogin
	}
	if err != nil {
		return User{}, "", fmt.Errorf("get user by email: %w", err)
	}
	return user, passwordHash, nil
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, `
		SELECT id, name, email, role FROM users WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user by ID: %w", err)
	}
	return user, nil
}

func (s *Store) UpdateName(ctx context.Context, id int64, name string) (User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, `
		UPDATE users SET name = $2 WHERE id = $1 RETURNING id, name, email, role
	`, id, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("update user name: %w", err)
	}
	return user, nil
}

func (s *Store) InsertRefreshToken(ctx context.Context, userID int64, hash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hash, expiresAt)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

func (s *Store) RotateRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin refresh rotation: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID int64
	err = tx.QueryRow(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		RETURNING user_id
	`, oldHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidRefresh
	}
	if err != nil {
		return User{}, fmt.Errorf("revoke rotated refresh token: %w", err)
	}

	user, err := scanUser(tx.QueryRow(ctx, `
		SELECT id, name, email, role FROM users WHERE id = $1
	`, userID))
	if err != nil {
		return User{}, fmt.Errorf("load refresh token user: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)
	`, userID, newHash, expiresAt); err != nil {
		return User{}, fmt.Errorf("insert rotated refresh token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit refresh rotation: %w", err)
	}
	return user, nil
}

func (s *Store) RevokeRefreshToken(ctx context.Context, hash string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE user_id = (
			SELECT user_id FROM refresh_tokens
			WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		) AND revoked_at IS NULL
	`, hash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (s *Store) ChangePassword(ctx context.Context, userID int64, passwordHash string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password change: %w", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID); err != nil {
		return fmt.Errorf("revoke refresh tokens after password change: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}

func (s *Store) SetRole(ctx context.Context, userID int64, role string) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin role change: %w", err)
	}
	defer tx.Rollback(ctx)
	user, err := scanUser(tx.QueryRow(ctx, `
		UPDATE users SET role = $2 WHERE id = $1 RETURNING id, name, email, role
	`, userID, role))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("update user role: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID); err != nil {
		return User{}, fmt.Errorf("revoke refresh tokens after role change: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit role change: %w", err)
	}
	return user, nil
}

func (s *Store) ListUsers(ctx context.Context, page, pageSize int) (UserPage, error) {
	offset := (page - 1) * pageSize
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, email, role FROM users
		ORDER BY id LIMIT $1 OFFSET $2
	`, pageSize+1, offset)
	if err != nil {
		return UserPage{}, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0, pageSize)
	hasNext := false
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return UserPage{}, fmt.Errorf("scan listed user: %w", err)
		}
		if len(users) == pageSize {
			hasNext = true
			break
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return UserPage{}, fmt.Errorf("read listed users: %w", err)
	}
	return UserPage{Items: users, Page: page, PageSize: pageSize, HasNext: hasNext}, nil
}

func (s *Store) SubmissionOwner(ctx context.Context, submissionID int64) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx, `
		SELECT user_id FROM submissions WHERE id = $1
	`, submissionID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get submission owner: %w", err)
	}
	return userID, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Role)
	return user, err
}
