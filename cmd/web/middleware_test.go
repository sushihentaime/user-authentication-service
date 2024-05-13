package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sushihentaime/user-management-service/internal/db"

	"github.com/stretchr/testify/assert"
)

func TestRecoverPanic(t *testing.T) {
	app := newTestApplication(t)

	// Create a mock HTTP handler
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a panic
		panic("Something went wrong")
	})

	// Call the recoverPanic middleware with the mock handler
	handler := app.recoverPanic(mockHandler)

	// Create a mock HTTP request and response
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Serve the request using the middleware
	handler.ServeHTTP(rec, req)

	// Verify that the panic was recovered and an error response was sent
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, but got %d", http.StatusInternalServerError, rec.Code)
	}
}
func TestAuthenticate(t *testing.T) {
	app := newTestApplication(t)

	pwd := "Test1234!"

	validUser := &db.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		Password: db.Password{
			Plain: &pwd,
		},
	}

	setup := func(expiration time.Duration) (*db.Token, error) {
		err := app.models.Users.Create(validUser)
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

		return dbAuthToken, nil
	}

	testCases := []struct {
		name         string
		setup        func() (*db.Token, error)
		token        *string
		expectedUser *db.User
		wantStatus   int
	}{
		{
			name: "No authentication header",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			token:        nil,
			expectedUser: db.AnonymousUser,
			wantStatus:   http.StatusOK,
		}, {
			name: "Invalid authentication token",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			token:        strPtr("invalid_token"),
			expectedUser: db.AnonymousUser,
			wantStatus:   http.StatusForbidden,
		}, {
			name: "Valid authentication token",
			setup: func() (*db.Token, error) {
				token, err := setup(db.AuthTokenTime)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			expectedUser: validUser,
			wantStatus:   http.StatusOK,
		}, {
			name: "Expired authentication token",
			setup: func() (*db.Token, error) {
				token, err := setup(-time.Second)
				if err != nil {
					return nil, err
				}
				return token, nil
			},
			expectedUser: db.AnonymousUser,
			wantStatus:   http.StatusForbidden,
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

			mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user := app.getUserContext(r)
				if user.ID != tt.expectedUser.ID || user.Username != tt.expectedUser.Username || user.Email != tt.expectedUser.Email {
					t.Errorf("Expected user to be %v, but got %v", tt.expectedUser, user)
				}
			})

			handler := app.authenticate(mockHandler)

			req := httptest.NewRequest(http.MethodGet, "/", nil)

			if tt.name != "No authentication header" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			t.Cleanup(func() {
				err := cleanup(app)
				assert.NoError(t, err)
			})
		})
	}
}
