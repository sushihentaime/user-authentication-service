package main

import (
	"fmt"
	"net/http"

	"github.com/sushihentaime/user-management-service/internal/db"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Authorization")

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			r = app.createUserContext(r, db.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		token := app.extractTokenFromHeader(authHeader)
		if token == "" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		dbToken := &db.Token{Plain: token}
		if dbToken.ValidateToken(); !dbToken.Validator.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		user, err := app.models.Users.GetToken(db.TokenScopeAccess, db.HashToken(dbToken.Plain))
		if err != nil {
			switch {
			case err == db.ErrNotFound:
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		r = app.createUserContext(r, user)
		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAuthUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.getUserContext(r)
		if user.IsAnonymous() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireActivatedUser(next http.Handler) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.getUserContext(r)
		if !user.Activated {
			app.unauthorizedActionResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})

	return app.requireAuthUser(fn)
}

func (app *application) requirePermission(next http.HandlerFunc, permissions ...db.Permission) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.getUserContext(r)

		userPermissions, err := app.models.Permissions.Get(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		for _, permission := range permissions {
			if !userPermissions.Include(permission) {
				app.unauthorizedActionResponse(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})

	return app.requireActivatedUser(fn)
}

func (app *application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			ip     = r.RemoteAddr
			proto  = r.Proto
			method = r.Method
			uri    = r.URL.RequestURI()
		)

		app.logger.Info("request from", "remote_addr", ip, "proto", proto, "method", method, "uri", uri)

		next.ServeHTTP(w, r)
	})
}
