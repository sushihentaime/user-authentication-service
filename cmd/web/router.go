package main

import (
	"net/http"

	"github.com/sushihentaime/user-management-service/internal/db"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	standard := alice.New(app.authenticate)

	router.HandlerFunc(http.MethodGet, "/health", app.healthCheckHandler)

	router.HandlerFunc(http.MethodPost, "/v1/users/new", adaptHandler(standard.ThenFunc(app.createUserHandler)))
	router.HandlerFunc(http.MethodPut, "/v1/users/activate", adaptHandler(standard.ThenFunc(app.activateUserHandler)))
	router.HandlerFunc(http.MethodPost, "/v1/users/authenticate", adaptHandler(standard.ThenFunc(app.createAuthTokenHandler)))
	router.HandlerFunc(http.MethodPost, "/v1/tokens/refresh", app.refreshAuthTokenHandler)
	router.HandlerFunc(http.MethodDelete, "/v1/tokens", adaptHandler(standard.ThenFunc(app.deleteAuthTokenHandler)))
	router.HandlerFunc(http.MethodPost, "/v1/users/password/reset", adaptHandler(standard.ThenFunc(app.requestPasswordResetHandler)))
	router.HandlerFunc(http.MethodPut, "/v1/users/password/update", adaptHandler(standard.ThenFunc(app.updatePasswordHandler)))
	router.HandlerFunc(http.MethodGet, "/v1/users/account/:username", adaptHandler(standard.ThenFunc(app.requirePermission(app.getAccountHandler, db.PermissionReadUser))))
	router.HandlerFunc(http.MethodPut, "/v1/users/account/:username/update", adaptHandler(standard.ThenFunc(app.requirePermission(app.updateAccountHandler, db.PermissionWriteUser, db.PermissionReadUser))))

	return app.recoverPanic(app.logRequest(router))
}

func adaptHandler(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}
}
