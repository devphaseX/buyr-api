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
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
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

	asynqOpts := []asynq.Option{
		asynq.MaxRetry(10),
		asynq.ProcessIn(time.Second * 10),
		asynq.Queue(worker.QueueCritical),
	}

	app.taskDistributor.DistributeTaskSendActivateAccountEmail(r.Context(), &worker.PayloadSendActivateAcctEmail{
		Username: fmt.Sprintf("%s %s", user.FirstName, user.LastName),
		Token:    activationToken.Plaintext,
	}, asynqOpts...)

	app.logger.Info("activation token", activationToken)
}
