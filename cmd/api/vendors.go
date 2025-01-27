package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/encrypt"
	"github.com/devphaseX/buyr-api.git/internal/store"
)

func (app *application) getVendorUsers(w http.ResponseWriter, r *http.Request) {
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

	users, metadata, err := app.store.Users.GetVendorUsers(r.Context(), fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"users":    users,
		"metadata": metadata,
	})
}

type createVendorForm struct {
	BusinessName    string `form:"business_name" validate:"min=1,max=255"`
	BusinessAddress string `form:"business_address" validate:"min=1,max=500"`
	Email           string `form:"email" validate:"email"`
	ContactNumber   string `form:"contact_number" validate:"min=10,max=13"`
	City            string `form:"city" validate:"min=1,max=50"`
	Country         string `form:"country" validate:"min=1,max=50"`
}

func (app *application) createVendor(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	var form createVendorForm

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

	newVendorUser := &store.VendorUser{
		BusinessName:     form.BusinessName,
		BusinessAddress:  form.BusinessAddress,
		ContactNumber:    form.ContactNumber,
		CreatedByAdminID: user.AdminUser.ID,
		City:             form.City,
		Country:          form.Country,
		User: store.User{
			Email:     form.Email,
			Role:      store.VendorRole,
			AvatarURL: photoURL,
		},
	}

	genPassword, err := encrypt.GenerateRandomString(8)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to generate password: %v", err))
		return
	}

	newVendorUser.User.Password.Set(genPassword)

	fmt.Println(genPassword)

	// Save the new vendor user to the database
	if err := app.store.Users.CreateVendorUser(r.Context(), newVendorUser); err != nil {
		switch {
		case errors.Is(err, store.ErrDuplicateEmail):
			app.conflictResponse(w, r, "the email address is already in use. Please use a different email.")
		default:
			app.serverErrorResponse(w, r, fmt.Errorf("failed to create vendor user: %v", err))
		}
		return
	}

	// Return a success response
	app.successResponse(w, http.StatusCreated, envelope{
		"message": "Vendor created successfully",
	})
}
