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

type changeAdminRoleRequest struct {
	Level store.AdminLevel `json:"level" validate:"required"`
}

func (app *application) changeAdminRole(w http.ResponseWriter, r *http.Request) {
	currentAdminUser := getUserFromCtx(r)
	memberID := app.readStringID(r, "memberID")
	var form changeAdminRoleRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	adminUser, err := app.store.Users.GetAdminUserByID(r.Context(), memberID)

	if err != nil {
		app.notFoundResponse(w, r, "admin user not found")
		return
	}

	// 1. Ensure the current admin is not trying to change their own role.
	if currentAdminUser.AdminUser.ID == adminUser.ID {
		app.forbiddenResponse(w, r, "you cannot change your own role")
		return
	}

	// 2. Ensure the current admin has a higher or equal level than the target admin.
	if !currentAdminUser.AdminUser.AdminLevel.CanModifyAdminLevel(adminUser.AdminLevel) {
		app.forbiddenResponse(w, r, "you are not authorized to change this admin's role")
		return
	}

	// 3.Super admin specific checks
	if adminUser.AdminLevel == store.AdminLevelSuper {
		if currentAdminUser.AdminUser.AdminLevel != store.AdminLevelSuper ||
			currentAdminUser.AdminUser.CreatedAt.After(adminUser.CreatedAt) {
			app.forbiddenResponse(w, r, "you are not authorized to change a super admin's role")
			return
		}
	}

	// 4.Ensure new level isn't higher than current admin's level
	if form.Level.GetRank() > currentAdminUser.AdminUser.AdminLevel.GetRank() {
		app.forbiddenResponse(w, r, "cannot set admin level higher than your own")
		return
	}

	if err := app.store.Users.ChangeAdminLevel(r.Context(), memberID, form.Level); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		if err := app.store.AuditLogs.LogEvent(r.Context(), store.AuditEvent{
			EventType:   store.ChangeRoleAuditEventType,
			AccountID:   adminUser.User.ID,
			PerformedBy: currentAdminUser.AdminUser.ID,
			Timestamp:   time.Now().UTC(),
			IPAddress:   r.RemoteAddr,
			UserAgent:   r.UserAgent(),
		}); err != nil {
			app.logger.Error("failed to log audit event", "error", err)
		}
	})

	response := envelope{
		"message": "admin role updated successfully",
		"data": map[string]interface{}{
			"admin_id":  memberID,
			"new_level": form.Level,
		},
	}
	app.successResponse(w, http.StatusOK, response)
}

type disableAdminAccountRequest struct {
	Reason string `json:"reason" validate:"required"`
}

func (app *application) disableAdminAccount(w http.ResponseWriter, r *http.Request) {
	currentAdminUser := getUserFromCtx(r)
	memberID := app.readStringID(r, "memberID")
	var form disableAdminAccountRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	adminUser, err := app.store.Users.GetAdminUserByID(r.Context(), memberID)

	if err != nil {
		app.notFoundResponse(w, r, "admin user not found")
		return
	}

	// 1. Ensure the current admin is not trying to change their own role.
	if currentAdminUser.AdminUser.ID == adminUser.ID {
		app.forbiddenResponse(w, r, "you cannot disable yourself")
		return
	}

	if !currentAdminUser.AdminUser.AdminLevel.CanModifyAdminLevel(adminUser.AdminLevel) {
		app.forbiddenResponse(w, r, "you cannot disable this admin account")
		return
	}

	if adminUser.AdminLevel == store.AdminLevelSuper {
		if currentAdminUser.AdminUser.AdminLevel != store.AdminLevelSuper ||
			currentAdminUser.AdminUser.CreatedAt.After(adminUser.CreatedAt) {
			app.forbiddenResponse(w, r, "you cannot disable this admin account")
			return
		}
	}

	err = app.store.Users.DisableUser(r.Context(), adminUser.UserID)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		if err := app.store.AuditLogs.LogEvent(r.Context(), store.AuditEvent{
			EventType:   store.AccountDisableAuditEventType,
			AccountID:   adminUser.User.ID,
			Reason:      form.Reason,
			PerformedBy: currentAdminUser.AdminUser.ID,
			Timestamp:   time.Now().UTC(),
			IPAddress:   r.RemoteAddr,
			UserAgent:   r.UserAgent(),
		}); err != nil {
			app.logger.Error("failed to log audit event", "error", err)
		}
	})

	response := envelope{
		"message": "user account disabled successfully",
		"data": map[string]interface{}{
			"admin_id": memberID,
			// "new_level": form.Level,
		},
	}
	app.successResponse(w, http.StatusOK, response)
}

func (app *application) enableAdminAccount(w http.ResponseWriter, r *http.Request) {
	currentAdminUser := getUserFromCtx(r)
	memberID := app.readStringID(r, "memberID")
	var form changeAdminRoleRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	adminUser, err := app.store.Users.GetAdminUserByID(r.Context(), memberID)

	if err != nil {
		app.notFoundResponse(w, r, "admin user not found")
		return
	}

	// 1. Ensure the current admin is not trying to change their own role.
	if currentAdminUser.AdminUser.ID == adminUser.ID {
		app.serverErrorResponse(w, r, errors.New("you cannot enable yourself"))
		return
	}

	if !currentAdminUser.AdminUser.AdminLevel.CanModifyAdminLevel(adminUser.AdminLevel) {
		app.forbiddenResponse(w, r, "you cannot enable this admin account")
		return
	}

	err = app.store.Users.EnableUser(r.Context(), adminUser.UserID)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		if err := app.store.AuditLogs.LogEvent(r.Context(), store.AuditEvent{
			EventType:   store.AccountEnabledAuditEventType,
			PerformedBy: currentAdminUser.AdminUser.ID,
			Timestamp:   time.Now().UTC(),
			IPAddress:   r.RemoteAddr,
			UserAgent:   r.UserAgent(),
		}); err != nil {
			app.logger.Error("failed to log audit event", "error", err)
		}
	})

	response := envelope{
		"message": "user account enabled successfully",
		"data": map[string]interface{}{
			"admin_id":  memberID,
			"new_level": form.Level,
		},
	}
	app.successResponse(w, http.StatusOK, response)
}
