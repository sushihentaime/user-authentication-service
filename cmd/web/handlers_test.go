package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/sushihentaime/user-management-service/internal/db"

	"github.com/stretchr/testify/assert"
)

func TestCreateUserHandler(t *testing.T) {
	t.Parallel()

	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())

	testCases := []struct {
		name       string
		payload    any
		setup      func() error
		wantStatus int
		wantBody   envelope
	}{
		{
			name: "valid payload",
			payload: createUserInput{
				Username: "testuser",
				Email:    "testuser@example.com",
				Password: "Test1234!",
			},
			wantStatus: http.StatusCreated,
		}, {
			name: "invalid payload",
			payload: createUserInput{
				Username: "testuser",
				Email:    "test",
				Password: "Test1234!",
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"email": "must be a valid email address",
				},
			},
		}, {
			name: "duplicate username",
			payload: createUserInput{
				Username: "testuser",
				Email:    "testuser1@example.com",
				Password: "Test1234!",
			},
			setup: func() error {
				password := "Test1234!"

				user := &db.User{
					Username: "testuser",
					Email:    "testuser@example.com",
					Password: db.Password{
						Plain: &password,
					},
				}
				err := app.models.Users.Create(user)
				if err != nil {
					return err
				}
				return nil
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"username": "a user with this username already exists",
				},
			}}, {
			name: "duplicate email",
			payload: createUserInput{
				Username: "testuser1",
				Email:    "testuser@example.com",
				Password: "Test1234!",
			},
			setup: func() error {
				password := "Test1234!"

				user := &db.User{
					Username: "testuser",
					Email:    "testuser@example.com",
					Password: db.Password{
						Plain: &password,
					},
				}

				err := app.models.Users.Create(user)
				if err != nil {
					return err
				}
				return nil
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"email": "a user with this email address already exists",
				},
			},
		},
		{
			name: "invalid password",
			payload: createUserInput{
				Username: "testuser1",
				Email:    "testuser@example.com",
				Password: "test123",
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"password": "must be 8-72 characters long and contain at least one uppercase letter, one lowercase letter, one number, and one symbol",
				},
			},
		},
		{
			name: "invalid username",
			payload: createUserInput{
				Username: "testuser1!",
				Email:    "testuser@example.com",
				Password: "Test1234!",
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"username": "must contain only letters and numbers",
				},
			},
		}, {
			name: "invalid email",
			payload: createUserInput{
				Username: "testuser1",
				Email:    "testuser@example",
				Password: "Test1234!",
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"email": "must be a valid email address",
				},
			},
		}, {
			name: "additional field in payload",
			payload: map[string]any{
				"name":     "testuser",
				"username": "testuser",
				"email":    "testuser@example.com",
				"password": "Test1234!",
			},
			wantStatus: http.StatusBadRequest,
			wantBody: envelope{
				"error": "request body contains unknown field \"name\"",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {

			if tt.setup != nil {
				err := tt.setup()
				if err != nil {
					t.Fatalf("could not setup test: %v", err)
				}
			}

			status, _, body := ts.post(t, "/v1/users/new", tt.payload)
			assert.Equal(t, tt.wantStatus, status, "want %d; got %d", tt.wantStatus, status)

			if tt.wantBody == nil {
				tt.wantBody = body
			}

			assert.JSONEq(t, tt.wantBody.JSON(), body.JSON(), "want %s; got %s", tt.wantBody.JSON(), body.JSON())

			if tt.wantStatus == http.StatusCreated {
				dbUser, err := app.models.Users.GetByUsername(tt.payload.(createUserInput).Username)
				fmt.Printf("%+v", dbUser)
				assert.NoError(t, err)
				assert.NotNil(t, dbUser)
				assert.Equal(t, tt.payload.(createUserInput).Username, dbUser.Username)
				assert.False(t, dbUser.Activated)
				assert.Equal(t, 1, dbUser.Version)

				dbToken, err := app.models.Tokens.Get(dbUser.ID, db.TokenScopeActivation)
				assert.NoError(t, err)
				assert.NotNil(t, dbToken)
				assert.Equal(t, dbUser.ID, dbToken.UserID)

				permissions, err := app.models.Permissions.Get(dbUser.ID)
				assert.NoError(t, err)
				assert.NotNil(t, permissions)
				assert.Len(t, *permissions, 1)
			} else {
				var count int
				err := app.models.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
				assert.NoError(t, err)
				if tt.setup != nil {
					assert.Equal(t, 1, count)
				} else {
					assert.Equal(t, 0, count)
				}

				err = app.models.DB.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&count)
				assert.NoError(t, err)
				if tt.setup != nil {
					assert.Equal(t, 0, count)
				} else {
					assert.Equal(t, 0, count)
				}

				err = app.models.DB.QueryRow("SELECT COUNT(*) FROM user_permissions").Scan(&count)
				assert.NoError(t, err)
				if tt.setup != nil {
					assert.Equal(t, 0, count)
				} else {
					assert.Equal(t, 0, count)
				}

			}
			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})
		})
	}
}

func TestActivateUserHandler(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())

	pwd := "Test1234!"

	validUser := db.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Password: db.Password{
			Plain: &pwd,
		},
	}

	setup := func(expiration time.Duration) (*db.Token, error) {
		err := app.models.Users.Create(&validUser)
		if err != nil {
			return nil, err
		}
		validToken, err := app.models.Tokens.CreateToken(validUser.ID, expiration, db.TokenScopeActivation)
		if err != nil {
			return nil, err
		}
		return validToken, nil
	}

	testCases := []struct {
		name       string
		payload    *tokenInput
		setup      func() (*db.Token, error)
		wantStatus int
		wantBody   envelope
	}{
		{
			name:       "valid token",
			wantStatus: http.StatusOK,
			wantBody: envelope{
				"message": "user account successfully activated",
			},
			setup: func() (*db.Token, error) {
				token, err := setup(db.ActivationTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
		},
		{
			name: "invalid token",
			payload: &tokenInput{
				Token: "invalidtoken",
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"token": "must be 26 bytes long",
				},
			},
		},
		{
			name: "expired token",
			setup: func() (*db.Token, error) {
				token, err := setup(-1 * time.Second)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"token": "invalid or expired activation token",
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var validToken *db.Token
			if tt.setup != nil {
				token, err := tt.setup()
				validToken = token
				assert.NoError(t, err)
			}

			if tt.payload == nil {
				tt.payload = &tokenInput{
					Token: validToken.Plain,
				}
			}

			status, _, body := ts.put(t, "/v1/users/activate", tt.payload)
			assert.Equal(t, tt.wantStatus, status, "want %d; got %d", tt.wantStatus, status)
			assert.JSONEq(t, tt.wantBody.JSON(), body.JSON(), "want %s; got %s", tt.wantBody.JSON(), body.JSON())

			if tt.wantStatus == http.StatusOK {
				activatedUser, err := app.models.Users.GetByUsername(validUser.Username)
				assert.NoError(t, err)
				assert.True(t, activatedUser.Activated)

				// Check that the token has been deleted from the database.
				_, err = app.models.Tokens.Get(validToken.UserID, db.TokenScopeActivation)
				if err != db.ErrNotFound {
					t.Errorf("want token to be deleted from the database")
				}

				permissions, err := app.models.Permissions.Get(activatedUser.ID)
				assert.NoError(t, err)
				assert.NotNil(t, permissions)
				assert.Contains(t, *permissions, db.PermissionWriteUser)

			} else {
				var count int

				assert.Equal(t, validUser.Activated, false)
				err := app.models.DB.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&count)
				assert.NoError(t, err)

				permissions, err := app.models.Permissions.Get(validUser.ID)
				assert.NoError(t, err)
				for _, p := range *permissions {
					assert.NotEqual(t, p, db.PermissionWriteUser)
				}
			}

			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})
		})
	}
}

func TestCreateAuthTokenHandler(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())

	pwd := "Test1234!"

	validUser := db.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Password: db.Password{
			Plain: &pwd,
		},
	}

	setup := func() error {
		err := app.models.Users.Create(&validUser)
		if err != nil {
			return err
		}

		err = app.models.Permissions.Add(validUser.ID, db.PermissionWriteUser, db.PermissionReadUser)
		if err != nil {
			return err
		}

		return nil
	}

	testCases := []struct {
		name       string
		payload    loginUserInput
		setup      func() error
		wantStatus int
		wantBody   envelope
	}{
		{
			name: "Valid request",
			payload: loginUserInput{
				Username: validUser.Username,
				Password: pwd,
			},
			setup:      setup,
			wantStatus: http.StatusOK,
		},
		{
			name: "Invalid request",
			payload: loginUserInput{
				Username: "testuser1",
				Password: "Test1234!",
			},
			setup:      setup,
			wantStatus: http.StatusUnauthorized,
			wantBody: envelope{
				"error": "invalid authentication credentials",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				err := tt.setup()
				assert.NoError(t, err)
			}

			status, _, body := ts.post(t, "/v1/users/authenticate", tt.payload)

			assert.Equal(t, tt.wantStatus, status, "want %d; got %d", tt.wantStatus, status)

			if tt.wantBody == nil {
				tt.wantBody = body
			}

			assert.JSONEq(t, tt.wantBody.JSON(), body.JSON(), "want %s; got %s", tt.wantBody.JSON(), body.JSON())

			if tt.wantStatus == http.StatusOK {
				dbAccessToken, err := app.models.Tokens.Get(validUser.ID, db.TokenScopeAccess)
				assert.NoError(t, err)
				assert.Equal(t, validUser.ID, dbAccessToken.UserID)
				assert.Equal(t, db.TokenScopeAccess, dbAccessToken.Scope)
				assert.WithinDuration(t, dbAccessToken.Expiry, time.Now().Add(db.AuthTokenTime), 10*time.Second)

				dbRefreshToken, err := app.models.Tokens.Get(validUser.ID, db.TokenScopeRefresh)
				assert.NoError(t, err)
				assert.Equal(t, validUser.ID, dbRefreshToken.UserID)
				assert.Equal(t, db.TokenScopeRefresh, dbRefreshToken.Scope)
				assert.WithinDuration(t, dbRefreshToken.Expiry, time.Now().Add(db.RefreshTokenTime), 10*time.Second)

				permissions, err := app.models.Permissions.Get(validUser.ID)
				assert.NoError(t, err)
				assert.NotNil(t, permissions)
				assert.Len(t, *permissions, 2)
				assert.Contains(t, *permissions, db.PermissionWriteUser)
				assert.Contains(t, *permissions, db.PermissionReadUser)
			} else {
				var count int
				err := app.models.DB.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 0, count)

				user, err := app.models.Users.GetByUsername(validUser.Username)
				assert.NoError(t, err)

				permissions, err := app.models.Permissions.Get(user.ID)
				assert.NoError(t, err)
				assert.NotNil(t, permissions)
				assert.Len(t, *permissions, 2)

				err = app.models.DB.QueryRow("SELECT COUNT(*) FROM user_permissions").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 2, count)

			}

			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})

		})
	}
}

func TestRefreshAuthTokenHandler(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())

	pwd := "Test1234!"

	validUser := db.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Password: db.Password{
			Plain: &pwd,
		},
	}

	setup := func() (*db.Token, error) {
		if err := app.models.Users.Create(&validUser); err != nil {
			return nil, err
		}

		_, err := app.models.Tokens.CreateToken(validUser.ID, db.AuthTokenTime, db.TokenScopeAccess)
		if err != nil {
			return nil, err
		}

		dbRefreshToken, err := app.models.Tokens.CreateToken(validUser.ID, db.RefreshTokenTime, db.TokenScopeRefresh)
		if err != nil {
			return nil, err
		}

		return dbRefreshToken, nil
	}

	testCases := []struct {
		name       string
		payload    *tokenInput
		setup      func() (*db.Token, error)
		wantStatus int
		wantBody   envelope
	}{
		{
			name:       "Valid request",
			setup:      setup,
			wantStatus: http.StatusOK,
		},
		{
			name: "Send an access token",
			setup: func() (*db.Token, error) {
				if err := app.models.Users.Create(&validUser); err != nil {
					return nil, err
				}

				dbAuthToken, err := app.models.Tokens.CreateToken(validUser.ID, db.AuthTokenTime, db.TokenScopeAccess)
				if err != nil {
					return nil, err
				}

				_, err = app.models.Tokens.CreateToken(validUser.ID, db.RefreshTokenTime, db.TokenScopeRefresh)
				if err != nil {
					return nil, err
				}

				return dbAuthToken, nil
			},
			wantStatus: http.StatusUnauthorized,
			wantBody: envelope{
				"error": "unknown or invalid refresh token",
			},
		},
		{
			name: "Send an invalid token",
			payload: &tokenInput{
				Token: "invalidtoken",
			},
			setup:      setup,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"token": "must be 26 bytes long",
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var validToken *db.Token
			if tt.setup != nil {
				token, err := tt.setup()
				assert.NoError(t, err)
				validToken = token
			}

			if tt.payload == nil {
				tt.payload = &tokenInput{
					Token: validToken.Plain,
				}
			}

			status, _, body := ts.post(t, "/v1/tokens/refresh", tt.payload)

			assert.Equal(t, tt.wantStatus, status, "want %d; got %d", tt.wantStatus, status)

			if tt.wantBody == nil {
				tt.wantBody = body
			}

			assert.JSONEq(t, tt.wantBody.JSON(), body.JSON(), "want %s; got %s", tt.wantBody.JSON(), body.JSON())

			if tt.wantStatus == http.StatusOK {
				dbAccessToken, err := app.models.Tokens.Get(validUser.ID, db.TokenScopeAccess)
				assert.NoError(t, err)
				assert.Equal(t, validUser.ID, dbAccessToken.UserID)
				assert.Equal(t, db.TokenScopeAccess, dbAccessToken.Scope)
				assert.WithinDuration(t, dbAccessToken.Expiry, time.Now().Add(db.AuthTokenTime), 10*time.Second)

				dbRefreshToken, err := app.models.Tokens.Get(validUser.ID, db.TokenScopeRefresh)
				assert.NoError(t, err)
				assert.Equal(t, validUser.ID, dbRefreshToken.UserID)
				assert.Equal(t, db.TokenScopeRefresh, dbRefreshToken.Scope)
				assert.WithinDuration(t, dbRefreshToken.Expiry, time.Now().Add(db.RefreshTokenTime), 10*time.Second)
			} else {
				var count int
				err := app.models.DB.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 2, count)
			}

			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})
		})
	}
}

func TestDeleteAuthTokenHandler(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())

	pwd := "Test1234!"
	// Create a test user
	user := &db.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Password: db.Password{
			Plain: &pwd,
		},
	}

	setup := func() (*db.Token, error) {
		err := app.models.Users.Insert(user)
		if err != nil {
			return nil, err
		}

		accessToken, err := app.models.Tokens.CreateToken(user.ID, db.AuthTokenTime, db.TokenScopeAccess)
		if err != nil {
			return nil, err
		}

		_, err = app.models.Tokens.CreateToken(user.ID, db.RefreshTokenTime, db.TokenScopeRefresh)
		if err != nil {
			return nil, err
		}

		return accessToken, nil
	}

	testCases := []struct {
		name       string
		setup      func() (*db.Token, error)
		payload    *tokenInput
		wantStatus int
		wantBody   envelope
	}{
		{
			name:       "Valid request",
			setup:      setup,
			wantStatus: http.StatusOK,
			wantBody: envelope{
				"message": "user successfully logged out",
			},
		},
		{
			name:       "No token provided",
			wantStatus: http.StatusForbidden,
			setup:      setup,
			payload: &tokenInput{
				Token: "",
			},
			wantBody: envelope{
				"error": "invalid or missing authentication token",
			},
		},
		{
			name:       "Invalid token",
			wantStatus: http.StatusForbidden,
			setup:      setup,
			payload: &tokenInput{
				Token: "invalidtoken",
			},
			wantBody: envelope{
				"error": "invalid or missing authentication token",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var validToken *db.Token
			if tt.setup != nil {
				token, err := tt.setup()
				assert.NoError(t, err)
				validToken = token
			}

			if tt.payload != nil {
				validToken.Plain = tt.payload.Token
			}

			req, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/tokens", nil)
			assert.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+validToken.Plain)

			res, err := ts.Client().Do(req)
			assert.NoError(t, err)

			status, _, body := readResponse(t, res)

			assert.Equal(t, tt.wantStatus, status, "want %d; got %d", tt.wantStatus, status)

			if tt.wantBody == nil {
				tt.wantBody = body
			}

			assert.JSONEq(t, tt.wantBody.JSON(), body.JSON(), "want %s; got %s", tt.wantBody.JSON(), body.JSON())

			if tt.wantStatus == http.StatusOK {
				_, err := app.models.Tokens.Get(user.ID, db.TokenScopeAccess)
				assert.ErrorIs(t, err, db.ErrNotFound)

				_, err = app.models.Tokens.Get(user.ID, db.TokenScopeRefresh)
				assert.ErrorIs(t, err, db.ErrNotFound)
			} else {
				var count int
				err := app.models.DB.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 2, count)
			}

			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})
		})
	}
}

func TestRequestPasswordResetHandler(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())

	pwd := "Test1234!"

	validUser := db.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Password: db.Password{
			Plain: &pwd,
		},
	}

	setup := func() error {
		err := app.models.Users.Create(&validUser)
		if err != nil {
			return err
		}

		return nil
	}

	testCases := []struct {
		name       string
		payload    requestPwdResetInput
		setup      func() error
		wantStatus int
		wantBody   envelope
	}{
		{
			name: "valid payload",
			payload: requestPwdResetInput{
				Email: validUser.Email,
			},
			setup:      setup,
			wantStatus: http.StatusOK,
		}, {
			name: "invalid email",
			payload: requestPwdResetInput{
				Email: "testuser",
			},
			setup:      setup,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"email": "must be a valid email address",
				},
			},
		}, {
			name: "non-existent email",
			payload: requestPwdResetInput{
				Email: "testuser1@example.com",
			},
			setup:      setup,
			wantStatus: http.StatusUnauthorized,
			wantBody: envelope{
				"error": "invalid authentication credentials",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				err := tt.setup()
				assert.NoError(t, err)
			}

			status, _, body := ts.post(t, "/v1/users/password/reset", tt.payload)

			assert.Equal(t, tt.wantStatus, status, "want %d; got %d", tt.wantStatus, status)

			if tt.wantBody == nil {
				tt.wantBody = body
			}

			assert.JSONEq(t, tt.wantBody.JSON(), body.JSON(), "want %s; got %s", tt.wantBody.JSON(), body.JSON())

			if tt.wantStatus == http.StatusOK {
				dbToken, err := app.models.Tokens.Get(validUser.ID, db.TokenScopeResetPwd)
				assert.NoError(t, err)
				assert.Equal(t, validUser.ID, dbToken.UserID)
				assert.Equal(t, db.TokenScopeResetPwd, dbToken.Scope)
				assert.WithinDuration(t, dbToken.Expiry, time.Now().Add(db.ResetPwdTokenTime), 10*time.Second)
			} else {
				var count int
				err := app.models.DB.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			}

			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})
		})
	}
}

func TestUpdatePasswordHandler(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())

	pwd := "Test1234!"

	validUser := db.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Password: db.Password{
			Plain: &pwd,
		},
	}

	setup := func(expiration time.Duration) (*db.Token, error) {
		err := app.models.Users.Create(&validUser)
		if err != nil {
			return nil, err
		}

		token, err := app.models.Tokens.CreateToken(validUser.ID, expiration, db.TokenScopeResetPwd)
		if err != nil {
			return nil, err
		}

		return token, nil
	}

	testCases := []struct {
		name       string
		payload    updatePwdInput
		setup      func() (*db.Token, error)
		wantStatus int
		wantBody   envelope
	}{
		{
			name: "valid payload",
			setup: func() (*db.Token, error) {
				token, err := setup(db.ResetPwdTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			payload: updatePwdInput{
				Password: "NewPassword123!",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid password",
			setup: func() (*db.Token, error) {
				token, err := setup(db.ResetPwdTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			payload: updatePwdInput{
				Password: "test1234!",
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: envelope{
				"error": map[string]string{
					"password": "must be 8-72 characters long and contain at least one uppercase letter, one lowercase letter, one number, and one symbol",
				},
			},
		}, {
			name: "expired token",
			setup: func() (*db.Token, error) {
				token, err := setup(-1 * time.Second)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			payload: updatePwdInput{
				Password: "NewPassword123!",
			},
			wantStatus: http.StatusUnauthorized,
			wantBody: envelope{
				"error": "invalid authentication credentials",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				token, err := tt.setup()
				assert.NoError(t, err)
				tt.payload.Token = token.Plain
			}

			status, _, body := ts.put(t, "/v1/users/password/update", tt.payload)

			assert.Equal(t, tt.wantStatus, status, "want %d; got %d", tt.wantStatus, status)

			fmt.Printf("%s", body)
			if tt.wantBody == nil {
				tt.wantBody = body
			}

			assert.JSONEq(t, tt.wantBody.JSON(), body.JSON(), "want %s; got %s", tt.wantBody.JSON(), body.JSON())

			if tt.wantStatus == http.StatusOK {
				user, err := app.models.Users.GetByUsername(validUser.Username)
				assert.NoError(t, err)
				assert.Equal(t, 2, user.Version)
			}

			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})

		})
	}
}

func TestGetAccountHandler(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())

	pwd := "Test1234!"

	validUser := db.User{
		Username:  "testuser",
		Email:     "testuser@example.com",
		Activated: false,
		Password: db.Password{
			Plain: &pwd,
		},
		Version: 1,
	}

	setup := func(expiration time.Duration) (*db.Token, error) {
		err := app.models.Users.Create(&validUser)
		if err != nil {
			return nil, err
		}

		err = app.models.Users.Activate(validUser.ID)
		if err != nil {
			return nil, err
		}

		dbAuthToken, err := app.models.Tokens.CreateToken(validUser.ID, expiration, db.TokenScopeAccess)
		if err != nil {
			return nil, err
		}

		_, err = app.models.Tokens.CreateToken(validUser.ID, expiration, db.TokenScopeRefresh)
		if err != nil {
			return nil, err
		}

		err = app.models.Permissions.Add(validUser.ID, db.PermissionReadUser)
		if err != nil {
			return nil, err
		}

		return dbAuthToken, nil
	}

	testCases := []struct {
		name       string
		setup      func() (*db.Token, error)
		token      *string
		wantStatus int
		wantBody   envelope
		username   *string
	}{
		{
			name: "Valid request",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			wantStatus: http.StatusOK,
		}, {
			name: "No token provided",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			token:      nil,
			wantStatus: http.StatusForbidden,
		}, {
			name: "Invalid token",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			token:      strPtr("invalidtoken"),
			wantStatus: http.StatusForbidden,
		}, {
			name: "Invalid user access",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			wantStatus: http.StatusForbidden,
			username:   strPtr("testuser1"),
		}, {
			name: "Expired token",
			setup: func() (*db.Token, error) {
				token, err := setup(-1 * time.Second)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var token string
			if tt.token != nil {
				_, err := tt.setup()
				assert.NoError(t, err)
				token = *tt.token
			} else {
				dbToken, err := tt.setup()
				assert.NoError(t, err)
				token = dbToken.Plain
			}

			var username string
			if tt.username == nil {
				username = validUser.Username
			} else {
				username = *tt.username
			}
			req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/users/account/"+username, nil)
			assert.NoError(t, err)

			if tt.name != "No token provided" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			res, err := ts.Client().Do(req)
			assert.NoError(t, err)

			status, _, body := readResponse(t, res)
			assert.Equal(t, tt.wantStatus, status, "want %d; got %d", tt.wantStatus, status)
			if tt.wantBody == nil {
				tt.wantBody = body
			}

			assert.JSONEq(t, tt.wantBody.JSON(), body.JSON(), "want %s; got %s", tt.wantBody.JSON(), body.JSON())

			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})

		})
	}
}

func TestUpdateAccountHandler(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())

	validUser := db.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Password: db.Password{
			Plain: strPtr("Test1234!"),
		},
	}

	setup := func(expiration time.Duration) (*db.Token, error) {
		err := app.models.Users.Create(&validUser)
		if err != nil {
			return nil, err
		}

		err = app.models.Users.Activate(validUser.ID)
		if err != nil {
			return nil, err
		}

		dbAuthToken, err := app.models.Tokens.CreateToken(validUser.ID, expiration, db.TokenScopeAccess)
		if err != nil {
			return nil, err
		}

		_, err = app.models.Tokens.CreateToken(validUser.ID, expiration, db.TokenScopeRefresh)
		if err != nil {
			return nil, err
		}

		err = app.models.Permissions.Add(validUser.ID, db.PermissionWriteUser, db.PermissionReadUser)
		if err != nil {
			return nil, err
		}

		return dbAuthToken, nil
	}

	testCases := []struct {
		name       string
		payload    updateAccountInput
		setup      func() (*db.Token, error)
		wantStatus int
		wantBody   envelope
		token      *string
		username   *string
	}{
		{
			name: "Valid request",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			payload: updateAccountInput{
				Email:    "testuser1@example.com",
				Password: "Abcd1234!",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Password omitted from payload",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			payload: updateAccountInput{
				Email: "testuser1@example.com",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Email omitted from payload",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			payload: updateAccountInput{
				Password: "Abcd1234!",
			},
			wantStatus: http.StatusOK,
		}, {
			name: "Unauthorized access",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			username: strPtr("testuser1"),
			payload: updateAccountInput{
				Email:    "abcd@example.com",
				Password: "Abcd1234!",
			},
			wantStatus: http.StatusForbidden,
			wantBody: envelope{
				"error": "you do not have permission to perform this action",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var token string
			if tt.token != nil {
				_, err := tt.setup()
				assert.NoError(t, err)
				token = *tt.token
			} else {
				dbToken, err := tt.setup()
				assert.NoError(t, err)
				token = dbToken.Plain
			}

			var username string
			if tt.username == nil {
				username = validUser.Username
			} else {
				username = *tt.username
			}

			jsonPayload, err := json.Marshal(tt.payload)
			assert.NoError(t, err)

			req, err := http.NewRequest(http.MethodPut, ts.URL+"/v1/users/account/"+username+"/update", bytes.NewReader(jsonPayload))
			assert.NoError(t, err)

			if tt.name != "No token provided" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			res, err := ts.Client().Do(req)
			assert.NoError(t, err)

			status, _, body := readResponse(t, res)
			assert.Equal(t, tt.wantStatus, status, "want %d; got %d", tt.wantStatus, status)

			if tt.wantBody == nil {
				tt.wantBody = body
			}

			assert.JSONEq(t, tt.wantBody.JSON(), body.JSON(), "want %s; got %s", tt.wantBody.JSON(), body.JSON())

			if tt.wantStatus == http.StatusOK {
				user, err := app.models.Users.GetByUsername(username)
				assert.NoError(t, err)

				if tt.payload.Email != "" {
					assert.Equal(t, tt.payload.Email, user.Email)
				}

				if tt.payload.Password != "" {
					match, err := user.Password.Compare(tt.payload.Password)
					assert.NoError(t, err)

					assert.True(t, match)
				}

				assert.Equal(t, 2, user.Version)
			}

			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})
		})
	}
}
