package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/store/cache"
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/hibiken/asynq"
)

type registerUserForm struct {
	FirstName string `json:"first_name" validate:"required,min=1,max=255"`
	LastName  string `json:"last_name" validate:"required,min=1,max=255"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8,max=20"`
}

func (app *application) registerNormalUser(w http.ResponseWriter, r *http.Request) {
	var form registerUserForm
	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &store.NormalUser{
		FirstName: form.FirstName,
		LastName:  form.LastName,
		User: store.User{
			Email: form.Email,
			Role:  "user",
		},
	}

	if err := user.User.Password.Set(form.Password); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err := app.store.Users.CreateNormalUser(r.Context(), user)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrDuplicateEmail):
			app.conflictResponse(w, r, err.Error())
			return

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	activationToken, err := app.cacheStore.Tokens.New(
		user.UserID,
		time.Hour*24*3,
		cache.ScopeActivation,
	)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.cacheStore.Tokens.Insert(r.Context(), activationToken)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	asynqOpts := []asynq.Option{
		asynq.MaxRetry(10),
		asynq.ProcessIn(time.Second * 10),
		asynq.Queue(worker.QueueCritical),
	}

	err = app.taskDistributor.DistributeTaskSendActivateAccountEmail(r.Context(),
		&worker.PayloadSendActivateAcctEmail{
			UserID:    user.User.ID,
			Username:  fmt.Sprintf("%s %s", user.FirstName, user.LastName),
			Email:     user.User.Email,
			ClientURL: app.cfg.clientURL,
			Token:     activationToken.Plaintext,
		}, asynqOpts...)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusCreated, nil)
}

func (app *application) activateUser(w http.ResponseWriter, r *http.Request) {
	tokenKey := app.readStringID(r, "token")
	userID := r.URL.Query().Get("userId")

	if userID == "" {
		app.unauthorizedResponse(w, r, "expired or invalid token")
		return
	}

	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.ScopeActivation, userID, tokenKey)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if token == nil {
		app.unauthorizedResponse(w, r, "expired or invalid token")
		return
	}

	user, err := app.store.Users.GetByID(r.Context(), token.UserID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.unauthorizedResponse(w, r, "expired or invalid token")

		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	if user.EmailVerifiedAt != nil {
		app.conflictResponse(w, r, "user account activated already")
		return
	}

	err = app.store.Users.SetUserAccountAsActivate(r.Context(), user)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.cacheStore.Tokens.DeleteAllForUser(r.Context(), cache.ScopeActivation, userID)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, nil)
}

type signInForm struct {
	Email      string `json:"email" validate:"email"`
	Password   string `json:"password" validate:"min=1,max=255"`
	RememberMe bool   `json:"remember_me"`
}

func (app *application) signIn(w http.ResponseWriter, r *http.Request) {
	var form signInForm

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user, err := app.store.Users.GetByEmail(r.Context(), form.Email)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	match, err := user.Password.Matches(form.Password)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	var (
		sessionExpiry     = app.cfg.authConfig.RefreshTokenTTL
		accessTokenExpiry = app.cfg.authConfig.AccessTokenTTL
	)
	session := &store.Session{
		UserID:     user.ID,
		IP:         r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Version:    1,
		RememberMe: form.RememberMe,
	}

	if form.RememberMe {
		sessionExpiry = app.cfg.authConfig.RememberMeTTL
		session.MaxRenewalDuration = time.Now().AddDate(0, 6, 0).Unix() //6 months
	}

	session.ExpiresAt = time.Now().Add(sessionExpiry)

	err = app.store.Sessions.Create(r.Context(), session)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	accessToken, err := app.authToken.GenerateAccessToken(user.ID, session.ID, app.cfg.authConfig.AccessTokenTTL)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	refreshToken, err := app.authToken.GenerateRefreshToken(session.ID, session.Version, sessionExpiry)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"access_token":         accessToken,
		"access_token_expiry":  time.Now().Add(accessTokenExpiry),
		"refresh_token":        refreshToken,
		"refresh_token_expiry": time.Now().Add(sessionExpiry),
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func (app *application) refreshToken(w http.ResponseWriter, r *http.Request) {
	var form refreshRequest
	// Try to get the refresh token from the cookie
	refreshTokenCookie, err := r.Cookie("sid")
	if err == nil && refreshTokenCookie != nil && strings.TrimSpace(refreshTokenCookie.Value) != "" {
		form.RefreshToken = refreshTokenCookie.Value
	} else {
		// If the cookie is not available, try to get the refresh token from the JSON body
		if err := app.readJSON(w, r, &form); err != nil {
			app.badRequestResponse(w, r, errors.New("missing refresh token"))
			return
		}

	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, errors.New("missing refresh token"))
		return
	}

	// Validate the refresh token
	claims, err := app.authToken.ValidateRefreshToken(form.RefreshToken)
	// fmt.Printf("claim: %+v", claims)

	if err != nil {
		app.unauthorizedResponse(w, r, "invalid refresh token")
		return
	}
	// Validate the session
	session, user, canExtend, err := app.store.Sessions.ValidateSession(r.Context(), claims.SessionID, claims.Version)
	if err != nil || session == nil {
		app.unauthorizedResponse(w, r, "invalid session")
		return
	}

	// Generate a new access token
	accessToken, err := app.authToken.GenerateAccessToken(user.ID, session.ID, app.cfg.authConfig.AccessTokenTTL)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	var (
		newRefreshToken string
		rememberPeriod  = app.cfg.authConfig.RefreshTokenTTL
	)

	if session.RememberMe {
		rememberPeriod = app.cfg.authConfig.RememberMeTTL
	}

	if canExtend {
		newRefreshToken, err = app.store.Sessions.ExtendSessionAndGenerateRefreshToken(r.Context(), session, app.authToken, rememberPeriod)
		if err != nil {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to extend session: %v", err))
			return
		}
	}
	// Return the new access token
	response := envelope{
		"access_token":            accessToken,
		"access_token_expires_in": time.Now().Add(app.cfg.authConfig.AccessTokenTTL),
	}
	if newRefreshToken != "" {
		response["refresh_token"] = newRefreshToken
		response["refresh_token_expires_in"] = time.Now().Add(rememberPeriod)
	}

	app.successResponse(w, http.StatusOK, response)
}
