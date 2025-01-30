package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/store/cache"
	"github.com/devphaseX/buyr-api.git/worker"
)

type createAdminForm struct {
	FirstName string `form:"first_name" validate:"required,min=1,max=255"`
	LastName  string `form:"last_name" validate:"required,min=1,max=255"`
	Email     string `form:"email" validate:"required,email"`
}

func (app *application) createAdmin(w http.ResponseWriter, r *http.Request) {
	// user := getUserFromCtx(r)

	var form createAdminForm

	if err := app.decodeForm(r, &form, 10<<10); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	photo, photoHeader, err := r.FormFile("photo")

	if !(err == nil || errors.Is(err, http.ErrMissingFile)) {
		app.badRequestResponse(w, r, errors.New("photo is required and must be a valid image file"))
		return
	}

	var photoURL string

	if err == nil {
		defer photo.Close()
		if !isImage(photoHeader) {
			app.badRequestResponse(w, r, errors.New("only image files (JPEG, PNG, GIF) are allowed for the photo"))
			return
		}

		photoURL, err = app.fileobject.UploadFile(r.Context(), app.cfg.supabaseConfig.profileImageBucketName, photoHeader.Filename, photo)

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	newAdminUser := &store.AdminUser{
		FirstName:  form.FirstName,
		LastName:   form.LastName,
		AdminLevel: store.AdminLevelNone,
		User: store.User{
			Email:               form.Email,
			Role:                store.AdminRole,
			ForcePasswordChange: true,
			AvatarURL:           photoURL,
		},
	}

	// Save the new vendor user to the database
	if err := app.store.Users.CreateAdminUser(r.Context(), newAdminUser); err != nil {
		switch {
		case errors.Is(err, store.ErrDuplicateEmail):
			app.conflictResponse(w, r, "the email address is already in use. Please use a different email.")
		default:
			app.serverErrorResponse(w, r, fmt.Errorf("failed to create admin user: %v", err))
		}
		return
	}

	token, err := app.cacheStore.Tokens.New(newAdminUser.UserID, time.Hour*24*7, cache.ScopeActivation, nil)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.cacheStore.Tokens.Insert(r.Context(), token)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.taskDistributor.DistributeTaskSendAdminOnboardEmail(r.Context(),
		&worker.PayloadSendAdminOnboardEmail{
			Username:  fmt.Sprintf("%s %s", newAdminUser.FirstName, newAdminUser.LastName),
			Token:     token.Plaintext,
			Email:     newAdminUser.User.Email,
			ClientURL: app.cfg.clientURL,
		}, nil)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Return a success response
	app.successResponse(w, http.StatusCreated, envelope{
		"message": "adminstration account created successfully",
	})
}

func (app *application) getAdminUsers(w http.ResponseWriter, r *http.Request) {
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

	users, metadata, err := app.store.Users.GetAdminUsers(r.Context(), fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"users":    users,
		"metadata": metadata,
	})
}
