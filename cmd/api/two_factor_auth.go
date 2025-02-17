package main

import (
	"fmt"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/encrypt"
	"github.com/devphaseX/buyr-api.git/internal/store"
)

func (app *application) setup2fa(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	if user.TwoFactorAuthEnabled {
		app.forbiddenResponse(w, r, "2fa setup already complete")
		return
	}

	// Flatten the user to get role-specific details
	flattenUser, err := app.store.Users.FlattenUser(r.Context(), user.User)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to flatten user: %w", err))
		return
	}

	// Determine the user's name based on their role
	var userName string
	switch flattenUser.Role {
	case store.UserRole, store.AdminRole:
		userName = fmt.Sprintf("%s %s", flattenUser.FirstName, flattenUser.LastName)
	case store.VendorRole:
		userName = flattenUser.BusinessName
	default:
		app.serverErrorResponse(w, r, fmt.Errorf("invalid user role: %s", flattenUser.Role))
		return
	}

	// Generate the TOTP secret and QR code
	secret, qr, err := app.totp.GenerateSecret(app.cfg.authConfig.totpIssuerName, userName, 256)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to generate TOTP secret: %w", err))
		return
	}

	// Return the secret and QR code in the response
	app.successResponse(w, http.StatusOK, envelope{
		"secret": secret,
		"qr":     qr,
	})

}

type verify2faSetupForm struct {
	Code   string `json:"code" validate:"min=6,max=6"`
	Secret string `json:"secret" validate:"required"`
}

func (app *application) verify2faSetup(w http.ResponseWriter, r *http.Request) {
	var (
		form verify2faSetupForm
		user = getUserFromCtx(r)
	)

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if !app.totp.VerifyCode(form.Secret, form.Code) {
		app.unauthorizedResponse(w, r, "invalid code")
		return
	}

	encryptedSecret, err := encrypt.EncryptSecret(form.Secret, app.cfg.encryptConfig.masterSecretKey)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	recoveryCodes, err := encrypt.GenerateRecoveryCodes(10, 10)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	encryptedCodes, err := encrypt.EncryptRecoveryCodes(recoveryCodes, app.cfg.encryptConfig.masterSecretKey)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.store.Users.EnableTwoFactorAuth(r.Context(), user.ID, encryptedSecret, encryptedCodes)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"message": "mfa setup completed",
	})
}

type viewRecoveryCodesForm struct {
	Password string `json:"password" validate:"required"`
}

func (app *application) viewRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	var (
		form viewRecoveryCodesForm
		user = getUserFromCtx(r)
	)

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if match := app.withPasswordAccess(r, form.Password); !match {
		app.forbiddenResponse(w, r, "password not a match")
		return
	}

	recoveryCodes, err := encrypt.DecryptRecoveryCodes(user.RecoveryCodes, app.cfg.encryptConfig.masterSecretKey)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"recovery_codes": recoveryCodes,
	})

}

func (app *application) resetRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	if !user.TwoFactorAuthEnabled {
		app.forbiddenResponse(w, r, "2fa not enabled")
		return
	}

	recoveryCodes, err := encrypt.GenerateRecoveryCodes(10, 10)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	encryptedCodes, err := encrypt.EncryptRecoveryCodes(recoveryCodes, app.cfg.encryptConfig.masterSecretKey)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.store.Users.ResetRecoveryCodes(r.Context(), user.ID, encryptedCodes); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"reovery_codes": recoveryCodes,
		"message":       "recovery code resetted successfully",
	})

}
