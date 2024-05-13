package db

import (
	"database/sql"
	"database/sql/driver"
	"log"
	"regexp"
	"testing"
	"time"

	"github.com/sushihentaime/user-management-service/internal/validator"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

type anyTime struct{}

func (a anyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

func TestPassword_SetAndCompare(t *testing.T) {
	p := &Password{}

	plain := "password123"
	err := p.Set(plain)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	match, err := p.Compare(plain)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !match {
		t.Errorf("expected password to match, got false")
	}
}

func TestUser_ValidateUsername(t *testing.T) {
	tests := []struct {
		username string
		valid    bool
	}{
		{username: "", valid: false},                           // Empty username
		{username: "a", valid: false},                          // Username less than 3 characters
		{username: "abcdefghijklmnopqrstuvwxyz", valid: false}, // Username more than 25 characters
		{username: "validusername", valid: true},               // Valid username
		{username: "valid_username", valid: false},             // Valid username with underscore
		{username: "valid123", valid: true},                    // Valid username with numbers
		{username: "invalid username", valid: false},           // Username with space
		{username: "invalid@username", valid: false},           // Username with special character
		{username: "InvalidUsername", valid: true},             // Username with uppercase
	}

	for _, test := range tests {
		u := &User{
			Username:  test.username,
			Validator: validator.New(),
		}

		u.validateUsername()

		if u.Validator.Valid() != test.valid {
			t.Errorf("expected valid=%v, got valid=%v for username=%s", test.valid, u.Validator.Valid(), test.username)
		}
	}
}

func TestUser_ValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{email: "", valid: false},                   // Empty email
		{email: "invalid", valid: false},            // Invalid email
		{email: "invalid@", valid: false},           // Invalid email
		{email: "invalid.com", valid: false},        // Invalid email
		{email: "invalid@invalid", valid: false},    // Invalid email
		{email: "invalid@invalid.", valid: false},   // Invalid email
		{email: "invalid@invalid.com", valid: true}, // Valid email
	}

	for _, test := range tests {
		u := &User{
			Email:     test.email,
			Validator: validator.New(),
		}

		u.validateEmail()

		if u.Validator.Valid() != test.valid {
			t.Errorf("expected valid=%v, got valid=%v for email=%s", test.valid, u.Validator.Valid(), test.email)
		}
	}
}

func TestUser_ValidatePassword(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
	}{
		{password: "", valid: false},              // Empty password
		{password: "pass", valid: false},          // Password less than 8 characters
		{password: "pass123", valid: false},       // Password less than 8 characters
		{password: "password", valid: false},      // Password without uppercase
		{password: "password123", valid: false},   // Password without uppercase
		{password: "Password123", valid: false},   // Password without symbol
		{password: "Password123!", valid: true},   // Valid password
		{password: "Password 1234", valid: false}, // Password with space
	}

	for _, test := range tests {
		u := &User{
			Password:  Password{Plain: &test.password},
			Validator: validator.New(),
		}

		u.validatePassword()

		if u.Validator.Valid() != test.valid {
			t.Errorf("expected valid=%v, got valid=%v for password=%s", test.valid, u.Validator.Valid(), test.password)
		}
	}
}

var plain = "Test1234!"

var dataUser = &User{
	ID:       1,
	Username: "testuser",
	Email:    "testuser@example.com",
	Password: Password{
		Plain: &plain,
	},
	Version: 1,
}

var updatedDataUser = &User{
	ID:       1,
	Username: "testuser2",
	Email:    "testuser2@example.com",
	Password: Password{
		Plain: &plain,
	},
	Version: 1,
}

var expectedDataUser = &User{
	ID:       1,
	Version:  1,
	Username: "testuser",
	Email:    "testuser@example.com",
}

func MockDB() (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatalf("failed to create mock database connection: %v", err)
	}

	return db, mock
}

func TestUserModel_Insert(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := UserModel{DB: db}

	err := dataUser.Password.Set(*dataUser.Password.Plain)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	query := regexp.QuoteMeta(
		`INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, version`)

	mock.ExpectQuery(query).WithArgs(dataUser.Username, dataUser.Email, dataUser.Password.hash).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "version"}).AddRow(1, time.Now(), 1))

	err = m.Insert(dataUser)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}

	assert.Equal(t, expectedDataUser.ID, dataUser.ID)
	assert.Equal(t, expectedDataUser.Version, dataUser.Version)

	if dataUser.CreatedAt.IsZero() {
		t.Errorf("expected CreatedAt to be set, got zero value")
	}
}

func TestUserModel_GetByUsername(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := UserModel{DB: db}

	query := regexp.QuoteMeta(
		`SELECT id, username, email, activated, password_hash, version
		FROM users
		WHERE username = $1`)

	rows := sqlmock.NewRows([]string{"id", "username", "email", "activated", "password_hash", "version"}).AddRow(1, dataUser.Username, dataUser.Email, false, dataUser.Password.hash, 1)
	mock.ExpectQuery(query).WithArgs(dataUser.Username).WillReturnRows(rows)

	user, err := m.GetByUsername(dataUser.Username)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}

	assert.Equal(t, expectedDataUser.ID, user.ID)
	assert.Equal(t, expectedDataUser.Username, user.Username)
	assert.Equal(t, expectedDataUser.Email, user.Email)
	assert.Equal(t, expectedDataUser.Activated, user.Activated)
	assert.Equal(t, expectedDataUser.Version, user.Version)
}

func TestUserModel_GetByEmail(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := UserModel{DB: db}

	query := regexp.QuoteMeta(
		`SELECT id, username, email, activated
		FROM users
		WHERE email = $1`)

	rows := sqlmock.NewRows([]string{"id", "username", "email", "activated"}).AddRow(1, dataUser.Username, dataUser.Email, false)
	mock.ExpectQuery(query).WithArgs(dataUser.Email).WillReturnRows(rows)

	user, err := m.GetByEmail(dataUser.Email)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}

	assert.Equal(t, expectedDataUser.ID, user.ID)
	assert.Equal(t, expectedDataUser.Username, user.Username)
	assert.Equal(t, expectedDataUser.Email, user.Email)
	assert.Equal(t, expectedDataUser.Activated, user.Activated)
}

func TestUserModel_Update(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := UserModel{DB: db}

	err := updatedDataUser.Password.Set(*dataUser.Password.Plain)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	query := regexp.QuoteMeta(
		`UPDATE users
		SET email = $1, password_hash = $2, version = version + 1
		WHERE id = $3 AND version = $4
		RETURNING version`)

	rows := sqlmock.NewRows([]string{"version"}).AddRow(2)
	mock.ExpectQuery(query).WithArgs(updatedDataUser.Email, updatedDataUser.Password.hash, 1, 1).WillReturnRows(rows)

	err = m.Update(updatedDataUser)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}

	assert.Equal(t, 2, updatedDataUser.Version)
	assert.Equal(t, "testuser2@example.com", updatedDataUser.Email)
	assert.Equal(t, 1, updatedDataUser.ID)
}

func TestUserModel_Delete(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := UserModel{DB: db}

	query := regexp.QuoteMeta(
		`DELETE FROM users
		WHERE id = $1`)

	mock.ExpectExec(query).WithArgs(1).WillReturnResult(sqlmock.NewResult(0, 1))

	err := m.Delete(1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestUserModel_Activate(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := UserModel{DB: db}

	query := regexp.QuoteMeta(
		`UPDATE users
		SET activated = TRUE
		WHERE id = $1`)

	mock.ExpectExec(query).WithArgs(1).WillReturnResult(sqlmock.NewResult(0, 1))

	err := m.Activate(1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestUserModel_GetToken(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := UserModel{DB: db}

	tokenScope := TokenScope("scope")
	token := []byte("token")

	query := regexp.QuoteMeta(`
		SELECT u.id, u.username, u.email, u.activated
		FROM users u
		INNER JOIN tokens t ON u.id = t.user_id
		INNER JOIN scopes s ON t.scope_id = s.id
		WHERE t.hash = $1 AND s.name = $2 AND t.expiry > $3`)

	rows := sqlmock.NewRows([]string{"id", "username", "email", "activated"}).AddRow(1, "testuser", "testuser@example.com", true)
	mock.ExpectQuery(query).WithArgs(token, tokenScope, anyTime{}).WillReturnRows(rows)

	user, err := m.GetToken(tokenScope, token)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}

	expectedUser := &User{
		ID:        1,
		Username:  "testuser",
		Email:     "testuser@example.com",
		Activated: true,
	}

	assert.Equal(t, expectedUser.ID, user.ID)
	assert.Equal(t, expectedUser.Username, user.Username)
	assert.Equal(t, expectedUser.Email, user.Email)
	assert.Equal(t, expectedUser.Activated, user.Activated)
}
