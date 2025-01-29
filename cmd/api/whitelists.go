package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

func (app *application) addProductToWhitelist(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	productID := app.readStringID(r, "productID")

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

	whitelist := &store.Whitelist{
		UserID:    user.ID,
		ProductID: product.ID,
	}

	if err := app.store.Whitelists.AddItem(r.Context(), whitelist); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"whitelist": whitelist,
	}

	app.successResponse(w, http.StatusCreated, response)
}
func (app *application) removeProductFromWhitelist(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	productID := app.readStringID(r, "productID")

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

	// Remove product from whitelist
	if err := app.store.Whitelists.RemoveItem(r.Context(), user.ID, product.ID); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Return success response
	response := envelope{
		"message": "product removed from whitelist successfully",
	}
	app.successResponse(w, http.StatusOK, response)
}
