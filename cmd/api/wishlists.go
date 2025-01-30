package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

type addProductWishlistRequest struct {
	ProductID string `json:"product_id" validate:"required"`
}

func (app *application) addProductToWhitelist(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	var form addProductWishlistRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	product, err := app.store.Products.GetProductByID(r.Context(), form.ProductID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")

		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	whitelist := &store.Wishlist{
		UserID:    user.ID,
		ProductID: product.ID,
	}

	if err := app.store.Wishlists.AddItem(r.Context(), whitelist); err != nil {
		switch {
		case errors.Is(err, store.ErrProductWishlistedAlready):
			app.conflictResponse(w, r, err.Error())

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"whitelist": whitelist,
	}

	app.successResponse(w, http.StatusCreated, response)
}

func (app *application) removeProductFromWhitelist(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	itemID := app.readStringID(r, "itemID")

	// Remove product from whitelist
	if err := app.store.Wishlists.RemoveItem(r.Context(), itemID, user.ID); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "item not found")

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Return success response
	response := envelope{
		"message": "product removed from whitelist successfully",
	}
	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getGroupVendorWishlisttem(w http.ResponseWriter, r *http.Request) {

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

	user := getUserFromCtx(r)

	wishlists, metadata, err := app.store.Wishlists.GetGroupWishlistItems(r.Context(), user.ID, 3, fq)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"vendor_wishlists": wishlists,
		"metadata":         metadata,
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getVendorWishlistItem(w http.ResponseWriter, r *http.Request) {
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

	vendorID := r.URL.Query().Get("vendor_id")
	if vendorID == "" {
		app.badRequestResponse(w, r, errors.New("vendor_id missing in query"))
		return
	}

	vendor, err := app.store.Users.GetVendorByID(r.Context(), vendorID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "vendor not found")

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	user := getUserFromCtx(r)

	wishlists, metadata, err := app.store.Wishlists.GetWishlistItems(r.Context(), user.ID, vendor.ID, fq)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"vendor_wishlists": wishlists,
		"metadata":         metadata,
	}

	app.successResponse(w, http.StatusOK, response)
}
