package main

import (
	"errors"
	"fmt"
	"net/http"
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
