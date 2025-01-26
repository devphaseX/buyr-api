package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/encrypt"
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
		nil,
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
		asynq.MaxRetry(3),
		asynq.Queue(worker.QueueCritical),
	}

	err = app.taskDistributor.DistributeTaskSendActivateAccountEmail(r.Context(),
		&worker.PayloadSendActivateAcctEmail{
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

	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.ScopeActivation, tokenKey)

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

	err = app.cacheStore.Tokens.DeleteAllForUser(r.Context(), cache.ScopeActivation, user.ID)

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

	if user.TwoFactorAuthEnabled {
		token, err := app.cacheStore.Tokens.New(
			user.ID,
			time.Hour*4,
			cache.Require2faConfirmation,
			nil,
		)

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		err = app.cacheStore.Tokens.Insert(r.Context(), token)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		app.successResponse(w, http.StatusOK, envelope{
			"mfa_enabled":    true,
			"mfa_auth_token": token.Plaintext,
		})
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

	app.setAuthCookiesAndRespond(w,
		accessToken,
		accessTokenExpiry,
		refreshToken,
		sessionExpiry,
	)
}

type verify2FAForm struct {
	MfaToken string `json:"mfa_token" validate:"required"`
	MfaCode  string `json:"mfa_code" validate:"required,min=6,max=6"` // Assuming 6-digit codes
}

func (app *application) verify2FA(w http.ResponseWriter, r *http.Request) {
	var form verify2FAForm

	// Parse and validate the request body
	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Fetch the token from the cache
	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.Require2faConfirmation, form.MfaToken)
	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired 2FA token")
		return
	}

	// Verify the token scope
	if token.Scope != cache.Require2faConfirmation {
		app.unauthorizedResponse(w, r, "invalid 2FA token")
		return
	}

	// Fetch the user
	user, err := app.store.Users.GetByID(r.Context(), token.UserID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	secret, err := encrypt.DecryptSecret(user.AuthSecret, app.cfg.encryptConfig.masterSecretKey)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Verify the 2FA code (e.g., using a TOTP library)
	if !app.totp.VerifyCode(secret, form.MfaCode) {
		app.unauthorizedResponse(w, r, "invalid 2FA code")
		return
	}

	// Delete the token after successful verification
	if err := app.cacheStore.Tokens.DeleteAllForUser(r.Context(), user.ID, form.MfaToken); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Create a new session
	sessionExpiry := app.cfg.authConfig.RefreshTokenTTL
	session := &store.Session{
		UserID:    user.ID,
		IP:        r.RemoteAddr,
		UserAgent: r.UserAgent(),
		Version:   1,
		ExpiresAt: time.Now().Add(sessionExpiry),
	}

	// Save the session
	if err := app.store.Sessions.Create(r.Context(), session); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Generate access and refresh tokens
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

	// Set cookies and respond
	app.setAuthCookiesAndRespond(w, accessToken, app.cfg.authConfig.AccessTokenTTL, refreshToken, sessionExpiry)
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

	app.setAuthCookiesAndRespond(
		w,
		accessToken,
		app.cfg.authConfig.AccessTokenTTL,
		newRefreshToken,
		rememberPeriod,
	)
}

type forgetPasswordForm struct {
	Email string `json:"email"`
}

type forgetPassword2faPayload struct {
	EmailVerify        bool
	Email              string
	EnableTwoFactor    bool
	TwoVerifyConfirmed bool
}

func (app *application) forgetPassword(w http.ResponseWriter, r *http.Request) {
	var form forgetPasswordForm

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
		app.successResponse(w, http.StatusOK, "")
		return
	}

	payload, err := json.Marshal(forgetPassword2faPayload{
		Email:           form.Email,
		EnableTwoFactor: user.TwoFactorAuthEnabled,
	})

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	token, err := app.cacheStore.Tokens.New(user.ID, time.Hour*4, cache.ForgetPassword, payload)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.cacheStore.Tokens.Insert(r.Context(), token)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	asynqOpts := []asynq.Option{
		asynq.MaxRetry(3),
		asynq.Queue(worker.QueueCritical),
	}

	err = app.taskDistributor.DistributeTaskSendRecoverAccountEmail(r.Context(), &worker.PayloadSendRecoverAccountEmail{
		Email:     user.Email,
		ClientURL: app.cfg.clientURL,
		Token:     token.Plaintext,
	}, asynqOpts...)

	app.successResponse(w, http.StatusNoContent, envelope{
		"message": "A reset link has being sent to your email",
	})
}

type confirmForgetPasswordTokenForm struct {
	Token string `json:"token" validate:"required"`
}

func (app *application) confirmForgetPasswordToken(w http.ResponseWriter, r *http.Request) {
	var form confirmForgetPasswordTokenForm

	// Parse and validate the request body
	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Fetch the token from the cache
	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.ForgetPassword, form.Token)
	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired token")
		return
	}

	// Verify the token scope
	if token.Scope != cache.ForgetPassword {
		app.unauthorizedResponse(w, r, "invalid token scope")
		return
	}

	// Unmarshal the payload
	var payload forgetPassword2faPayload
	if err := json.Unmarshal(token.Data, &payload); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Update the payload to set EmailVerify to true
	payload.EmailVerify = true

	// Marshal the updated payload
	updatedPayload, err := json.Marshal(payload)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	token.Plaintext = form.Token

	// Update the token with the new payload
	token.Data = updatedPayload
	err = app.cacheStore.Tokens.Insert(r.Context(), token)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// If 2FA is enabled, return a 2FA token
	if payload.EnableTwoFactor {

		// Respond with the 2FA token
		app.successResponse(w, http.StatusOK, envelope{
			"message":        "Token confirmed successfully. 2FA verification required.",
			"mfa_enabled":    true,
			"mfa_auth_token": form.Token,
		})
		return
	}

	// If 2FA is not enabled, respond with success
	app.successResponse(w, http.StatusOK, envelope{
		"message": "Token confirmed successfully. Please reset your password.",
	})
}

func (app *application) verifyForgetPassword2fa(w http.ResponseWriter, r *http.Request) {
	var form verify2FAForm

	// Parse and validate the request body
	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Fetch the 2FA token from the cache
	mfaToken, err := app.cacheStore.Tokens.Get(r.Context(), cache.ForgetPassword, form.MfaToken)
	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired 2FA token")
		return
	}

	// Verify the token scope
	if mfaToken.Scope != cache.ForgetPassword {
		app.unauthorizedResponse(w, r, "invalid 2FA token scope")
		return
	}

	// Unmarshal the payload
	var payload forgetPassword2faPayload
	if err := json.Unmarshal(mfaToken.Data, &payload); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !payload.EmailVerify {
		app.unauthorizedResponse(w, r, "complete your email verification")
		return

	}

	if !payload.EnableTwoFactor {
		app.unauthorizedResponse(w, r, "two factor not enabled")
		return
	}
	// Update the payload to set EmailVerify to true
	payload.TwoVerifyConfirmed = true

	// Marshal the updated payload
	updatedPayload, err := json.Marshal(payload)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Update the token with the new payload
	mfaToken.Data = updatedPayload
	err = app.cacheStore.Tokens.Insert(r.Context(), mfaToken)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Fetch the user
	user, err := app.store.Users.GetByID(r.Context(), mfaToken.UserID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	secret, err := encrypt.DecryptSecret(user.AuthSecret, app.cfg.encryptConfig.masterSecretKey)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Verify the 2FA code (e.g., using a TOTP library)
	if !app.totp.VerifyCode(secret, form.MfaCode) {
		app.unauthorizedResponse(w, r, "invalid 2FA code")
		return
	}

	// Respond with success
	app.successResponse(w, http.StatusOK, envelope{
		"message": "2FA verification successful. Please reset your password.",
	})
}

type resetPasswordForm struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,min=8,max=255"`
}

func (app *application) resetPassword(w http.ResponseWriter, r *http.Request) {
	var form resetPasswordForm

	// Parse and validate the request body
	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Fetch the token from the cache
	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.ForgetPassword, form.Token)

	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired token")
		return
	}

	// Verify the token scope
	if token.Scope != cache.ForgetPassword {
		app.unauthorizedResponse(w, r, "invalid token scope")
		return
	}

	// Unmarshal the payload
	var payload forgetPassword2faPayload
	if err := json.Unmarshal(token.Data, &payload); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Verify that the email has been confirmed
	if !payload.EmailVerify {
		app.unauthorizedResponse(w, r, "email not verified")
		return
	}

	if payload.EnableTwoFactor && !payload.TwoVerifyConfirmed {
		// Verify that the two factor has been confirmed for two factor enabled account
		app.unauthorizedResponse(w, r, "two factor not verified")
		return
	}

	// Fetch the user by email
	user, err := app.store.Users.GetByEmail(r.Context(), payload.Email)
	_ = user
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// // Update the user's password
	err = app.store.Users.UpdatePassword(r.Context(), user, form.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Delete the token after successful password reset
	err = app.cacheStore.Tokens.DeleteAllForUser(r.Context(), cache.ForgetPassword, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with success
	app.successResponse(w, http.StatusOK, envelope{
		"message": "Password reset successfully.",
	})
}
