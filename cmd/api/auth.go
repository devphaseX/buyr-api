package main

import (
	"context"
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
	"github.com/justinas/nosurf"
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

	err = app.sendAccountActivationEmail(r.Context(), &user.User)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusCreated, nil)
}

func (app *application) sendAccountActivationEmail(ctx context.Context, user *store.User) error {
	// Create new activation token
	activationToken, err := app.cacheStore.Tokens.New(
		user.ID,
		time.Hour*24*3,
		cache.ActivationTokenScope,
		nil,
	)

	if err != nil {
		return fmt.Errorf("creating activation token: %w", err)
	}

	err = app.cacheStore.Tokens.Insert(ctx, activationToken)
	if err != nil {
		return fmt.Errorf("inserting activation token: %w", err)
	}

	flattenUser, err := app.store.Users.FlattenUser(ctx, user)
	if err != nil {
		return fmt.Errorf("flattening user: %w", err)
	}

	// Distribute email task
	asynqOpts := []asynq.Option{
		asynq.MaxRetry(3),
		asynq.Queue(worker.QueueCritical),
	}

	switch user.Role {
	case store.UserRole:
		err = app.taskDistributor.DistributeTaskSendActivateAccountEmail(ctx,
			&worker.PayloadSendActivateAcctEmail{
				Username:  fmt.Sprintf("%s %s", flattenUser.FirstName, flattenUser.LastName),
				Email:     user.Email,
				ClientURL: app.cfg.clientURL,
				Token:     activationToken.Plaintext,
			}, asynqOpts...)
	case store.VendorRole:
		err = app.taskDistributor.DistributeTaskSendVendorActivationEmail(ctx,
			&worker.PayloadSendVendorActivationEmail{
				Username:  flattenUser.BusinessName,
				Token:     activationToken.Plaintext,
				Email:     user.Email,
				ClientURL: app.cfg.clientURL,
			}, asynqOpts...)
	case store.AdminRole:
		err = app.taskDistributor.DistributeTaskSendAdminOnboardEmail(ctx,
			&worker.PayloadSendAdminOnboardEmail{
				Username:  fmt.Sprintf("%s %s", flattenUser.FirstName, flattenUser.LastName),
				Token:     activationToken.Plaintext,
				Email:     user.Email,
				ClientURL: app.cfg.clientURL,
			}, asynqOpts...)
	default:
		return fmt.Errorf("unsupported user role: %v", user.Role)
	}

	if err != nil {
		return fmt.Errorf("distributing activation email task: %w", err)
	}

	return nil
}

func (app *application) activateUser(w http.ResponseWriter, r *http.Request) {
	tokenKey := r.URL.Query().Get("token")

	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.ActivationTokenScope, tokenKey)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.unauthorizedResponse(w, r, "invalid or expired activation token")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if token == nil {
		app.unauthorizedResponse(w, r, "invalid or expired activation token")
		return
	}

	user, err := app.store.Users.GetByID(r.Context(), token.UserID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.unauthorizedResponse(w, r, "invalid or expired activation token")
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

	err = app.cacheStore.Tokens.DeleteAllForUser(r.Context(), cache.ActivationTokenScope, user.ID)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if user.ForcePasswordChange {
		payload, err := json.Marshal(forgetPassword2faPayload{
			Email: user.Email,
		})

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		fogetPasswordToken, err := app.cacheStore.Tokens.New(user.ID, time.Hour*4, cache.ForgetPasswordTokenScope, payload)

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		err = app.cacheStore.Tokens.Insert(r.Context(), fogetPasswordToken)

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		response := envelope{
			"message":               "Account activated successfully. Password reset required.",
			"reset_token":           fogetPasswordToken.Plaintext,
			"force_password_change": true,
		}
		app.successResponse(w, http.StatusOK, response)
		return
	}

	response := envelope{
		"message": "Account activated successfully.",
	}

	app.successResponse(w, http.StatusOK, response)
}

type signInForm struct {
	Email      string `json:"email" validate:"email"`
	Password   string `json:"password" validate:"min=1,max=255"`
	RememberMe bool   `json:"remember_me"`
}

type signIn2faPayload struct {
	RememberMe bool
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
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if user.Password.IsSetPasswordEmpty() {
		account, err := app.store.Users.GetUserAccountByUserID(r.Context(), user.ID)

		if !(err == nil || errors.Is(err, store.ErrRecordNotFound)) {
			app.serverErrorResponse(w, r, err)
			return
		}

		if account != nil {
			app.errorResponse(w, http.StatusUnprocessableEntity,
				fmt.Sprintf("This account was registered via %s. Please log in using %s.", account.Provider, account.Provider),
				envelope{
					"code":     ErrorCodeOAuthAccount,
					"provider": account.Provider,
				})
			return
		}

		app.errorResponse(w, http.StatusUnprocessableEntity,
			errors.New("Your account requires a password reset. Please use the 'Forgot Password' feature to reset your password."),
			envelope{
				"code": ErrorPasswordResetRequired,
			})
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

	if user.EmailVerifiedAt == nil {
		err = app.sendAccountActivationEmail(r.Context(), user)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		app.forbiddenResponse(w, r, "Account not activated. A new activation email has been sent to your email address.")
		return
	}

	if user.TwoFactorAuthEnabled {
		payload, _ := json.Marshal(signIn2faPayload{RememberMe: form.RememberMe})
		token, err := app.cacheStore.Tokens.New(
			user.ID,
			time.Hour*4,
			cache.Login2faTokenScope,
			payload,
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

	app.createUserSessionAndSetCookies(w, r, user, form.RememberMe)
}

type verify2FAForm struct {
	MfaToken string `json:"mfa_token" validate:"required"`
	MfaCode  string `json:"mfa_code" validate:"required,min=6,max=6"` // Assuming 6-digit codes
}

func (app *application) verifyLogin2FA(w http.ResponseWriter, r *http.Request) {
	var (
		form             verify2FAForm
		signin2faPayload *signIn2faPayload
	)

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
	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.Login2faTokenScope, form.MfaToken)
	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired 2FA token")
		return
	}

	// Verify the token scope
	if token.Scope != cache.Login2faTokenScope {
		app.unauthorizedResponse(w, r, "invalid 2FA token")
		return
	}

	if err := json.Unmarshal(token.Data, &signin2faPayload); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Fetch the user
	user, err := app.getUser(r.Context(), token.UserID)
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

	err = app.cacheStore.Tokens.DeleteAllForUser(r.Context(), cache.Login2faTokenScope, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.createUserSessionAndSetCookies(w, r, user.User, signin2faPayload.RememberMe)
}

type verifyLogin2faRecoveryCodeForm struct {
	MfaToken     string `json:"mfa_token" validate:"required"`
	RecoveryCode string `json:"recovery_code" validate:"required,min=10,max=10"` // Assuming 6-digit codes
}

func (app *application) verifyLogin2faRecoveryCode(w http.ResponseWriter, r *http.Request) {
	var (
		form             verifyLogin2faRecoveryCodeForm
		signin2faPayload *signIn2faPayload
	)

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.Login2faTokenScope, form.MfaToken)
	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired 2FA token")
		return
	}

	if token.Scope != cache.Login2faTokenScope {
		app.unauthorizedResponse(w, r, "invalid 2FA token")
		return
	}

	if err := json.Unmarshal(token.Data, &signin2faPayload); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	user, err := app.getUser(r.Context(), token.UserID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	recoveryCodes, err := encrypt.DecryptRecoveryCodes(user.RecoveryCodes, app.cfg.encryptConfig.masterSecretKey)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	validRecoveryCode := false
	for _, code := range recoveryCodes {
		if code == form.RecoveryCode {
			validRecoveryCode = true
			break
		}
	}

	if !validRecoveryCode {
		app.unauthorizedResponse(w, r, "invalid recovery code")
		return
	}

	err = app.store.Users.DisableTwoFactorAuth(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.cacheStore.Tokens.DeleteAllForUser(r.Context(), cache.Login2faTokenScope, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.createUserSessionAndSetCookies(w, r, user.User, signin2faPayload.RememberMe)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func (app *application) refreshToken(w http.ResponseWriter, r *http.Request) {
	var form refreshRequest
	refreshTokenCookie, err := r.Cookie("sid")

	if err == nil && refreshTokenCookie != nil && strings.TrimSpace(refreshTokenCookie.Value) != "" {
		form.RefreshToken = refreshTokenCookie.Value
	} else {

		if err := app.readJSON(w, r, &form); err != nil {
			app.badRequestResponse(w, r, errors.New("missing refresh token"))
			return
		}

	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, errors.New("missing refresh token"))
		return
	}

	claims, err := app.authToken.ValidateRefreshToken(form.RefreshToken)

	if err != nil {
		app.unauthorizedResponse(w, r, "invalid refresh token")
		return
	}

	session, user, canExtend, err := app.store.Sessions.ValidateSession(r.Context(), claims.SessionID, claims.Version)

	if err != nil || session == nil || user.Version != session.Version {
		app.unauthorizedResponse(w, r, "invalid session")
		return
	}

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

	// Check if the user registered via OAuth (password not set)
	if user.Password.IsSetPasswordEmpty() {
		account, err := app.store.Users.GetUserAccountByUserID(r.Context(), user.ID)

		if err != nil {
			if !errors.Is(err, store.ErrRecordNotFound) {
				app.serverErrorResponse(w, r, err)
				return
			}
		}

		if account != nil {
			app.errorResponse(w, http.StatusUnprocessableEntity,
				"This account was registered via "+account.Provider+" OAuth. Please sign in using your "+account.Provider+" account.",
				envelope{
					"code":     ErrorCodeOAuthAccount,
					"provider": account.Provider,
				},
			)
			return
		}
	}

	payload, err := json.Marshal(forgetPassword2faPayload{
		Email:           form.Email,
		EnableTwoFactor: user.TwoFactorAuthEnabled,
	})

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	token, err := app.cacheStore.Tokens.New(user.ID, time.Hour*4, cache.ForgetPasswordTokenScope, payload)

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
	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.ForgetPasswordTokenScope, form.Token)
	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired token")
		return
	}

	// Verify the token scope
	if token.Scope != cache.ForgetPasswordTokenScope {
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

	if payload.EnableTwoFactor {

		app.successResponse(w, http.StatusOK, envelope{
			"message":        "Token confirmed successfully. 2FA verification required.",
			"mfa_enabled":    true,
			"mfa_auth_token": form.Token,
		})
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"message": "Token confirmed successfully. Please reset your password.",
	})
}

func (app *application) verifyForgetPassword2fa(w http.ResponseWriter, r *http.Request) {
	var form verify2FAForm

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	mfaToken, err := app.cacheStore.Tokens.Get(r.Context(), cache.ForgetPasswordTokenScope, form.MfaToken)
	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired 2FA token")
		return
	}

	if mfaToken.Scope != cache.ForgetPasswordTokenScope {
		app.unauthorizedResponse(w, r, "invalid 2FA token scope")
		return
	}

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

	if !app.totp.VerifyCode(secret, form.MfaCode) {
		app.unauthorizedResponse(w, r, "invalid 2FA code")
		return
	}

	payload.TwoVerifyConfirmed = true

	updatedPayload, err := json.Marshal(payload)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	mfaToken.Data = updatedPayload
	err = app.cacheStore.Tokens.Insert(r.Context(), mfaToken)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"message": "2FA verification successful. Please reset your password.",
	})
}

type verifyForgetPasswordRecoveryCodeForm struct {
	MfaToken     string `json:"mfa_token" validate:"required"`
	RecoveryCode string `json:"recovery_code" validate:"required,min=10,max=10"` // Assuming 6-digit codes
}

func (app *application) verifyForgetPasswordRecoveryCode(w http.ResponseWriter, r *http.Request) {
	var form verifyForgetPasswordRecoveryCodeForm

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
	mfaToken, err := app.cacheStore.Tokens.Get(r.Context(), cache.ForgetPasswordTokenScope, form.MfaToken)
	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired 2FA token")
		return
	}

	// Verify the token scope
	if mfaToken.Scope != cache.ForgetPasswordTokenScope {
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

	// Fetch the user
	user, err := app.getUser(r.Context(), mfaToken.UserID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Decrypt recovery codes
	recoveryCodes, err := encrypt.DecryptRecoveryCodes(user.RecoveryCodes, app.cfg.encryptConfig.masterSecretKey)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Verify the recovery code
	validRecoveryCode := false
	for _, code := range recoveryCodes {
		if code == form.RecoveryCode {
			validRecoveryCode = true
			break
		}
	}

	if !validRecoveryCode {
		app.unauthorizedResponse(w, r, "invalid recovery code")
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

	err = app.store.Users.DisableTwoFactorAuth(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
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

	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.ForgetPasswordTokenScope, form.Token)

	if err != nil {
		app.unauthorizedResponse(w, r, "invalid or expired token")
		return
	}

	if token.Scope != cache.ForgetPasswordTokenScope {
		app.unauthorizedResponse(w, r, "invalid token scope")
		return
	}

	var payload forgetPassword2faPayload
	if err := json.Unmarshal(token.Data, &payload); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !payload.EmailVerify {
		app.unauthorizedResponse(w, r, "email not verified")
		return
	}

	if payload.EnableTwoFactor && !payload.TwoVerifyConfirmed {
		app.unauthorizedResponse(w, r, "two factor not verified")
		return
	}

	user, err := app.store.Users.GetByEmail(r.Context(), payload.Email)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.store.Users.UpdatePassword(r.Context(), user, form.Password)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.cacheStore.Tokens.DeleteAllForUser(r.Context(), cache.ForgetPasswordTokenScope, user.ID)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"message": "Password reset successfully.",
	})
}

func (app *application) getCSRFToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-CSRF-Token", nosurf.Token(r))
	w.WriteHeader(http.StatusOK)
}
