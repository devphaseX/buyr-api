package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

type createCommentRequest struct {
	Rating  int    `json:"rating" validate:"min=1,max=5"`
	Comment string `json:"comment" validate:"required,max=500"`
}

func (app *application) createReview(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	productID := app.readStringID(r, "productID")

	var form createCommentRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	product, err := app.store.Products.GetProductByID(r.Context(), productID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	review := &store.Review{
		UserID:    user.ID,
		ProductID: product.ID,
		Rating:    form.Rating,
		Comment:   form.Comment,
	}

	if err := app.store.Reviews.Create(r.Context(), review); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"review":  review,
		"message": "Review created successfully",
	}

	app.successResponse(w, http.StatusCreated, response)
}

func (app *application) getProductReviews(w http.ResponseWriter, r *http.Request) {
	productID := app.readStringID(r, "productID")

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

	product, err := app.store.Products.GetProductByID(r.Context(), productID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")

		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	reviews, metadata, err := app.store.Reviews.GetByProductID(r.Context(), product.ID, fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"reviews":  reviews,
		"metadata": metadata,
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) removeReview(w http.ResponseWriter, r *http.Request) {
	productID := app.readStringID(r, "productID")
	reviewID := app.readStringID(r, "reviewID")

	product, err := app.store.Products.GetProductByID(r.Context(), productID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.store.Reviews.Delete(r.Context(), product.ID, reviewID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"message": "review deleted successfully",
		"id":      reviewID,
	}

	app.successResponse(w, http.StatusOK, response)
}
