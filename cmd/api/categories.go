package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

type createCategoryForm struct {
	Name        string `json:"name" validate:"min=1,max=255"`
	Description string `json:"description" validate:"min=1,max=500"`
}

func (app *application) createCategory(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	var form createCategoryForm

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	category := &store.Category{
		Name:             form.Name,
		Description:      form.Description,
		Visible:          false,
		CreatedByAdminID: user.AdminUser.ID,
	}

	if err := app.store.Category.Create(r.Context(), category); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"category": category,
	}

	app.successResponse(w, http.StatusCreated, response)
}

func (app *application) getPublicCategories(w http.ResponseWriter, r *http.Request) {
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

	categories, metadata, err := app.store.Category.GetPublicCategories(r.Context(), fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"categories": categories,
		"metadata":   metadata,
	})
}

func (app *application) getAdminCategoriesView(w http.ResponseWriter, r *http.Request) {
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

	categories, metadata, err := app.store.Category.GetAdminCategoryView(r.Context(), fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"categories": categories,
		"metadata":   metadata,
	})
}

func (app *application) removeCategory(w http.ResponseWriter, r *http.Request) {
	categoryId := app.readStringID(r, "id")

	if err := app.store.Category.RemoveByID(r.Context(), categoryId); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"message": "category successfully deleted",
		"id":      categoryId,
	}

	app.successResponse(w, http.StatusOK, response)

}

type setCategoryVisibilityForm struct {
	Visible bool `json:"visible"`
}

func (app *application) setCategoryVisibility(w http.ResponseWriter, r *http.Request) {
	categoryId := app.readStringID(r, "id")

	var form setCategoryVisibilityForm

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := app.store.Category.SetCategoryVisibility(r.Context(), categoryId, form.Visible); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r)

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"message": "category visibility updated successfully",
		"id":      categoryId,
		"visible": form.Visible,
	}

	app.successResponse(w, http.StatusOK, response)

}
