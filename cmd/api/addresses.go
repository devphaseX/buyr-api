package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

type createUserAddressRequest struct {
	FirstName     string            `json:"first_name" validate:"required,max=100"`
	LastName      string            `json:"last_name" validate:"required,max=100"`
	PhoneNumber   string            `json:"phone_number" validate:"required,e164"`
	AddressType   store.AddressType `json:"address_type" validate:"required,oneof=billing shipping"`
	StreetAddress string            `json:"street_address" validate:"required,max=255"`
	City          string            `json:"city" validate:"required,max=100"`
	State         string            `json:"state" validate:"required,max=100"`
	PostalCode    string            `json:"postal_code" validate:"required,max=20"`
	Country       string            `json:"country" validate:"required,max=100"`
	IsDefault     bool              `json:"is_default"`
}

func (app *application) createUserAddress(w http.ResponseWriter, r *http.Request) {
	var (
		user = getUserFromCtx(r)
		form createUserAddressRequest
	)

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	address := &store.Address{
		FirstName:     form.FirstName,
		LastName:      form.LastName,
		PhoneNumber:   form.PhoneNumber,
		UserID:        user.ID,
		AddressType:   form.AddressType,
		StreetAddress: form.StreetAddress,
		City:          form.City,
		State:         form.State,
		PostalCode:    form.PostalCode,
		Country:       form.Country,
		IsDefault:     form.IsDefault,
	}

	if err := app.store.Address.Create(r.Context(), address); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"address": address,
		"message": "Address created successfully",
	}

	app.successResponse(w, http.StatusCreated, response)
}

func (app *application) getUserAddresses(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	addresses, err := app.store.Address.GetByUserID(r.Context(), user.ID)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"addresses": addresses,
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getUserAddressByID(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	addressID := app.readStringID(r, "addressID")

	address, err := app.store.Address.GetByID(r.Context(), addressID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "address not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if address.UserID != user.ID {
		app.notFoundResponse(w, r, "address not found")
		return
	}

	response := envelope{
		"address": address,
	}

	app.successResponse(w, http.StatusOK, response)
}

type setDefaultAddressRequest struct {
	AddressID   string            `json:"address_id" validate:"required"`
	AddressType store.AddressType `json:"address_type" validate:"required,oneof=billing shipping"`
}

func (app *application) setDefaultAddress(w http.ResponseWriter, r *http.Request) {
	var request setDefaultAddressRequest

	user := getUserFromCtx(r)
	if err := app.readJSON(w, r, &request); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := app.store.Address.SetDefault(r.Context(), user.ID, request.AddressID, request.AddressType); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"message": "Default address updated successfully",
		"id":      request.AddressID,
	}

	app.successResponse(w, http.StatusOK, response)
}
