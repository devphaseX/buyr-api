package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

type createOptionRequest struct {
	Name          string `json:"name" validate:"min=1,max=255"`
	DisplayName   string `json:"display_name" validate:"min=1,max=255"`
	InitialValues []struct {
		Value        string `json:"value" validate:"min=1,max=50"`
		DisplayValue string `json:"display_value" validate:"min=1,max=255"`
	} `json:"initial_values" validate:"dive"`
}

func (app *application) createOptionType(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	var form createOptionRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	option := &store.OptionType{
		Name:        form.Name,
		DisplayName: form.DisplayName,
		CreatedByID: user.AdminUser.ID,
		Values:      make([]*store.OptionValue, 0, len(form.InitialValues)),
	}

	for _, value := range form.InitialValues {
		optionValue := &store.OptionValue{
			Value:        value.Value,
			DisplayValue: value.DisplayValue,
			CreatedByID:  user.AdminUser.ID,
		}

		option.Values = append(option.Values, optionValue)
	}

	if err := app.store.OptionType.Create(r.Context(), option); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"option":  option,
		"message": "Option created successfully",
	}

	app.successResponse(w, http.StatusCreated, response)
}

type addValueRequest struct {
	Values []struct {
		Value        string `json:"value" validate:"min=1,max=50"`
		DisplayValue string `json:"display_value" validate:"min=1,max=255"`
	} `json:"values"`
}

func (app *application) addOptionValues(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	optionTypeID := app.readStringID(r, "id")

	var form addValueRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	option, err := app.store.OptionType.GetByID(r.Context(), optionTypeID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "option type not found")

		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	optionValues := make([]*store.OptionValue, 0, len(form.Values))

	for _, value := range form.Values {
		optionValues = append(optionValues, &store.OptionValue{
			OptionTypeID: option.ID,
			Value:        value.Value,
			DisplayValue: value.DisplayValue,
			CreatedByID:  user.AdminUser.ID,
		})
	}

	if err := app.store.OptionType.CreateOptionValues(r.Context(), optionValues); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"values": optionValues,
	}

	app.successResponse(w, http.StatusCreated, response)
}

func (app *application) getOptionTypes(w http.ResponseWriter, r *http.Request) {
	fq := store.PaginateQueryFilter{
		Page:     1,
		PageSize: 20,
	}

	if err := fq.Parse(r); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	optionTypes, metadata, err := app.store.OptionType.GetAll(r.Context(), fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"option_types": optionTypes,
		"metadata":     metadata,
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getOptionTypeByID(w http.ResponseWriter, r *http.Request) {
	optionTypeID := app.readStringID(r, "id")

	option, err := app.store.OptionType.GetByID(r.Context(), optionTypeID)

	if err != nil {
		switch {

		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "option type not found")
		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	response := envelope{
		"option": option,
	}

	app.successResponse(w, http.StatusOK, response)
}

type updateOptionTypeRequest struct {
	Name        string `json:"name" validate:"min=1,max=255"`
	DisplayName string `json:"display_name" validate:"min=1,max=255"`
}

func (app *application) updateOptionType(w http.ResponseWriter, r *http.Request) {
	optionTypeID := app.readStringID(r, "id")

	var form updateOptionTypeRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	option, err := app.store.OptionType.GetByID(r.Context(), optionTypeID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "option type not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	option.Name = form.Name
	option.DisplayName = form.DisplayName

	err = app.store.OptionType.Update(r.Context(), option)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"option":  option,
		"message": "Option type updated successfully",
	}
	app.successResponse(w, http.StatusOK, response)
}

type updateOptionValueRequest struct {
	Value        string `json:"value" validate:"min=1,max=50"`
	DisplayValue string `json:"display_value" validate:"min=1,max=255"`
}

func (app *application) updateOptionValue(w http.ResponseWriter, r *http.Request) {
	optionValueID := app.readStringID(r, "id")

	var form updateOptionValueRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	optionValue := &store.OptionValue{
		ID:           optionValueID,
		Value:        form.Value,
		DisplayValue: form.DisplayValue,
	}

	err := app.store.OptionType.UpdateOptionValue(r.Context(), optionValue)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "option value not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"option_value": optionValue,
		"message":      "Option value updated successfully",
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) deleteOptionValue(w http.ResponseWriter, r *http.Request) {
	optionValueID := app.readStringID(r, "id")

	err := app.store.OptionType.DeleteOptionValue(r.Context(), optionValueID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "option value not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"message": "Option value deleted successfully",
	}

	app.successResponse(w, http.StatusOK, response)
}
