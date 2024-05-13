package db

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"github.com/sushihentaime/user-management-service/internal/validator"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrDuplicateUsername = errors.New("duplicate username")
	ErrDuplicateEmail    = errors.New("duplicate email")

	EmailRX       = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	UsernameRX    = regexp.MustCompile("^[a-zA-Z0-9]+$")
	UppercaseRX   = regexp.MustCompile("[A-Z]")
	LowercaseRX   = regexp.MustCompile("[a-z]")
	NumberRX      = regexp.MustCompile("[0-9]")
	SymbolRX      = regexp.MustCompile(`[#?!@$%^&*_\\-]`)
	AnonymousUser = &User{}
)

type User struct {
	ID        int                  `json:"id"`
	Username  string               `json:"username"`
	Email     string               `json:"email"`
	Password  Password             `json:"-"`
	Activated bool                 `json:"activated"`
	CreatedAt time.Time            `json:"-"`
	Version   int                  `json:"-"`
	Validator *validator.Validator `json:"-"`
}

type Password struct {
	Plain *string `json:"-"`
	hash  []byte  `json:"-"`
}

type UserModel struct {
	DB *sql.DB
}

func (p *Password) Set(plain string) error {
	//  GenerateFromPassword does not accept passwords longer than 72 bytes, which is the longest password bcrypt will operate on.
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return err
	}

	p.Plain = &plain
	p.hash = hash

	return nil
}

func (p *Password) Compare(plain string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plain))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func (u *User) validateUsername() {
	u.Validator.Check(u.Username != "", "username", "must be provided")
	u.Validator.Check(u.Validator.CheckStringLength(u.Username, 3, 25), "username", "must be 3-25 characters long")
	u.Validator.Check(UsernameRX.MatchString(u.Username), "username", "must contain only letters and numbers")
}

func (u *User) validateEmail() {
	u.Validator.Check(u.Email != "", "email", "must be provided")
	u.Validator.Check(EmailRX.MatchString(u.Email), "email", "must be a valid email address")
}

func (u *User) validatePassword() {
	if u.Password.Plain == nil {
		return
	}

	value := len(*u.Password.Plain) >= 8 && len(*u.Password.Plain) <= 72 && UppercaseRX.MatchString(*u.Password.Plain) && LowercaseRX.MatchString(*u.Password.Plain) && NumberRX.MatchString(*u.Password.Plain) && SymbolRX.MatchString(*u.Password.Plain)

	u.Validator.Check(value, "password", "must be 8-72 characters long and contain at least one uppercase letter, one lowercase letter, one number, and one symbol")
}

func (u *User) ValidateUser() {
	u.Validator = validator.New()

	u.validateUsername()
	u.validateEmail()
	u.validatePassword()
}

func (u *User) ValidateEmail() {
	u.Validator = validator.New()

	u.validateEmail()
}

func (u *User) ValidatePassword() {
	u.Validator = validator.New()

	u.validatePassword()
}

func (u *User) ValidateLoginUser() {
	u.Validator = validator.New()

	u.validateUsername()
	u.validatePassword()
}

func (u *User) ValidateUpdateUser() {
	u.Validator = validator.New()

	if u.Email != "" {
		u.validateEmail()
	}
	if *u.Password.Plain != "" {
		u.validatePassword()
	}
}

func (m *UserModel) Create(user *User) error {
	err := user.Password.Set(*user.Password.Plain)
	if err != nil {
		return err
	}

	err = m.Insert(user)
	if err != nil {
		return err
	}

	return nil
}

func (m *UserModel) Insert(user *User) error {
	query := `
		INSERT INTO users (username, email, password_hash) 
		VALUES ($1, $2, $3)
		RETURNING id, created_at, version`

	args := []any{
		user.Username,
		user.Email,
		user.Password.hash,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)

	if err != nil {
		switch {
		case err.Error() == "pq: duplicate key value violates unique constraint \"users_username_key\"":
			return ErrDuplicateUsername
		case err.Error() == "pq: duplicate key value violates unique constraint \"users_email_key\"":
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

func (m *UserModel) GetByUsername(username string) (*User, error) {
	var user User

	query := `
		SELECT id, username, email, activated, password_hash, version
		FROM users
		WHERE username = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, username).Scan(&user.ID, &user.Username, &user.Email, &user.Activated, &user.Password.hash, &user.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

func (m *UserModel) GetByEmail(email string) (*User, error) {
	var user User

	query := `
		SELECT id, username, email, activated
		FROM users
		WHERE email = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(&user.ID, &user.Username, &user.Email, &user.Activated)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

// Update can only modify the email and password_hash fields of a user.
func (m *UserModel) Update(user *User) error {
	query := `
		UPDATE users
		SET email = $1, password_hash = $2, version = version + 1
		WHERE id = $3 AND version = $4
		RETURNING version`

	args := []any{
		user.Email,
		user.Password.hash,
		user.ID,
		user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrNotFound
		case err.Error() == "pq: duplicate key value violates unique constraint \"users_email_key\"":
			return ErrDuplicateEmail
		default:
			return err
		}
	}
	return nil
}

func (m *UserModel) Delete(id int) error {
	query := `
		DELETE FROM users
		WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrNotFound
		default:
			return err
		}
	}

	return nil
}

func (m *UserModel) GetToken(tokenScope TokenScope, token []byte) (*User, error) {
	var user User

	query := `
		SELECT u.id, u.username, u.email, u.activated
		FROM users u
		INNER JOIN tokens t ON u.id = t.user_id
		INNER JOIN scopes s ON t.scope_id = s.id
		WHERE t.hash = $1 AND s.name = $2 AND t.expiry > $3`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, token, tokenScope, time.Now()).Scan(&user.ID, &user.Username, &user.Email, &user.Activated)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

func (m *UserModel) Activate(userID int) error {
	query := `
		UPDATE users
		SET activated = TRUE
		WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrNotFound
		default:
			return err
		}
	}

	return nil
}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}
