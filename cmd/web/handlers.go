package main

import (
	"errors"
	"net/http"

	"github.com/sushihentaime/user-management-service/internal/db"
	"github.com/sushihentaime/user-management-service/pkg/jsonParser"
)

type createUserInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenInput struct {
	Token string `json:"token"`
}

type loginUserInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type requestPwdResetInput struct {
	Email string `json:"email"`
}

type updatePwdInput struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (app *application) createUserHandler(w http.ResponseWriter, r *http.Request) {
	var input createUserInput

	err := jsonParser.ParseJSON(w, r, &input)
	if err != nil {
		app.badRequestErrorResponse(w, r, err)
		return
	}

	user := &db.User{
		Username: input.Username,
		Email:    input.Email,
		Password: db.Password{
			Plain: &input.Password,
		},
	}

	if user.ValidateUser(); !user.Validator.Valid() {
		app.failedValidationResponse(w, r, user.Validator.Errors)
		return
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	tx, err := app.models.DB.Begin()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer tx.Rollback()

	err = app.models.Users.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrDuplicateUsername):
			user.Validator.AddError("username", "a user with this username already exists")
			app.failedValidationResponse(w, r, user.Validator.Errors)
		case errors.Is(err, db.ErrDuplicateEmail):
			user.Validator.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, user.Validator.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Permissions.Add(user.ID, db.PermissionReadUser)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	token, err := app.models.Tokens.CreateToken(user.ID, db.ActivationTokenTime, db.TokenScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.backgroundTask(func() {
		data := map[string]any{
			"activationToken": token.Plain,
		}

		err = app.mailer.Send(user.Email, "mail.html", data)
		if err != nil {
			app.logger.Error(err.Error())
		}

		app.logger.Info("email sent", "email", user.Email, "type", "activation")
	})

	err = app.writeJSON(w, http.StatusCreated, envelope{"token": token.Plain}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input tokenInput

	err := jsonParser.ParseJSON(w, r, &input)
	if err != nil {
		app.badRequestErrorResponse(w, r, err)
		return
	}

	token := &db.Token{
		Plain: input.Token,
	}

	if token.ValidateToken(); !token.Validator.Valid() {
		app.failedValidationResponse(w, r, token.Validator.Errors)
		return
	}

	tokenHash := db.HashToken(token.Plain)

	tx, err := app.models.DB.Begin()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer tx.Rollback()

	user, err := app.models.Users.GetToken(db.TokenScopeActivation, tokenHash)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			app.failedValidationResponse(w, r, map[string]string{"token": "invalid or expired activation token"})
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Users.Activate(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.Permissions.Add(user.ID, db.PermissionWriteUser)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.Tokens.Delete(user.ID, db.TokenScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "user account successfully activated"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

// create access token and refresh token
func (app *application) createAuthTokenHandler(w http.ResponseWriter, r *http.Request) {
	var input loginUserInput

	err := jsonParser.ParseJSON(w, r, &input)
	if err != nil {
		app.badRequestErrorResponse(w, r, err)
		return
	}

	user := &db.User{
		Username: input.Username,
		Password: db.Password{
			Plain: &input.Password,
		},
	}

	if user.ValidateLoginUser(); !user.Validator.Valid() {
		app.failedValidationResponse(w, r, user.Validator.Errors)
		return
	}

	dbUser, err := app.models.Users.GetByUsername(input.Username)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	match, err := dbUser.Password.Compare(input.Password)
	if err != nil {
		app.invalidCredentialsResponse(w, r)
		return
	}

	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	tx, err := app.models.DB.Begin()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer tx.Rollback()

	dbAuthToken, err := app.models.Tokens.Get(dbUser.ID, db.TokenScopeAccess)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	dbRefreshToken, err := app.models.Tokens.Get(dbUser.ID, db.TokenScopeRefresh)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
		default:
			app.serverErrorResponse(w, r, err)
			return
		}

	}

	if dbAuthToken != nil || dbRefreshToken != nil {
		err = app.models.Tokens.Delete(dbUser.ID, db.TokenScopeAccess)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		err = app.models.Tokens.Delete(dbUser.ID, db.TokenScopeRefresh)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	authToken, err := app.models.Tokens.CreateToken(dbUser.ID, db.AuthTokenTime, db.TokenScopeAccess)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	refreshToken, err := app.models.Tokens.CreateToken(dbUser.ID, db.RefreshTokenTime, db.TokenScopeRefresh)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"access_token": map[string]any{
		"token": authToken.Plain, "expiry": authToken.Expiry}, "refresh_token": map[string]any{
		"token": refreshToken.Plain, "expiry": refreshToken.Expiry}}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

// uses refresh token to create a new access token and refresh token
func (app *application) refreshAuthTokenHandler(w http.ResponseWriter, r *http.Request) {
	var input tokenInput

	err := jsonParser.ParseJSON(w, r, &input)
	if err != nil {
		app.invalidCredentialsResponse(w, r)
		return
	}

	token := &db.Token{
		Plain: input.Token,
	}

	if token.ValidateToken(); !token.Validator.Valid() {
		app.failedValidationResponse(w, r, token.Validator.Errors)
		return
	}

	tokenHash := db.HashToken(token.Plain)

	user, err := app.models.Users.GetToken(db.TokenScopeRefresh, tokenHash)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			app.invalidRefreshTokenResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	tx, err := app.models.DB.Begin()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer tx.Rollback()

	err = app.models.Tokens.Delete(user.ID, db.TokenScopeRefresh)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.Tokens.Delete(user.ID, db.TokenScopeAccess)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	newAccessToken, err := app.models.Tokens.CreateToken(user.ID, db.AuthTokenTime, db.TokenScopeAccess)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	newRefreshToken, err := app.models.Tokens.CreateToken(user.ID, db.RefreshTokenTime, db.TokenScopeRefresh)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"access_token": map[string]any{
		"token": newAccessToken.Plain, "expiry": newAccessToken.Expiry}, "refresh_token": map[string]any{"token": newRefreshToken.Plain, "expiry": newRefreshToken.Expiry}}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

// logout user by deleting access token and refresh token
func (app *application) deleteAuthTokenHandler(w http.ResponseWriter, r *http.Request) {
	user := app.getUserContext(r)

	tx, err := app.models.DB.Begin()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer tx.Rollback()

	err = app.models.Tokens.Delete(user.ID, db.TokenScopeAccess)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.Tokens.Delete(user.ID, db.TokenScopeRefresh)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "user successfully logged out"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) requestPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	var input requestPwdResetInput

	err := jsonParser.ParseJSON(w, r, &input)
	if err != nil {
		app.badRequestErrorResponse(w, r, err)
		return
	}

	dbUser := &db.User{
		Email: input.Email,
	}

	if dbUser.ValidateEmail(); !dbUser.Validator.Valid() {
		app.failedValidationResponse(w, r, dbUser.Validator.Errors)
		return
	}

	user, err := app.models.Users.GetByEmail(dbUser.Email)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	prevToken, err := app.models.Tokens.Get(user.ID, db.TokenScopeResetPwd)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	if prevToken != nil {
		err = app.models.Tokens.Delete(user.ID, db.TokenScopeResetPwd)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	token, err := app.models.Tokens.CreateToken(user.ID, db.ResetPwdTokenTime, db.TokenScopeResetPwd)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.backgroundTask(func() {
		data := map[string]any{
			"email":              user.Email,
			"resetPasswordToken": token.Plain,
		}

		err = app.mailer.Send(user.Email, "reset_pwd.html", data)
		if err != nil {
			app.logger.Error(err.Error())
		}

		app.logger.Info("email sent", "email", user.Email, "type", "reset pwd")
	})

	err = app.writeJSON(w, http.StatusOK, envelope{"token": token.Plain}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) updatePasswordHandler(w http.ResponseWriter, r *http.Request) {
	var input updatePwdInput

	err := jsonParser.ParseJSON(w, r, &input)
	if err != nil {
		app.badRequestErrorResponse(w, r, err)
		return
	}

	token := &db.Token{
		Plain: input.Token,
	}

	user := &db.User{
		Password: db.Password{
			Plain: &input.Password,
		},
	}

	if token.ValidateToken(); !token.Validator.Valid() {
		app.failedValidationResponse(w, r, token.Validator.Errors)
		return
	}

	if user.ValidatePassword(); !user.Validator.Valid() {
		app.failedValidationResponse(w, r, user.Validator.Errors)
		return
	}

	tokenHash := db.HashToken(token.Plain)

	tokenUser, err := app.models.Users.GetToken(db.TokenScopeResetPwd, tokenHash)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	dbUser, err := app.models.Users.GetByUsername(tokenUser.Username)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = dbUser.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	tx, err := app.models.DB.Begin()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer tx.Rollback()

	err = app.models.Users.Update(dbUser)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Tokens.Delete(dbUser.ID, db.TokenScopeResetPwd)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "password successfully updated"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) getAccountHandler(w http.ResponseWriter, r *http.Request) {
	userParam, err := app.readStringParam(r, "username")
	if err != nil {
		app.badRequestErrorResponse(w, r, err)
		return
	}

	user := app.getUserContext(r)
	if user.Username != *userParam {
		app.unauthorizedActionResponse(w, r)
		return
	}

	dbUser, err := app.models.Users.GetByUsername(user.Username)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			app.invalidAuthenticationTokenResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": dbUser}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

type updateAccountInput struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
}

// only allow user to update their account's email and password
func (app *application) updateAccountHandler(w http.ResponseWriter, r *http.Request) {
	var input updateAccountInput

	userParam, err := app.readStringParam(r, "username")
	if err != nil {
		app.badRequestErrorResponse(w, r, err)
		return
	}

	user := app.getUserContext(r)
	if user.Username != *userParam {
		app.unauthorizedActionResponse(w, r)
		return
	}

	err = jsonParser.ParseJSON(w, r, &input)
	if err != nil {
		app.badRequestErrorResponse(w, r, err)
		return
	}

	inputUser := &db.User{
		Email: input.Email,
		Password: db.Password{
			Plain: &input.Password,
		},
	}

	if inputUser.ValidateUpdateUser(); !inputUser.Validator.Valid() {
		app.failedValidationResponse(w, r, inputUser.Validator.Errors)
		return
	}

	dbUser, err := app.models.Users.GetByUsername(user.Username)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			app.invalidAuthenticationTokenResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if input.Password != "" {
		err = dbUser.Password.Set(input.Password)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return

		}
	}

	if inputUser.Email != "" {
		dbUser.Email = inputUser.Email
		dbUser.Activated = false
	}

	tx, err := app.models.DB.Begin()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer tx.Rollback()

	err = app.models.Users.Update(dbUser)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrDuplicateUsername):
			dbUser.Validator.AddError("username", "a user with this username already exists")
			app.failedValidationResponse(w, r, dbUser.Validator.Errors)
		case errors.Is(err, db.ErrDuplicateEmail):
			dbUser.Validator.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, dbUser.Validator.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	token, err := app.models.Tokens.Get(dbUser.ID, db.TokenScopeActivation)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	if token != nil {
		err = app.models.Tokens.Delete(dbUser.ID, db.TokenScopeActivation)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	newToken, err := app.models.Tokens.CreateToken(dbUser.ID, db.ActivationTokenTime, db.TokenScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.backgroundTask(func() {
		data := map[string]any{
			"activationToken": newToken.Plain,
		}

		err = app.mailer.Send(dbUser.Email, "mail.html", data)
		if err != nil {
			app.logger.Error(err.Error())
		}

		app.logger.Info("email sent", "email", dbUser.Email, "type", "reset pwd")
	})

	err = app.writeJSON(w, http.StatusOK, envelope{"user": dbUser}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	err := app.writeJSON(w, http.StatusOK, envelope{"status": "available"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}
