// Package auth 负责账号密码校验与 access token 的签发、校验和吊销。
// 数据库只保存 token 的 SHA-256 摘要，泄露数据库也无法直接冒用会话。
//
// Package auth verifies credentials and issues, validates and revokes access tokens.
// Only the SHA-256 digest of a token is stored, so a leaked database cannot be replayed as sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"private/construction_warehouse/internal/config"
	"private/construction_warehouse/internal/model"
	"private/construction_warehouse/internal/store"
)

var (
	ErrBadCredentials = errors.New("用户名或密码错误")
	ErrUnauthorized   = errors.New("未登录或登录已过期")

	usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]{3,64}$`)
)

// dummyHash 用户不存在时也执行一次 bcrypt 比较，避免通过响应时间探测用户名。
// dummyHash keeps a bcrypt comparison for unknown users so response time does not reveal usernames.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("construction-warehouse-dummy"), bcrypt.DefaultCost)

type Service struct {
	store *store.Store
	cfg   config.AuthConfig
}

func New(st *store.Store, cfg config.AuthConfig) *Service {
	return &Service{store: st, cfg: cfg}
}

func (s *Service) TTL() time.Duration { return s.cfg.TokenTTL.Duration }

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func validatePassword(pw string) error {
	if len(pw) < 6 {
		return model.Invalid("密码至少 6 位")
	}
	// bcrypt 只使用前 72 字节
	// bcrypt only uses the first 72 bytes.
	if len(pw) > 72 {
		return model.Invalid("密码不能超过 72 个字符")
	}
	return nil
}

// EnsureInitAdmin 用户表为空时按配置创建初始账号。
// EnsureInitAdmin creates the configured initial account when there are no users.
func (s *Service) EnsureInitAdmin(ctx context.Context) error {
	n, err := s.store.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	a := s.cfg.InitAdmin
	if a.Username == "" || a.Password == "" {
		log.Printf("no users yet and auth.init_admin is not configured; enable allow_register or configure init_admin")
		return nil
	}
	if _, err := s.Register(ctx, a.Username, a.Password); err != nil {
		return err
	}
	log.Printf("created initial account %q, please change its password after login", a.Username)
	return nil
}

func (s *Service) Register(ctx context.Context, username, password string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return nil, model.Invalid("用户名需为 3-64 位字母、数字或 _ . @ -")
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	id, err := s.store.CreateUser(ctx, username, string(hash))
	if err != nil {
		return nil, err
	}
	return &model.User{ID: id, Username: username, CreatedAt: time.Now().UnixMilli()}, nil
}

// Login 校验账号密码并签发 token，返回明文 token 与过期时间。
// Login verifies credentials and issues a token, returning the plain token and its expiry.
func (s *Service) Login(ctx context.Context, username, password string) (*model.User, string, time.Time, error) {
	u, hash, err := s.store.UserByName(ctx, strings.TrimSpace(username))
	if errors.Is(err, model.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, "", time.Time{}, ErrBadCredentials
	}
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, "", time.Time{}, ErrBadCredentials
	}
	token, expires, err := s.issue(ctx, u.ID)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	return u, token, expires, nil
}

func (s *Service) issue(ctx context.Context, userID int64) (string, time.Time, error) {
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(s.TTL())
	if err := s.store.InsertToken(ctx, HashToken(token), userID, expires.UnixMilli()); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (*model.User, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	u, _, err := s.store.UserByToken(ctx, HashToken(token))
	if errors.Is(err, model.ErrNotFound) {
		return nil, ErrUnauthorized
	}
	return u, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteToken(ctx, HashToken(token))
}

// ChangePassword 修改密码，并吊销该用户除当前会话外的所有 token。
// ChangePassword updates the password and revokes all of the user's tokens except the current one.
func (s *Service) ChangePassword(ctx context.Context, userID int64, currentToken, oldPassword, newPassword string) error {
	hash, err := s.store.PasswordHash(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)) != nil {
		return model.Invalid("原密码错误")
	}
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.store.UpdatePassword(ctx, userID, string(newHash)); err != nil {
		return err
	}
	return s.store.DeleteUserTokensExcept(ctx, userID, HashToken(currentToken))
}
