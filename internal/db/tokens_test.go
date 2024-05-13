package db

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestHashToken(t *testing.T) {
	token := "myToken"

	hash := HashToken(token)
	hashString := hex.EncodeToString(hash)
	fmt.Printf("Hash: %s\n", hashString)

	if hashString == "" {
		t.Error("HashToken should return a hash")
	}

	if hashString == token {
		t.Error("HashToken should return a different hash")
	}

	if len(hash) != 32 {
		t.Error("HashToken should return a 32 byte hash")
	}
}

func TestNewToken(t *testing.T) {
	token, err := new(1, AuthTokenTime, TokenScopeAccess)
	if err != nil {
		t.Error(err)
	}

	assert.NotEqual(t, token.Plain, hex.EncodeToString(token.Hash))
	assert.Equal(t, token.UserID, 1)
	assert.Equal(t, token.Scope, TokenScopeAccess)
	assert.Equal(t, len(token.Hash), 32)
	assert.NotEqual(t, token.Hash, nil)
	assert.Equal(t, len(token.Plain), 26)
	assert.NotEqual(t, token.Plain, "")

	if token.Expiry.Before(time.Now().Add(23 * time.Hour)) {
		t.Error("Token should expire in 24 hours")
	}
}

func TestTokenModel_Insert(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := TokenModel{DB: db}

	token, err := new(1, AuthTokenTime, TokenScopeAccess)
	if err != nil {
		t.Errorf("failed to create token: %v", err)
	}

	query := regexp.QuoteMeta(`
		INSERT INTO tokens (hash, user_id, expiry, scope_id)
		VALUES ($1, $2, $3, (SELECT id FROM scopes WHERE name = $4))`)

	mock.ExpectExec(query).WithArgs(token.Hash, token.UserID, token.Expiry, token.Scope).WillReturnResult(sqlmock.NewResult(1, 1))

	err = m.insert(token)
	if err != nil {
		t.Error(err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestTokenModel_Delete(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := TokenModel{DB: db}

	query := regexp.QuoteMeta(`
		DELETE FROM tokens
		WHERE user_id = $1 AND scope_id = (SELECT id FROM scopes WHERE name = $2)`)

	mock.ExpectExec(query).WithArgs(1, TokenScopeAccess).WillReturnResult(sqlmock.NewResult(0, 1))

	err := m.Delete(1, TokenScopeAccess)
	if err != nil {
		t.Error(err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
