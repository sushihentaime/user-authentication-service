package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"time"

	"github.com/sushihentaime/user-management-service/internal/validator"
)

type TokenScope string

const (
	TokenScopeAccess     TokenScope    = "token:access"
	TokenScopeRefresh    TokenScope    = "token:refresh"
	TokenScopeActivation TokenScope    = "token:activate"
	TokenScopeResetPwd   TokenScope    = "token:resetpwd"
	AuthTokenTime        time.Duration = 24 * time.Hour
	RefreshTokenTime     time.Duration = 7 * 24 * time.Hour
	ActivationTokenTime  time.Duration = 3 * 24 * time.Hour
	ResetPwdTokenTime    time.Duration = 1 * time.Hour
)

type Token struct {
	Plain     string               `json:"token"`
	Hash      []byte               `json:"-"`
	UserID    int                  `json:"-"`
	Expiry    time.Time            `json:"expiry"`
	Scope     TokenScope           `json:"-"`
	Validator *validator.Validator `json:"-"`
}

type TokenModel struct {
	DB *sql.DB
}

func HashToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func new(userID int, ttl time.Duration, scope TokenScope) (*Token, error) {
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	token := &Token{
		Plain:  base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes),
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	token.Hash = HashToken(token.Plain)

	return token, nil
}

func (t *Token) ValidateToken() {
	t.Validator = validator.New()

	t.Validator.Check(t.Plain != "", "token", "must be provided")
	t.Validator.Check(len(t.Plain) == 26, "token", "must be 26 bytes long")
}

func (m *TokenModel) insert(token *Token) error {
	query := `
		INSERT INTO tokens (hash, user_id, expiry, scope_id)
		VALUES ($1, $2, $3, (SELECT id FROM scopes WHERE name = $4))`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, token.Hash, token.UserID, token.Expiry, token.Scope)
	return err
}

func (m *TokenModel) CreateToken(userID int, ttl time.Duration, scope TokenScope) (*Token, error) {
	token, err := new(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = m.insert(token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (m *TokenModel) Delete(userID int, scope TokenScope) error {
	query := `
		DELETE FROM tokens
		WHERE user_id = $1 AND scope_id = (SELECT id FROM scopes WHERE name = $2)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID, scope)
	return err
}

// Get the token from the database regardless of it being expired or not
func (m *TokenModel) Get(userID int, scope TokenScope) (*Token, error) {
	token := &Token{}

	query := `
		SELECT hash, user_id, expiry, scopes.name
		FROM tokens
		INNER JOIN scopes ON tokens.scope_id = scopes.id
		WHERE user_id = $1 AND scopes.name = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, userID, scope).Scan(&token.Hash, &token.UserID, &token.Expiry, &token.Scope)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return token, nil
}
