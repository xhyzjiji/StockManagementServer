package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"

	"private/construction_warehouse/internal/model"
)

var ErrUserExists = errors.New("用户名已存在")

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_user`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO app_user(username, password_hash, created_at) VALUES(?, ?, ?)`,
		username, passwordHash, nowMillis())
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return 0, ErrUserExists
		}
		return 0, err
	}
	return res.LastInsertId()
}

// UserByName 返回用户与密码哈希。
// UserByName returns the user together with its password hash.
func (s *Store) UserByName(ctx context.Context, username string) (*model.User, string, error) {
	var u model.User
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, created_at FROM app_user WHERE username=?`, username).
		Scan(&u.ID, &u.Username, &hash, &u.CreatedAt)
	if err != nil {
		return nil, "", notFoundIfNoRows(err)
	}
	return &u, hash, nil
}

func (s *Store) PasswordHash(ctx context.Context, userID int64) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM app_user WHERE id=?`, userID).Scan(&hash)
	return hash, notFoundIfNoRows(err)
}

func (s *Store) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE app_user SET password_hash=? WHERE id=?`, passwordHash, userID)
	return err
}

func (s *Store) InsertToken(ctx context.Context, tokenHash string, userID, expiresAt int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO access_token(token_hash, user_id, expires_at, created_at) VALUES(?, ?, ?, ?)`,
		tokenHash, userID, expiresAt, nowMillis())
	return err
}

// UserByToken 返回未过期 token 对应的用户。
// UserByToken returns the user of a non-expired token.
func (s *Store) UserByToken(ctx context.Context, tokenHash string) (*model.User, int64, error) {
	var u model.User
	var expiresAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT u.id, u.username, u.created_at, t.expires_at
		 FROM access_token t JOIN app_user u ON u.id = t.user_id
		 WHERE t.token_hash=? AND t.expires_at > ?`, tokenHash, nowMillis()).
		Scan(&u.ID, &u.Username, &u.CreatedAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, model.ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	return &u, expiresAt, nil
}

func (s *Store) DeleteToken(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM access_token WHERE token_hash=?`, tokenHash)
	return err
}

// DeleteUserTokensExcept 修改密码后使该用户其它会话失效。
// DeleteUserTokensExcept revokes the user's other sessions after a password change.
func (s *Store) DeleteUserTokensExcept(ctx context.Context, userID int64, keepHash string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM access_token WHERE user_id=? AND token_hash<>?`, userID, keepHash)
	return err
}

func (s *Store) PurgeExpiredTokens(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM access_token WHERE expires_at <= ?`, nowMillis())
	return err
}
