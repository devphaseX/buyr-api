package main

import (
	"fmt"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

func (app *application) setup2fa(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	// Validate the user role
	if user.Role != store.UserRole && user.Role != store.AdminRole && user.Role != store.VendorRole {
		app.forbiddenResponse(w, r, "2FA is not available for this user role")
		return
	}
	// Flatten the user to get role-specific details
	flattenUser, err := app.store.Users.FlattenUser(r.Context(), user)
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

	err := app.store.Users.EnableTwoFactorAuth(r.Context(), user.ID, form.Secret)

	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"message": "mfa setup completed",
	})
}
