package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/store/cache"
)

func (app *application) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	userProfile, err := app.store.Users.FlattenUser(r.Context(), user.User)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	account, err := app.store.Users.GetUserAccountByUserID(r.Context(), userProfile.ID)

	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	response := envelope{
		"user": userProfile,
	}

	if account != nil {
		response["account"] = account
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getUser(ctx context.Context, userID string) (*AuthInfo, error) {
	var (
		user *store.User
		err  error
	)

	authInfo := &AuthInfo{}

	if app.cfg.redisCfg.enabled {
		user, err = app.cacheStore.Users.Get(ctx, userID)

		if !(err == nil || errors.Is(err, store.ErrRecordNotFound)) {
			app.logger.Errorf("Error fetching user from cache: %v", err)
			return nil, err
		}

		if user != nil {
			app.logger.Infow("cache hit", "key", "user", "id", userID)
			authInfo.User = user
			return authInfo, nil
		}
	}

	user, err = app.store.Users.GetByID(ctx, userID)

	if err != nil {
		return nil, fmt.Errorf("error fetching user from database: %w", err)
	}

	app.logger.Infof("fetched user %v from the database", userID)
	err = app.cacheStore.Users.Set(ctx, user)

	if err != nil {
		return nil, err
	}

	authInfo.User = user
	return authInfo, nil
}

func (app *application) getNormalUsers(w http.ResponseWriter, r *http.Request) {
	fq := store.PaginateQueryFilter{
		Page:         1,
		PageSize:     20,
		Sort:         "created_at",
		SortSafelist: []string{"created_at", "-created_at"},
	}

	if err := fq.Parse(r); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	users, metadata, err := app.store.Users.GetNormalUsers(r.Context(), fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"users":    users,
		"metadata": metadata,
	})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,nefield=OldPassword"`
}

type changePassword2faPayload struct {
	NewPassword string
	Email       string
}

func (app *application) changePassword(w http.ResponseWriter, r *http.Request) {
	var (
		form changePasswordRequest
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

	if !app.withPasswordAccess(r, form.OldPassword) {
		app.forbiddenResponse(w, r, "the provided old password is incorrect")
		return
	}

	if app.withPasswordAccess(r, form.NewPassword) {
		app.forbiddenResponse(w, r, "the new password cannot be the same as the old password")
		return
	}

	if user.User.TwoFactorAuthEnabled {

		payload, err := json.Marshal(&changePassword2faPayload{
			Email:       user.Email,
			NewPassword: form.NewPassword,
		})

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		token, err := app.cacheStore.Tokens.New(user.ID, time.Minute*30, cache.ChangePassword2faTokenScope, payload)

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		err = app.cacheStore.Tokens.Insert(r.Context(), token)

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		app.required2faCodeResponse(w, r, token.Plaintext)
		return
	}

	if err := user.User.Password.Set(form.NewPassword); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.store.Users.ChangePassword(r.Context(), user.User); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"message": "your password has been changed successfully",
	}
	app.successResponse(w, http.StatusOK, response)
}

type verifyChangePassword2faRequest struct {
	MfaToken string `json:"mfa_token" validate:"required"`
	Code     string `json:"code" validate:"min=6,max=6"`
}

func (app *application) verifyChangePassword2fa(w http.ResponseWriter, r *http.Request) {
	var (
		form verifyChangePassword2faRequest
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

	token, err := app.cacheStore.Tokens.Get(r.Context(), cache.ChangePassword2faTokenScope, form.MfaToken)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "token invalid or expired")

		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	var payload changePassword2faPayload

	if err := json.Unmarshal(token.Data, &payload); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if user.Email != payload.Email {
		app.notFoundResponse(w, r, "token invalid or expired")
		return
	}

	if err := user.User.Password.Set(payload.NewPassword); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.store.Users.ChangePassword(r.Context(), user.User); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"message": "your password has been changed successfully",
	}

	app.successResponse(w, http.StatusOK, response)
}
