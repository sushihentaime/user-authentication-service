package main

import (
	"net/http"
	"runtime/debug"
)

func (app *application) logError(r *http.Request, err error) {
	var (
		method = r.Method
		url    = r.URL.RequestURI()
		errMsg = err.Error()
		debug  = debug.Stack()
	)

	app.logger.Error(errMsg, "method", method, "url", url, "stack", string(debug))
}

func (app *application) writeErrorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	err := app.writeJSON(w, status, envelope{"error": message}, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)
	// message := "the server encountered a problem and could not process your request"
	app.writeErrorResponse(w, r, http.StatusInternalServerError, err)
}

func (app *application) badRequestErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.writeErrorResponse(w, r, http.StatusBadRequest, err.Error())
}

func (app *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	app.writeErrorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

func (app *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	message := "invalid authentication credentials"
	app.writeErrorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) invalidAuthenticationTokenResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")

	message := "invalid or missing authentication token"
	app.writeErrorResponse(w, r, http.StatusForbidden, message)
}

func (app *application) unauthorizedActionResponse(w http.ResponseWriter, r *http.Request) {
	message := "you do not have permission to perform this action"
	app.writeErrorResponse(w, r, http.StatusForbidden, message)
}

func (app *application) invalidRefreshTokenResponse(w http.ResponseWriter, r *http.Request) {
	message := "unknown or invalid refresh token"
	app.writeErrorResponse(w, r, http.StatusUnauthorized, message)
}
