package main

import (
	"net/http"
)

func (app *application) setup2fa(w http.ResponseWriter, r *http.Request) {
	_ = getUserFromCtx(r)

	var accountName string = "test"
	secret, qr, err := app.totp.GenerateSecret(app.cfg.authConfig.totpIssuerName, accountName, 256)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

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
