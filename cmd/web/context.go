package main

import (
	"context"
	"net/http"

	"github.com/sushihentaime/user-management-service/internal/db"
)

type contextKey string

const userContextKey contextKey = "user"

func (app *application) createUserContext(r *http.Request, user *db.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func (app *application) getUserContext(r *http.Request) *db.User {
	user, ok := r.Context().Value(userContextKey).(*db.User)
	if !ok {
		return nil
	}
	return user
}
